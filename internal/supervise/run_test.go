package supervise

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixtureCommand(mode, dir, scenario string) []string {
	return []string{"env", "GORACE=atexit_sleep_ms=0", os.Args[0], "-test.run=^TestFixture$", "--", mode, dir, scenario}
}

// TestFixture is a local protocol double, not a model or an HTB solve.
func TestFixture(t *testing.T) {
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) != 4 {
		return
	}
	mode, dir, scenario := args[1], args[2], args[3]
	if mode == "artifact" {
		b, _ := os.ReadFile(filepath.Join(dir, "pebble"))
		json.NewEncoder(os.Stdout).Encode(Evidence{Success: string(b) == "verified local value\n"})
		os.Exit(0)
	}
	if mode == "check" {
		if scenario == "check_error" {
			os.Exit(1)
		}
		if scenario == "check_hang" {
			time.Sleep(time.Hour)
		}
		if scenario == "check_invalid" {
			fmt.Println(`{"success":true,"untrusted":1}`)
			os.Exit(0)
		}
		if scenario == "check_large" {
			fmt.Print(strings.Repeat("x", 65537))
			os.Exit(0)
		}
		b, err := os.ReadFile(filepath.Join(dir, "proof"))
		if err != nil {
			b = []byte(`{"success":false,"milestones":[]}`)
		}
		fmt.Print(string(b))
		os.Exit(0)
	}
	var mu sync.Mutex
	send := func(m any) { mu.Lock(); defer mu.Unlock(); json.NewEncoder(os.Stdout).Encode(m) }
	response := func(id json.RawMessage, result any) {
		send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	}
	proof := func(body string) {
		p := filepath.Join(dir, "proof")
		if os.WriteFile(p+".tmp", []byte(body), 0600) != nil {
			os.Exit(4)
		}
		if os.Rename(p+".tmp", p) != nil {
			os.Exit(4)
		}
	}
	notify := func(kind, status string) {
		send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "local", "update": map[string]any{"sessionUpdate": kind, "toolCallId": "one", "status": status, "content": map[string]string{"type": "text", "text": "PRIVATE_SENTINEL success claimed"}}}})
	}
	scanner := bufio.NewScanner(os.Stdin)
	var pending json.RawMessage
	turns := 0
	for scanner.Scan() {
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &m) != nil {
			os.Exit(3)
		}
		switch m.Method {
		case "initialize":
			if scenario == "bad_version" {
				response(m.ID, map[string]any{"protocolVersion": 99})
			} else {
				response(m.ID, map[string]any{"protocolVersion": 1})
			}
		case "session/new":
			response(m.ID, map[string]string{"sessionId": "local"})
		case "session/prompt":
			turns++
			pending = m.ID
			switch scenario {
			case "lying":
				notify("agent_message_chunk", "")
				response(pending, map[string]string{"stopReason": "end_turn"})
			case "productive":
				notify("tool_call", "in_progress")
				go func() { time.Sleep(180 * time.Millisecond); proof(`{"success":true,"milestones":["access"]}`) }()
			case "false_alarm":
				go func() { time.Sleep(180 * time.Millisecond); proof(`{"success":true,"milestones":[]}`) }()
			case "milestones":
				proof(`{"success":false,"milestones":["access"]}`)
				notify("tool_call", "completed")
			case "permission":
				send(map[string]any{"jsonrpc": "2.0", "id": "host-choice", "method": "session/request_permission", "params": map[string]string{"sessionId": "local"}})
			case "wrong_session":
				send(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "unrelated", "update": map[string]string{"sessionUpdate": "tool_call", "toolCallId": "other", "status": "in_progress"}}})
			default:
				if turns > 1 {
					proof(`{"success":true,"milestones":[]}`)
					response(pending, map[string]string{"stopReason": "end_turn"})
				}
			}
		case "session/cancel":
			if scenario == "invalid_cancel" {
				response(pending, map[string]any{})
			} else if scenario != "ignores_cancel" {
				response(pending, map[string]string{"stopReason": "cancelled"})
			}
		}
	}
	os.Exit(0)
}

func testConfig(dir, scenario string) Config {
	return Config{Cwd: dir, Prompt: "Work on the harmless local task", Redirect: "Use the independent discriminator and produce the required result", Check: fixtureCommand("check", dir, scenario), Milestones: []string{"access"}, StallSeconds: .12, MaxSeconds: 3, ToolGraceSeconds: .3, CancelSeconds: .5, CheckEverySeconds: .02, CheckTimeoutSeconds: .4, MaxRedirects: 1}
}

