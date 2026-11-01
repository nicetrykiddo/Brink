package supervise

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func randomName(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return "p" + hex.EncodeToString(b[:])
}

// This opt-in test uses the real installed host and a scripted loopback provider.
// It measures control mechanics, never model intelligence or lab performance.
func TestOpenCodeLocal(t *testing.T) {
	host := os.Getenv("BRINK_ACP_HOST")
	if host == "" {
		t.Skip("set BRINK_ACP_HOST to an absolute OpenCode v2 executable")
	}
	if !filepath.IsAbs(host) {
		t.Fatal("BRINK_ACP_HOST must be absolute")
	}
	for _, supervised := range []bool{false, true} {
		t.Run(fmt.Sprint(supervised), func(t *testing.T) {
			dir := t.TempDir()
			provider, model := randomName(t), randomName(t)
			artifact := filepath.Join(dir, "pebble")
			redirect := "Use the direct file operation now. " + randomName(t)
			var aborted, requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" {
					http.Error(w, "unexpected route", 404)
					return
				}
				body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
				if err != nil {
					return
				}
				var req struct {
					Model    string            `json:"model"`
					Tools    []json.RawMessage `json:"tools"`
					Messages []struct {
						Role    string          `json:"role"`
						Content json.RawMessage `json:"content"`
					} `json:"messages"`
				}
				if json.Unmarshal(body, &req) != nil || req.Model != model {
					http.Error(w, "invalid model", 400)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				flush, ok := w.(http.Flusher)
				if !ok {
					return
				}
				chunk := func(delta any, finish any) {
					v := map[string]any{"id": provider, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
					b, _ := json.Marshal(v)
					fmt.Fprintf(w, "data: %s\n\n", b)
					flush.Flush()
				}
				if len(req.Tools) == 0 {
					chunk(map[string]string{"content": "Pebble"}, "stop")
					fmt.Fprint(w, "data: [DONE]\n\n")
					return
				}
				requests.Add(1)
				for _, m := range req.Messages {
					if m.Role == "tool" {
						t.Logf("local tool result: %.1500s", m.Content)
						chunk(map[string]string{"content": "Complete."}, "stop")
						fmt.Fprint(w, "data: [DONE]\n\n")
						return
					}
				}
				if !bytes.Contains(body, []byte(redirect)) {
					chunk(map[string]string{"role": "assistant", "content": "Examining a local possibility. "}, nil)
					timer := time.NewTimer(2 * time.Second)
					defer timer.Stop()
					select {
					case <-r.Context().Done():
						aborted.Add(1)
						return
					case <-timer.C:
					}
				}
				arguments, _ := json.Marshal(map[string]string{"path": artifact, "content": "verified local value\n"})
				chunk(map[string]any{"tool_calls": []any{map[string]any{"index": 0, "id": provider + "x", "type": "function", "function": map[string]string{"name": "write", "arguments": string(arguments)}}}}, nil)
				chunk(map[string]any{}, "tool_calls")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer server.Close()
			config := map[string]any{
				"model":       provider + "/" + model,
				"providers":   map[string]any{provider: map[string]any{"package": "@opencode/ai/providers/openai-compatible", "settings": map[string]string{"baseURL": server.URL + "/v1", "apiKey": "local-placeholder"}, "models": map[string]any{model: map[string]any{"name": model, "limit": map[string]int{"context": 32000, "output": 1000}}}}},
				"permissions": []any{map[string]string{"action": "*", "resource": "*", "effect": "deny"}, map[string]string{"action": "edit", "resource": "pebble", "effect": "allow"}},
			}
			b, _ := json.Marshal(config)
			configPath := filepath.Join(dir, "opencode.json")
			if err := os.WriteFile(configPath, b, 0600); err != nil {
				t.Fatal(err)
			}
			// Empty environment prevents user credentials/plugins/config from entering this experiment.
			command := []string{"env", "-i", "PATH=" + os.Getenv("PATH"), "XDG_CONFIG_HOME=" + dir + "/config", "XDG_DATA_HOME=" + dir + "/data", "XDG_STATE_HOME=" + dir + "/state", "XDG_CACHE_HOME=" + dir + "/cache", "OPENCODE_CONFIG=" + configPath, "OPENCODE_DISABLE_MODELS_FETCH=true", "OPENCODE_DISABLE_DEFAULT_PLUGINS=true", "HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1", "NO_PROXY=127.0.0.1,localhost", host, "acp"}
			cfg := testConfig(dir, "")
			cfg.Check = fixtureCommand("artifact", dir, "")
			cfg.Prompt = "Create the requested file using the available local operation."
			cfg.Redirect = redirect
			cfg.StallSeconds = 5
			cfg.MaxSeconds = 15
			cfg.CancelSeconds = 5
			cfg.CheckEverySeconds = .05
			cfg.CheckTimeoutSeconds = 1
			cfg.MaxRedirects = 0
			if supervised {
				cfg.StallSeconds = .5
				cfg.MaxRedirects = 1
			}
			var out bytes.Buffer
			begin := time.Now()
			result, err := Run(context.Background(), cfg, command, &out)
			elapsed := time.Since(begin)
			t.Logf("supervised=%v elapsed=%s requests=%d aborted_streams=%d result=%+v\n%s", supervised, elapsed, requests.Load(), aborted.Load(), result, &out)
			if err != nil || !result.Success {
				t.Fatalf("%v", err)
			}
			if supervised && (result.Redirects != 1 || aborted.Load() != 1 || !strings.Contains(out.String(), `"cancelled":true`)) {
				t.Fatal("no independent cancellation/resumption proof")
			}
			if !supervised && (result.Redirects != 0 || aborted.Load() != 0) {
				t.Fatal("baseline unexpectedly interrupted")
			}
		})
	}
}
