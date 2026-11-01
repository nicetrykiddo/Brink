# Go runtime and historical replay

The dependency-free Go CLI now supports live ACP supervision as well as historical replay. Start with the [README](../README.md), [architecture](architecture.md) and [validation record](validation.md).

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/brink ./cmd/brink
/tmp/brink probe -- opencode acp
```

The original Python and Go replayers matched on the private Reactor export: 96 model records, 108 tools and 11 retrospective alerts. Those heuristics remain diagnostic fixtures and are not the live supervision policy. Python is not required for the live runtime.

Live control uses independent external evidence checks and confirmed turn settlement before resumption. Runtime-specific logs, SQLite layouts and desktop GUI control are not its API. See [STATUS](STATUS.md) for current capabilities and remaining work.