func TestControl(t *testing.T) {
	cases := []struct {
		name      string
		success   bool
		reason    string
		redirects int
		err       bool
	}{
		{"stalled", true, "verified", 1, false},
		{"lying", false, "redirect_limit", 1, false},
		{"productive", true, "verified", 0, false},
		{"milestones", false, "redirect_limit", 1, false},
		{"ignores_cancel", false, "error", 0, true},
		{"invalid_cancel", false, "error", 0, true},
		{"permission", false, "redirect_limit", 1, false},
		{"check_error", false, "error", 0, true},
		{"check_hang", false, "error", 0, true},
		{"check_invalid", false, "error", 0, true},
		{"check_large", false, "error", 0, true},
		{"bad_version", false, "error", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := testConfig(dir, tc.name)
			var out bytes.Buffer
			r, err := Run(context.Background(), cfg, fixtureCommand("host", dir, tc.name), &out)
			if (err != nil) != tc.err || r.Success != tc.success || r.Reason != tc.reason || r.Redirects != tc.redirects {
				t.Fatalf("result=%+v err=%v log=%s", r, err, &out)
			}
			if strings.Contains(out.String(), "PRIVATE_SENTINEL") || strings.Contains(out.String(), cfg.Prompt) {
				t.Fatal("private content leaked")
			}
			if tc.name == "stalled" {
				s := out.String()
				a, b, c := strings.Index(s, "cancel_requested"), strings.Index(s, "turn_settled"), strings.Index(s, "redirect_sent")
				if a < 0 || a >= b || b >= c || !strings.Contains(s, `"cancelled":true`) {
					t.Fatalf("missing ordered cancellation proof: %s", s)
				}
			}
			if tc.name == "permission" && !strings.Contains(out.String(), "permission_cancelled") {
				t.Fatal("permission request ignored")
			}
		})
	}
}

func TestKnownPolicyLimits(t *testing.T) {
	for _, name := range []string{"missed_stall", "false_alarm", "wrong_session"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := testConfig(dir, name)
			cfg.MaxRedirects = 0
			if name == "missed_stall" {
				cfg.StallSeconds = 10
				cfg.MaxSeconds = .25
			}
			var out bytes.Buffer
			r, err := Run(context.Background(), cfg, fixtureCommand("host", dir, name), &out)
			if err != nil || r.Success {
				t.Fatalf("%+v %v %s", r, err, &out)
			}
			if name == "missed_stall" && r.Reason != "deadline" {
				t.Fatal("expected task deadline after missed stall")
			}
			if name != "missed_stall" && !strings.Contains(out.String(), "evidence_deadline") {
				t.Fatal("expected evidence deadline")
			}
		})
	}
}

func TestFalseAlarmControl(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir, "false_alarm")
	cfg.StallSeconds = .5
	cfg.MaxRedirects = 0
	var out bytes.Buffer
	r, err := Run(context.Background(), cfg, fixtureCommand("host", dir, "false_alarm"), &out)
	if err != nil || !r.Success || r.Redirects != 0 {
		t.Fatalf("uninterrupted productive control failed: %+v %v %s", r, err, &out)
	}
}

func TestEvidenceContract(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir, "")
	for _, body := range []string{`{}`, `{"success":false} {"success":true}`, `{"success":true,"milestones":["invented"]}`, `{"success":true,"other":1}`} {
		os.WriteFile(filepath.Join(dir, "proof"), []byte(body), 0600)
		if _, err := check(context.Background(), cfg); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	for _, change := range []func(*Config){func(c *Config) { c.StallSeconds = 0 }, func(c *Config) { c.Cwd = "relative" }, func(c *Config) { c.Check = nil }, func(c *Config) { c.Milestones = []string{"a", "a"} }} {
		bad := cfg
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("accepted invalid config")
		}
	}
}

func TestAlreadyVerified(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir, "")
	if err := os.WriteFile(filepath.Join(dir, "proof"), []byte(`{"success":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	r, err := Run(context.Background(), cfg, []string{"/nonexistent-host"}, &out)
	if err != nil || !r.Success || r.Reason != "already_verified" {
		t.Fatalf("%+v %v", r, err)
	}
}

func BenchmarkEvidenceCheck(b *testing.B) {
	dir := b.TempDir()
	cfg := testConfig(dir, "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := check(context.Background(), cfg); err != nil {
			b.Fatal(err)
		}
	}
}
