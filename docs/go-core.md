# Go core migration

This port adds a dependency-free Go 1.21+ replay engine and CLI alongside the
Python v0.1 reference. The initial React/Next.js Reactor trace was replayed in
both implementations and the resulting structured JSON was exactly equal
(96 model calls, 108 tool calls, 11 advisory alerts).

## Commands

```bash
go test ./...
go run ./cmd/brink replay "$HOME/Downloads/zcode-full-session(1).zip" --json go-report.json
go build -o brink-cli ./cmd/brink
```

## Boundaries

- The ZIP reader only loads metadata and command strings transiently for classification.
  Reports never contain commands, prompts, tool output, raw model reasoning, or flags.
- This is still **retrospective**. No live observer, model cancellation, or X11
  automation has been implemented.
- Preserve the Python replay until the Go build has also been verified on
  the intended host.
- Next milestone: read-only live ZCode adapter and incremental event IDs.
