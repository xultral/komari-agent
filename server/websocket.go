package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xultral/komari-agent/dnsresolver"
	"github.com/xultral/komari-agent/monitoring"
	"github.com/xultral/komari-agent/protocol/transport"
	v2 "github.com/xultral/komari-agent/protocol/v2"
	"github.com/xultral/komari-agent/utils"
	"github.com/xultral/komari-agent/ws"
)

var (
	v2AckMu       sync.Mutex
	v2AckEventIDs []string
	v2SeenEvents  = make(map[string]struct{})
)

func EstablishWebSocketConnection() {
	var conn *ws.SafeConn
	defer func() {
		if conn != nil {
			conn.Close()
		}
		resetConnectionProtocolVersion()
	}()
	var err error
	interval := math.Max(1, flags.Interval-1)

	dataTicker := time.NewTicker(time.Duration(interval * float64(time.Second)))
	defer dataTicker.Stop()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	nextProtocol := requestedProtocolVersion()
	activeProtocol := 0
	var readDone <-chan struct{}

	for {
		select {
		case <-dataTicker.C:
			if conn == nil {
				log.Println("Attempting to connect to WebSocket...")
				retry := 0
				connectProtocol := nextProtocol
				for retry <= flags.MaxRetries {
					if retry > 0 {
						log.Println("Retrying websocket connection, attempt:", retry)
					}
					websocketEndpoint := buildWebSocketEndpoint(connectProtocol)
					conn, err = connectWebSocket(websocketEndpoint)
					if err == nil {
						activeProtocol = connectProtocol
						nextProtocol = connectProtocol
						setConnectionProtocolVersion(activeProtocol)
						log.Printf("WebSocket connected using v%d protocol", activeProtocol)
						done := make(chan struct{})
						readDone = done
						go handleWebSocketMessages(conn, activeProtocol, done)
						break
					} else if shouldFallbackToV1(connectProtocol, err) {
						log.Printf("v2 WebSocket endpoint failed (%v), falling back to v1 until this connection is lost", err)
						connectProtocol = 1
						retry = 0
						continue
					} else {
						log.Println("Failed to connect to WebSocket:", err)
					}
					retry++
					time.Sleep(time.Duration(flags.ReconnectInterval) * time.Second)
				}

				if retry > flags.MaxRetries {
					log.Println("Max retries reached.")
					if connectProtocol < 2 {
						return
					}
					conn, err = runPostFallback(buildWebSocketEndpoint(connectProtocol), interval)
					if err != nil {
						if connectProtocol >= 2 && isV2ProtocolFailure(err) {
							log.Printf("v2 POST fallback failed (%v), falling back to v1 until this connection is lost", err)
							nextProtocol = 1
							setConnectionProtocolVersion(1)
							continue
						}
						log.Println("POST fallback stopped:", err)
						return
					}
					log.Println("WebSocket recovered from POST fallback")
					activeProtocol = connectProtocol
					nextProtocol = connectProtocol
					setConnectionProtocolVersion(activeProtocol)
					done := make(chan struct{})
					readDone = done
					go handleWebSocketMessages(conn, activeProtocol, done)
				}
			}

			data := monitoring.GenerateReport()
			if activeProtocol >= 2 {
				data = v2.BuildReportPayload(data)
			}
			err = conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				log.Println("Failed to send WebSocket message:", err)
				conn.Close()
				conn = nil // Mark connection as dead
				readDone = nil
				resetConnectionProtocolVersion()
				if requestedProtocolVersion() >= 2 {
					nextProtocol = 2
				}
				continue
			}
		case <-heartbeatTicker.C:
			if conn != nil {
				err := conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					log.Println("Failed to send heartbeat:", err)
					conn.Close()
					conn = nil // Mark connection as dead
					readDone = nil
					resetConnectionProtocolVersion()
					if requestedProtocolVersion() >= 2 {
						nextProtocol = 2
					}
				}
			}
		case <-readDone:
			log.Println("WebSocket disconnected")
			if conn != nil {
				conn.Close()
				conn = nil
			}
			readDone = nil
			activeProtocol = 0
			resetConnectionProtocolVersion()
			if requestedProtocolVersion() >= 2 {
				nextProtocol = 2
			}
		}
	}
}

