package server

import (
	"fmt"
	"testing"
	"time"

	v2 "github.com/xultral/komari-agent/protocol/v2"
)

func TestProcessV2EventIgnoresRemoteMethodsInMonitoringOnlyMode(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "exec", method: v2.MethodAgentExec},
		{name: "ping", method: v2.MethodAgentPing},
		{name: "terminal", method: v2.MethodAgentTerminal},
		{name: "file", method: "agent.file"},
		{name: "startup-config", method: "agent.startupConfig"},
		{name: "switch-version", method: "agent.switchVersion"},
		{name: "unknown-future-method", method: "agent.futureSensitiveOperation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetV2SeenEvents()
			if !processV2Event(nil, tt.method, nil, tt.name) {
				t.Fatalf("expected remote method %s to be acknowledged and ignored", tt.method)
			}
		})
	}
}

func TestProcessV2EventDeduplicatesSeenEventIDs(t *testing.T) {
	resetV2SeenEvents()
	if !processV2Event(nil, v2.MethodAgentExec, nil, "dup-event") {
		t.Fatal("expected first event to be handled")
	}
	if !processV2Event(nil, v2.MethodAgentExec, nil, "dup-event") {
		t.Fatal("expected duplicate event to be treated as already handled")
	}
}

func TestMarkV2EventSeenExpiresOldEvents(t *testing.T) {
	resetV2SeenEvents()
	v2AckMu.Lock()
	v2SeenEvents["expired"] = time.Now().Add(-v2SeenEventTTL - time.Second)
	v2AckMu.Unlock()

	if !markV2EventSeen("fresh") {
		t.Fatal("expected fresh event to be marked as seen")
	}

	v2AckMu.Lock()
	_, expiredStillPresent := v2SeenEvents["expired"]
	_, freshPresent := v2SeenEvents["fresh"]
	v2AckMu.Unlock()
	if expiredStillPresent {
		t.Fatal("expected expired event to be removed")
	}
	if !freshPresent {
		t.Fatal("expected fresh event to be retained")
	}
}

func TestMarkV2EventSeenCapsTrackedEvents(t *testing.T) {
	resetV2SeenEvents()
	v2AckMu.Lock()
	now := time.Now()
	for i := 0; i < v2SeenEventLimit; i++ {
		v2SeenEvents[fmt.Sprintf("event-%d", i)] = now
	}
	v2SeenEvents["event-0"] = now.Add(-time.Minute)
	v2AckMu.Unlock()

	if !markV2EventSeen("new-event") {
		t.Fatal("expected new event to be marked as seen")
	}

	v2AckMu.Lock()
	_, oldestPresent := v2SeenEvents["event-0"]
	_, newPresent := v2SeenEvents["new-event"]
	size := len(v2SeenEvents)
	v2AckMu.Unlock()
	if oldestPresent {
		t.Fatal("expected oldest event to be evicted")
	}
	if !newPresent {
		t.Fatal("expected new event to be retained")
	}
	if size != v2SeenEventLimit {
		t.Fatalf("expected event map to stay capped at %d, got %d", v2SeenEventLimit, size)
	}
}

func resetV2SeenEvents() {
	v2AckMu.Lock()
	v2SeenEvents = make(map[string]time.Time)
	v2AckMu.Unlock()
}
