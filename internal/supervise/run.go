// Package supervise runs evidence-bounded experiments through a replaceable ACP host.
package supervise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"time"

	"github.com/nicetrykiddo/Brink/internal/acp"
)

type Config struct {
	Cwd                 string   `json:"cwd"`
	Prompt              string   `json:"prompt"`
	Redirect            string   `json:"redirect"`
	Check               []string `json:"check"`
	Milestones          []string `json:"milestones"`
	StallSeconds        float64  `json:"stall_seconds"`
	MaxSeconds          float64  `json:"max_seconds"`
	ToolGraceSeconds    float64  `json:"tool_grace_seconds"`
	CancelSeconds       float64  `json:"cancel_seconds"`
	CheckEverySeconds   float64  `json:"check_every_seconds"`
	CheckTimeoutSeconds float64  `json:"check_timeout_seconds"`
	MaxRedirects        int      `json:"max_redirects"`
}

func Defaults() Config {
	return Config{StallSeconds: 180, MaxSeconds: 1800, ToolGraceSeconds: 30, CancelSeconds: 10, CheckEverySeconds: 2, CheckTimeoutSeconds: 5, MaxRedirects: 2}
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

func (c Config) Validate() error {
	if !filepath.IsAbs(c.Cwd) || c.Prompt == "" || len(c.Check) == 0 || c.Check[0] == "" {
		return errors.New("absolute cwd, prompt and check command are required")
	}
	for _, v := range []float64{c.StallSeconds, c.MaxSeconds, c.CancelSeconds, c.CheckEverySeconds, c.CheckTimeoutSeconds} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < .001 || v > 86400 {
			return errors.New("positive time budgets between .001 and 86400 seconds are required")
		}
	}
	if c.ToolGraceSeconds < 0 || math.IsNaN(c.ToolGraceSeconds) || math.IsInf(c.ToolGraceSeconds, 0) || c.ToolGraceSeconds > 86400 || c.MaxRedirects < 0 {
		return errors.New("invalid tool grace or redirect limit")
	}
	if c.MaxRedirects > 0 && c.Redirect == "" {
		return errors.New("redirect instructions are required when redirects are enabled")
	}
	seen := map[string]bool{}
	for _, name := range c.Milestones {
		if name == "" || seen[name] {
			return errors.New("milestone names must be nonempty and unique")
		}
		seen[name] = true
	}
	return nil
}

type Record struct {
	Event     string  `json:"event"`
	ElapsedS  float64 `json:"elapsed_s"`
	Count     int     `json:"count,omitempty"`
	Cancelled bool    `json:"cancelled,omitempty"`
}

type Result struct {
	Success   bool
	Redirects int
	Reason    string
}