func buildWebSocketEndpoint(protocolVersion int) string {
	path := "/api/clients/report?token=" + flags.Token
	if protocolVersion >= 2 {
		path = "/api/clients/v2/rpc?token=" + flags.Token
	}
	websocketEndpoint := strings.TrimSuffix(flags.Endpoint, "/") + path
	websocketEndpoint = "ws" + strings.TrimPrefix(websocketEndpoint, "http")
	if convertedEndpoint, err := utils.ConvertIDNToASCII(websocketEndpoint); err == nil {
		return convertedEndpoint
	} else {
		log.Printf("Warning: Failed to convert WebSocket IDN to ASCII: %v", err)
	}
	return websocketEndpoint
}

func runPostFallback(websocketEndpoint string, interval float64) (*ws.SafeConn, error) {
	log.Println("Entering v2 POST fallback mode")
	reportTicker := time.NewTicker(time.Duration(interval * float64(time.Second)))
	defer reportTicker.Stop()
	reconnectTicker := time.NewTicker(time.Duration(flags.ReconnectInterval) * time.Second)
	defer reconnectTicker.Stop()

	for {
		select {
		case <-reportTicker.C:
			reportID := fmt.Sprintf("report-%d", time.Now().UnixNano())
			ackIDs := snapshotV2AckEventIDs()
			resp, err := postV2Request(v2.BuildReportRequest(reportID, monitoring.GenerateReport(), ackIDs))
			if err != nil {
				if shouldFallbackToV1(2, err) {
					return nil, err
				}
				log.Println("Failed to POST v2 report:", err)
				continue
			}
			clearV2AckEventIDs(ackIDs)
			processV2ResponseEvents(resp)
		case <-reconnectTicker.C:
			conn, err := connectWebSocket(websocketEndpoint)
			if err == nil {
				return conn, nil
			}
			if shouldFallbackToV1(2, err) {
				return nil, err
			}
			log.Println("POST fallback WebSocket recovery failed:", err)
		}
	}
}

func postV2Request(payload []byte) (*v2.Response, error) {
	endpoint := strings.TrimSuffix(flags.Endpoint, "/") + "/api/clients/v2/rpc?token=" + flags.Token
	body := payload
	compressed := false
	if !flags.DisableCompression {
		if gz, err := transport.GzipBytes(payload); err == nil {
			body = gz
			compressed = true
		}
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if flags.CFAccessClientID != "" && flags.CFAccessClientSecret != "" {
		req.Header.Set("CF-Access-Client-Id", flags.CFAccessClientID)
		req.Header.Set("CF-Access-Client-Secret", flags.CFAccessClientSecret)
	}
	client := dnsresolver.GetHTTPClientWithPreference(35*time.Second, flags.PreferIPVersion)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bytesBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &httpStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(bytesBody)}
	}
	rpcResp, err := parseV2Response(bytesBody)
	if err != nil {
		return nil, err
	}
	resetV2ProtocolFailures(2)
	return rpcResp, nil
}

func processV2ResponseEvents(resp *v2.Response) {
	if resp == nil || resp.Result == nil {
		return
	}
	var result v2.EventResult
	if err := v2.BindResult(resp.Result, &result); err != nil {
		log.Println("Failed to bind v2 event result:", err)
		return
	}
	for _, event := range result.Events {
		if processV2Event(nil, event.Method, event.Params, event.ID) {
			addV2AckEventID(event.ID)
		}
	}
}

func snapshotV2AckEventIDs() []string {
	v2AckMu.Lock()
	defer v2AckMu.Unlock()
	return append([]string{}, v2AckEventIDs...)
}

func clearV2AckEventIDs(sent []string) {
	if len(sent) == 0 {
		return
	}
	sentSet := make(map[string]struct{}, len(sent))
	for _, id := range sent {
		sentSet[id] = struct{}{}
	}
	v2AckMu.Lock()
	defer v2AckMu.Unlock()
	remaining := v2AckEventIDs[:0]
	for _, id := range v2AckEventIDs {
		if _, ok := sentSet[id]; !ok {
			remaining = append(remaining, id)
		}
	}
	v2AckEventIDs = remaining
}

