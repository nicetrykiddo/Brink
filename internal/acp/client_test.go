package acp

import (
	"encoding/json"
	"testing"
)

func TestStopReason(t *testing.T) {
	for _, reason := range []string{"end_turn", "max_tokens", "max_turn_requests", "refusal", "cancelled"} {
		raw, _ := json.Marshal(map[string]string{"stopReason": reason})
		if got, err := StopReason(raw); err != nil || got != reason {
			t.Fatalf("%s: %s %v", reason, got, err)
		}
	}
	for _, raw := range []string{`{}`, `null`, `{"stopReason":true}`, `{"stopReason":"unknown"}`, `not JSON`} {
		if _, err := StopReason(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