// Run never trusts prose, tool counts, or agent-supplied "verified" labels as evidence.
func Run(parent context.Context, cfg Config, command []string, out io.Writer) (result Result, err error) {
	if err := cfg.Validate(); err != nil {
		return result, err
	}
	start := time.Now()
	enc := json.NewEncoder(out)
	log := func(event string, count int, cancelled bool) error {
		return enc.Encode(Record{Event: event, ElapsedS: time.Since(start).Seconds(), Count: count, Cancelled: cancelled})
	}
	defer func() {
		if err != nil {
			result.Reason = "error"
		}
		if e := log("finished_"+result.Reason, result.Redirects, false); err == nil {
			err = e
		}
	}()
	ctx, cancel := context.WithTimeout(parent, seconds(cfg.MaxSeconds))
	defer cancel()
	known := map[string]bool{}
	lastProgress := time.Now()
	verify := func() (bool, error) {
		e, err := check(ctx, cfg)
		if err != nil {
			return false, err
		}
		fresh := 0
		for _, name := range e.Milestones {
			if !known[name] {
				known[name] = true
				fresh++
			}
		}
		if fresh > 0 {
			lastProgress = time.Now()
			if err := log("verified_progress", len(known), false); err != nil {
				return false, err
			}
		}
		return e.Success, nil
	}
	if ok, err := verify(); err != nil {
		return result, err
	} else if ok {
		return Result{Success: true, Reason: "already_verified"}, nil
	}
	c, err := acp.Start(command, cfg.Cwd)
	if err != nil {
		return result, err
	}
	defer c.Close()
	setupCtx, setupCancel := context.WithTimeout(ctx, seconds(cfg.CancelSeconds))
	defer setupCancel()
	if err := c.Initialize(setupCtx); err != nil {
		return result, err
	}
	var session struct {
		ID string `json:"sessionId"`
	}
	if err := c.Call(setupCtx, "session/new", map[string]any{"cwd": cfg.Cwd, "mcpServers": []any{}}, &session); err != nil {
		return result, err
	}
	if session.ID == "" {
		return result, errors.New("host returned an empty session ID")
	}
	if err := log("session_ready", 0, false); err != nil {
		return result, err
	}
	prompt := func(text string) (int, error) {
		return c.Request(ctx, "session/prompt", map[string]any{"sessionId": session.ID, "prompt": []any{map[string]string{"type": "text", "text": text}}})
	}
	id, err := prompt(cfg.Prompt)
	if err != nil {
		return result, err
	}
	lastProgress = time.Now()
	active := map[string]bool{}
	update := func(m acp.Message) error {
		if m.Method == "session/update" {
			var p struct {
				SessionID string `json:"sessionId"`
				Update    struct {
					Type   string `json:"sessionUpdate"`
					ID     string `json:"toolCallId"`
					Status string `json:"status"`
				} `json:"update"`
			}
			if json.Unmarshal(m.Params, &p) != nil {
				return errors.New("invalid session update")
			}
			if p.SessionID == session.ID && (p.Update.Type == "tool_call" || p.Update.Type == "tool_call_update") {
				if p.Update.ID == "" || len(active) >= 4096 {
					return errors.New("invalid or excessive outstanding tool calls")
				}
				switch p.Update.Status {
				case "pending", "in_progress":
					active[p.Update.ID] = true
				case "completed", "failed":
					delete(active, p.Update.ID)
				}
			}
		}
		if m.Method == "session/request_permission" {
			if err := log("permission_cancelled", 0, false); err != nil {
				return err
			}
		}
		return c.HandleRequest(ctx, m)
	}
	settle := func() error {
		// The run deadline may have expired; cancellation has a separate bounded cleanup window.
		stopCtx, stopCancel := context.WithTimeout(context.Background(), seconds(cfg.CancelSeconds))
		defer stopCancel()
		if err := c.Notify(stopCtx, "session/cancel", map[string]string{"sessionId": session.ID}); err != nil {
			return err
		}
		if err := log("cancel_requested", 0, false); err != nil {
			return err
		}
		for {
			select {
			case <-stopCtx.Done():
				return errors.New("host did not settle after cancellation; no redirect sent")
			case p := <-c.Incoming:
				if p.Err != nil {
					return p.Err
				}
				if p.Message.IsResponse(id) {
					m := p.Message
					var response struct {
						StopReason string `json:"stopReason"`
					}
					if m.Error != nil && m.Error.Code != -32800 {
						return errors.New("host failed while cancelling")
					}
					if m.Error == nil && json.Unmarshal(m.Result, &response) != nil {
						return errors.New("invalid cancellation response")
					}
					return log("turn_settled", 0, response.StopReason == "cancelled" || m.Error != nil)
				}
				if err := c.HandleRequest(stopCtx, p.Message); err != nil {
					return err
				}
			}
		}
	}
	ticker := time.NewTicker(seconds(cfg.CheckEverySeconds))
	defer ticker.Stop()
	for {
		reason := ""
		ended := false
		select {
		case <-ctx.Done():
			if err := settle(); err != nil {
				return result, err
			}
			result.Reason = "deadline"
			return result, nil
		case p := <-c.Incoming:
			if p.Err != nil {
				return result, p.Err
			}
			if p.Message.IsResponse(id) {
				if p.Message.Error != nil {
					return result, fmt.Errorf("prompt failed (code %d)", p.Message.Error.Code)
				}
				var response struct {
					StopReason string `json:"stopReason"`
				}
				if json.Unmarshal(p.Message.Result, &response) != nil || response.StopReason == "" {
					return result, errors.New("invalid prompt result")
				}
				reason, ended = "unverified_end", true
			} else if err := update(p.Message); err != nil {
				return result, err
			}
		case <-ticker.C:
			ok, err := verify()
			if err != nil {
				return result, err
			}
			if ok {
				if err := settle(); err != nil {
					return result, err
				}
				result.Success, result.Reason = true, "verified"
				return result, nil
			}
			budget := seconds(cfg.StallSeconds)
			if len(active) > 0 {
				budget += seconds(cfg.ToolGraceSeconds)
			}
			if time.Since(lastProgress) >= budget {
				reason = "evidence_deadline"
			}
		}
		if reason == "" {
			continue
		}
		if err := log(reason, 0, false); err != nil {
			return result, err
		}
		if !ended {
			if err := settle(); err != nil {
				return result, err
			}
		}
		ok, err := verify()
		if err != nil {
			return result, err
		}
		if ok {
			result.Success, result.Reason = true, "verified"
			return result, nil
		}
		if result.Redirects >= cfg.MaxRedirects {
			result.Reason = "redirect_limit"
			return result, nil
		}
		result.Redirects++
		active = map[string]bool{}
		// A redirect gets one fresh bounded experiment, not a claim of progress.
		lastProgress = time.Now()
		id, err = prompt(cfg.Redirect)
		if err != nil {
			return result, err
		}
		if err := log("redirect_sent", result.Redirects, false); err != nil {
			return result, err
		}
	}
}
