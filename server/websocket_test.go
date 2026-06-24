package server

import (
	"testing"

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v2SeenEvents = make(map[string]struct{})
			if !processV2Event(nil, tt.method, nil, tt.name) {
				t.Fatalf("expected remote method %s to be acknowledged and ignored", tt.method)
			}
		})
	}
}

func TestProcessV2EventDeduplicatesSeenEventIDs(t *testing.T) {
	v2SeenEvents = make(map[string]struct{})
	if !processV2Event(nil, v2.MethodAgentExec, nil, "dup-event") {
		t.Fatal("expected first event to be handled")
	}
	if !processV2Event(nil, v2.MethodAgentExec, nil, "dup-event") {
		t.Fatal("expected duplicate event to be treated as already handled")
	}
}