func addV2AckEventID(id string) {
	if id == "" {
		return
	}
	v2AckMu.Lock()
	defer v2AckMu.Unlock()
	v2AckEventIDs = append(v2AckEventIDs, id)
}

func markV2EventSeen(id string) bool {
	if id == "" {
		return true
	}
	v2AckMu.Lock()
	defer v2AckMu.Unlock()
	if _, ok := v2SeenEvents[id]; ok {
		return false
	}
	v2SeenEvents[id] = struct{}{}
	return true
}

func connectWebSocket(websocketEndpoint string) (*ws.SafeConn, error) {
	dialer := newWSDialer()

	headers := newWSHeaders()

	conn, resp, err := dialer.Dial(websocketEndpoint, headers)
	if err != nil {
		if resp != nil && resp.StatusCode != 101 {
			return nil, &httpStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
		}
		return nil, err
	}

	return ws.NewSafeConn(conn), nil
}

func handleWebSocketMessages(conn *ws.SafeConn, protocolVersion int, done chan<- struct{}) {
	defer close(done)
	for {
		_, message_raw, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			return
		}
		var message struct {
			JSONRPC string      `json:"jsonrpc,omitempty"`
			Method  string      `json:"method,omitempty"`
			Params  interface{} `json:"params,omitempty"`
			Message string      `json:"message"`
			// Terminal
			TerminalId string `json:"request_id,omitempty"`
			// Remote Exec
			ExecCommand string `json:"command,omitempty"`
			ExecTaskID  string `json:"task_id,omitempty"`
			// Ping
			PingTaskID uint   `json:"ping_task_id,omitempty"`
			PingType   string `json:"ping_type,omitempty"`
			PingTarget string `json:"ping_target,omitempty"`
		}
		err = json.Unmarshal(message_raw, &message)
		if err != nil {
			log.Println("Bad ws message:", err)
			continue
		}
		if message.JSONRPC == v2.Version && protocolVersion >= 2 {
			processV2Event(conn, message.Method, message.Params, "")
			continue
		}

		if message.Message == "terminal" || message.TerminalId != "" {
			log.Println("Ignoring terminal request in monitoring-only mode")
			continue
		}
		if message.Message == "exec" {
			log.Println("Ignoring remote exec request in monitoring-only mode")
			continue
		}
		if message.Message == "ping" || message.PingTaskID != 0 || message.PingType != "" || message.PingTarget != "" {
			log.Println("Ignoring remote ping task in monitoring-only mode")
			continue
		}
	}
}

func processV2Event(conn *ws.SafeConn, method string, params interface{}, eventID string) bool {
	if !markV2EventSeen(eventID) {
		return true
	}
	switch method {
	case v2.MethodAgentExec:
		log.Println("Ignoring v2 remote exec event in monitoring-only mode")
		return true
	case v2.MethodAgentPing:
		log.Println("Ignoring v2 remote ping event in monitoring-only mode")
		return true
	case v2.MethodAgentTerminal:
		log.Println("Ignoring v2 terminal event in monitoring-only mode")
		return true
	case v2.MethodAgentMessage, v2.MethodAgentEvent:
		log.Printf("received v2 %s: %+v", method, params)
		return true
	default:
		log.Printf("unknown v2 event method %s", method)
	}
	return false
}

// newWSDialer 构造统一的 WebSocket 拨号器（自定义解析、IPv4/IPv6 动态排序、可选 TLS 忽略）
func newWSDialer() *websocket.Dialer {
	d := &websocket.Dialer{
		HandshakeTimeout:  15 * time.Second,
		NetDialContext:    dnsresolver.GetDialContextWithPreference(15*time.Second, flags.PreferIPVersion),
		Proxy:             http.ProxyFromEnvironment,
		EnableCompression: !flags.DisableCompression,
	}
	if flags.IgnoreUnsafeCert {
		d.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return d
}

// newWSHeaders 统一构造 WS 请求头（含 Cloudflare Access 头）
func newWSHeaders() http.Header {
	headers := http.Header{}
	if flags.CFAccessClientID != "" && flags.CFAccessClientSecret != "" {
		headers.Set("CF-Access-Client-Id", flags.CFAccessClientID)
		headers.Set("CF-Access-Client-Secret", flags.CFAccessClientSecret)
	}
	return headers
}
