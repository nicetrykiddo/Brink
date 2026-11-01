package supervise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"
)

type Evidence struct {
	Milestones []string `json:"milestones"`
	Success    bool     `json:"success"`
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p)+b.Len() > 65536 {
		return 0, errors.New("check output exceeds 64 KiB")
	}
	return b.Buffer.Write(p)
}

func check(ctx context.Context, cfg Config) (Evidence, error) {
	ctx, cancel := context.WithTimeout(ctx, seconds(cfg.CheckTimeoutSeconds))
	defer cancel()
	cmd := exec.CommandContext(ctx, cfg.Check[0], cfg.Check[1:]...)
	cmd.Dir = cfg.Cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 100 * time.Millisecond
	var b limitedBuffer
	cmd.Stdout = &b
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return Evidence{}, errors.New("cannot start evidence check")
	}
	err := cmd.Wait()
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err != nil {
		return Evidence{}, errors.New("evidence check failed or timed out")
	}
	var result struct {
		Milestones []string `json:"milestones"`
		Success    *bool    `json:"success"`
	}
	d := json.NewDecoder(&b)
	d.DisallowUnknownFields()
	if d.Decode(&result) != nil || result.Success == nil || d.Decode(new(any)) != io.EOF {
		return Evidence{}, errors.New("invalid evidence check JSON")
	}
	allowed := map[string]bool{}
	for _, name := range cfg.Milestones {
		allowed[name] = true
	}
	for _, name := range result.Milestones {
		if !allowed[name] {
			return Evidence{}, errors.New("check returned an undeclared milestone")
		}
	}
	return Evidence{Success: *result.Success, Milestones: result.Milestones}, nil
}
