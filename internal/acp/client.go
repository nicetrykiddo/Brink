// Package acp implements the small ACP v1 surface needed for owned agent processes.
package acp

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
)

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Packet struct {
	Message Message
	Err     error
}

type Client struct {
	Incoming chan Packet
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	done     chan struct{}
	mu       sync.Mutex
	next     int
}

func Start(command []string, cwd string) (*Client, error) {
	if len(command) == 0 {
		return nil, errors.New("missing ACP command")
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	// Raw host diagnostics can contain credentials and model output.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	c := &Client{Incoming: make(chan Packet, 64), cmd: cmd, stdin: stdin, done: make(chan struct{})}
	go func() {
		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 65536), 4<<20)
		for s.Scan() {
			var m Message
			if json.Unmarshal(s.Bytes(), &m) != nil || m.JSONRPC != "2.0" {
				c.deliver(Packet{Err: errors.New("invalid ACP message")})
				return
			}
			if !c.deliver(Packet{Message: m}) {
				return
			}
		}
		c.deliver(Packet{Err: errors.New("ACP stream ended or exceeded message limit")})
	}()
	return c, nil
}

func (c *Client) deliver(p Packet) bool {
	select {
	case c.Incoming <- p:
		return true
	case <-c.done:
		return false
	}
}

func (c *Client) Close() {
	close(c.done)
	// Only the group created for this invocation is ever signalled.
	syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
	c.stdin.Close()
	c.cmd.Wait()
}

func (c *Client) send(ctx context.Context, m any) error {
	result := make(chan error, 1)
	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		result <- json.NewEncoder(c.stdin).Encode(m)
	}()
	select {
	case err := <-result:
		if err != nil {
			return errors.New("ACP write failed")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) Request(ctx context.Context, method string, params any) (int, error) {
	c.next++
	id := c.next
	return id, c.send(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

func (c *Client) Notify(ctx context.Context, method string, params any) error {
	return c.send(ctx, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (m Message) IsResponse(id int) bool {
	return m.Method == "" && string(m.ID) == strconv.Itoa(id)
}

func StopReason(raw json.RawMessage) (string, error) {
	var result struct {
		Reason string `json:"stopReason"`
	}
	if json.Unmarshal(raw, &result) == nil {
		switch result.Reason {
		case "end_turn", "max_tokens", "max_turn_requests", "refusal", "cancelled":
			return result.Reason, nil
		}
	}
	return "", errors.New("invalid prompt result")
}

// HandleRequest never grants permissions or advertises client filesystem/terminal access.
func (c *Client) HandleRequest(ctx context.Context, m Message) error {
	if m.Method == "" || len(m.ID) == 0 {
		return nil
	}
	if m.Method == "session/request_permission" {
		return c.send(ctx, map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{"outcome": map[string]string{"outcome": "cancelled"}}})
	}
	return c.send(ctx, map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": RPCError{Code: -32601, Message: "unsupported client method"}})
}

func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	id, err := c.Request(ctx, method, params)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p := <-c.Incoming:
			if p.Err != nil {
				return p.Err
			}
			m := p.Message
			if m.IsResponse(id) {
				if m.Error != nil {
					return fmt.Errorf("ACP %s failed (code %d)", method, m.Error.Code)
				}
				if json.Unmarshal(m.Result, result) != nil {
					return errors.New("invalid ACP result")
				}
				return nil
			}
			if err := c.HandleRequest(ctx, m); err != nil {
				return err
			}
		}
	}
}

func (c *Client) Initialize(ctx context.Context) error {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	var result struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if err := c.Call(ctx, "initialize", map[string]any{
		"protocolVersion": 1, "clientCapabilities": map[string]any{},
		"clientInfo": map[string]string{"name": hex.EncodeToString(nonce[:]), "version": "0.2"},
	}, &result); err != nil {
		return err
	}
	if result.ProtocolVersion != 1 {
		return errors.New("host did not negotiate ACP v1")
	}
	return nil
}
