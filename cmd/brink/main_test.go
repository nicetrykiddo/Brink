package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.json")
	config := map[string]any{"cwd": dir, "prompt": "A harmless local check", "check": []string{"sh", "-c", `printf '{"success":true}'`}, "max_redirects": 0}
	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"supervise", path, "--", "/nonexistent-host"}); err != nil {
		t.Fatalf("already-passing check should avoid host startup: %v", err)
	}
	for _, body := range []string{`{}`, `{"unexpected":1}`, string(b) + " {}", string(b) + strings.Repeat(" ", 1<<20)} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"supervise", path, "--", "/nonexistent-host"}); err == nil {
			t.Fatal("accepted invalid config")
		}
	}
	for _, args := range [][]string{{"probe"}, {"probe", "--"}, {"supervise"}, {"supervise", path, "--"}} {
		if run(args) == nil {
			t.Fatalf("accepted missing arguments: %v", args)
		}
	}
}
