package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nicetrykiddo/Brink/internal/acp"
	"github.com/nicetrykiddo/Brink/internal/replay"
	"github.com/nicetrykiddo/Brink/internal/supervise"
	"io"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: brink replay SESSION.zip [--json OUTPUT.json] [--model-budget SECONDS] [--research-budget SECONDS]\n       brink probe -- HOST [ARGS...]\n       brink supervise CONFIG.json -- HOST [ARGS...]")
}
func run(args []string) error {
	if len(args) > 0 && (args[0] == "probe" || args[0] == "supervise") {
		return live(args)
	}
	if len(args) == 0 || args[0] != "replay" {
		usage()
		return fmt.Errorf("expected replay subcommand")
	}
	var path, output string
	opts := replay.Defaults()
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--json" || a == "--model-budget" || a == "--research-budget" {
			i++
			if i >= len(args) {
				return fmt.Errorf("missing value for %s", a)
			}
			val := args[i]
			switch a {
			case "--json":
				output = val
			case "--model-budget":
				n, err := strconv.ParseInt(val, 10, 64)
				if err != nil || n <= 0 {
					return fmt.Errorf("invalid model budget %q", val)
				}
				opts.ModelBudgetS = n
			case "--research-budget":
				n, err := strconv.ParseInt(val, 10, 64)
				if err != nil || n <= 0 {
					return fmt.Errorf("invalid research budget %q", val)
				}
				opts.ResearchBudgetS = n
			}
		} else if len(a) > 0 && a[0] == '-' {
			return fmt.Errorf("unknown flag %q", a)
		} else if path == "" {
			path = a
		} else {
			return fmt.Errorf("unexpected argument %q", a)
		}
	}
	if path == "" {
		usage()
		return fmt.Errorf("missing ZIP path")
	}
	events, meta, err := replay.Load(path)
	if err != nil {
		return err
	}
	report := replay.Detect(events, meta, opts)
	fmt.Println(replay.Summary(report))
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return err
		}
		err = replay.WriteJSON(f, report)
		cerr := f.Close()
		if err != nil {
			return err
		}
		if cerr != nil {
			return cerr
		}
		fmt.Printf("Metadata-only JSON written to %s\n", output)
	}
	return nil
}
func live(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if args[0] == "probe" {
		if len(args) < 3 || args[1] != "--" {
			return fmt.Errorf("usage: brink probe -- HOST [ARGS...]")
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		c, err := acp.Start(args[2:], cwd)
		if err != nil {
			return err
		}
		defer c.Close()
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := c.Initialize(ctx); err != nil {
			return err
		}
		fmt.Println("ACP v1 initialization verified; no session or model prompt sent. Cancellation remains untested.")
		return nil
	}
	if len(args) < 4 || args[2] != "--" {
		return fmt.Errorf("usage: brink supervise CONFIG.json -- HOST [ARGS...]")
	}
	f, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	cfg := supervise.Defaults()
	if err := d.Decode(&cfg); err != nil {
		return fmt.Errorf("invalid supervision config")
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("unexpected data after supervision config")
	}
	result, err := supervise.Run(ctx, cfg, args[3:], os.Stdout)
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("task not verified: %s", result.Reason)
	}
	return nil
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "brink:", err)
		os.Exit(2)
	}
}
