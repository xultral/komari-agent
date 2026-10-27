package v2

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildReportPayloadKeepsRawReport(t *testing.T) {
	report := []byte(`{"cpu":{"usage":12.5},"message":"ok"}`)
	payload := BuildReportPayload(report)

	var request Request
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("failed to decode report payload: %v", err)
	}
	if request.Method != MethodAgentReport {
		t.Fatalf("unexpected method: %s", request.Method)
	}

	params, ok := request.Params.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected params type: %T", request.Params)
	}
	raw, err := json.Marshal(params["report"])
	if err != nil {
		t.Fatalf("failed to re-encode report: %v", err)
	}
	var got, want interface{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(report, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("report changed during encoding: got %s want %s", raw, report)
	}
}

func TestBuildReportRequestIncludesAcknowledgements(t *testing.T) {
	payload := BuildReportRequest("report-1", []byte(`{"cpu":{"usage":1}}`), []string{"event-1"})

	var request struct {
		ID     interface{} `json:"id"`
		Method string      `json:"method"`
		Params struct {
			Report      json.RawMessage `json:"report"`
			AckEventIDs []string        `json:"ack_event_ids"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("failed to decode report request: %v", err)
	}
	if request.ID != "report-1" || request.Method != MethodAgentReport {
		t.Fatalf("unexpected request metadata: %+v", request)
	}
	if string(request.Params.Report) != `{"cpu":{"usage":1}}` {
		t.Fatalf("unexpected report: %s", request.Params.Report)
	}
	if !reflect.DeepEqual(request.Params.AckEventIDs, []string{"event-1"}) {
		t.Fatalf("unexpected acknowledgements: %v", request.Params.AckEventIDs)
	}
}
