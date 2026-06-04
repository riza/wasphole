# Development

## Prerequisites

- Go 1.25+
- `make`
- An Anthropic or OpenAI-compatible API key (for identity generation on first run)

## Building

```bash
make build    # produces ./wasphole
make test     # run all tests
make vet      # run go vet
make clean    # remove build artifacts
```

The binary embeds version metadata from git tags:

```bash
./wasphole version
# wasphole v0.1.0 (commit abc1234, built 2025-10-01T12:00:00Z)
```

## Running locally

```bash
cp config.yaml.example config.yaml
# edit config.yaml as needed

export ANTHROPIC_API_KEY="sk-ant-..."
./wasphole server -config config.yaml
```

On first run, wasphole calls the AI provider to generate the honeypot's identity (server name, persona, and business tools). The result is cached in `cache_dir` and reused on subsequent starts.

## CLI commands

```bash
./wasphole server   -config config.yaml          # start the MCP server
./wasphole sessions -dir ./sessions              # list recorded sessions
./wasphole replay   <session-id> -dir ./sessions # pretty-print a session
./wasphole version                               # print build metadata
```

## Project layout

```text
cmd/wasphole/        CLI entrypoint (server, sessions, replay, version)
internal/ai/         AI provider clients, identity generation, response cache
internal/alert/      Detection engine and alert sinks (stdout, webhook, SIEM)
internal/canary/     Canary token issuing, injection, and HTTP listener
internal/config/     YAML config loading and struct definitions
internal/mcp/        MCP server wiring, tool and resource registration
internal/session/    JSONL session recorder and replay helpers
internal/sim/        Linux and Windows OS simulation state
internal/proxy/      OpenAI-compatible canary-scanning reverse proxy
docker/              Docker runtime config and compose file
docs/                Project documentation
```

## Adding a new simulated tool

1. Define the tool schema in `internal/mcp/tools.go` using `mcplib.NewTool`.
2. Register it inside `registerLinuxTools`, `registerWindowsTools`, or `registerBizTools` depending on the target mode.
3. Implement the handler. Tools that need AI-generated responses can use `cache.Get(ctx, toolName, description, params)` — the response is generated once and cached.
4. If the tool accesses credential-like content, call `injectCredentialForPath` or `replaceExplicitCanaryPlaceholder` to plant a canary in the output.
5. Update `internal/alert/alert.go` classification maps (`readClassTools`, `writeClassTools`, `executeClassTools`) if the tool should trigger behavioral detection.

## Adding a new alert pattern

Detection logic lives in `internal/alert/alert.go` in the `sessionState.observe` method. Each pattern:
- Reads from `sessionState` fields (counters, path sets, flags)
- Returns an `Event` and `true` when the threshold is crossed
- Sets `s.alerted[PatternKind]` to true to fire at most once per session

Add a new `PatternKind` constant, extend `observe`, and add tests in the same package.

## Testing

```bash
go test ./...
```

Key test packages:

| Package | What it covers |
|---------|---------------|
| `internal/canary` | Token issuance, injection styles, lookup |
| `internal/mcp` | Tool schema shape, canary policy |
| `internal/sim` | OS simulation consistency across calls |
| `internal/proxy` | Canary scan in proxy request bodies |
| `internal/ai` | Identity generation and caching |
