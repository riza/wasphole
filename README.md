# wasphole

[![CI](https://github.com/riza/wasphole/actions/workflows/ci.yml/badge.svg)](https://github.com/riza/wasphole/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/riza/wasphole?sort=semver)](https://github.com/riza/wasphole/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/riza/wasphole)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/riza/wasphole)](https://goreportcard.com/report/github.com/riza/wasphole)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)

```
                .' '.            __
       .        .   .           (__\_
        .         .         . -{{_(|8)
          ' .  . ' ' .  . '     (__/
```

> Open-source MCP honeypot with fake tools, canary credentials, session replay, and alerting.

wasphole impersonates a believable MCP server so you can observe how AI agents behave when they discover tools, read resources, probe files, retry errors, or exfiltrate planted credentials.

Use it to test prompt-injection fallout, evaluate agent behavior, and collect MCP-native security telemetry before suspicious agents reach real systems.

## Use cases

- **Prompt-injection labs**: watch an injected agent enumerate tools, read resources, and hunt for secrets.
- **Agent security evaluation**: compare how different models or agent frameworks behave against the same MCP surface.
- **MCP deception**: expose believable fake tools, files, databases, processes, logs, registry keys, and business workflows.
- **Canary monitoring**: plant fake credentials only where secrets naturally belong and alert when they are used.
- **Red-team telemetry**: record complete MCP sessions as JSONL for replay, research, and detection tuning.

## Quickstart with Docker

```bash
git clone https://github.com/riza/wasphole
cd wasphole

export ANTHROPIC_API_KEY="sk-ant-..."
docker compose up --build
```

The default Docker config runs streamable HTTP MCP on port `8080`, stores cache in `/data/cache`, and writes session logs to `/data/sessions`.

To customize behavior, edit [`docker/config.yaml`](docker/config.yaml) or mount your own config. Until published container images are available, build locally:

```bash
docker build -t wasphole:local .
docker run --rm -p 8080:8080 \
  -e ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY" \
  -v "$PWD/docker/config.yaml:/etc/wasphole/config.yaml:ro" \
  wasphole:local
```

## Quickstart from source

Prerequisites:

- Go 1.25+
- Anthropic API key or OpenAI-compatible API key

```bash
git clone https://github.com/riza/wasphole
cd wasphole

cp config.yaml.example config.yaml
export ANTHROPIC_API_KEY="sk-ant-..."

make build
./wasphole server -config config.yaml
```

Useful commands:

```bash
make test
make vet
./wasphole version
./wasphole sessions -dir ./sessions
./wasphole replay <session-id> -dir ./sessions
```

## Claude Desktop / stdio

Set `transport.type: stdio` in `config.yaml`, then add wasphole as an MCP server:

```json
{
  "mcpServers": {
    "wasphole": {
      "command": "/absolute/path/to/wasphole",
      "args": ["server", "-config", "/absolute/path/to/config.yaml"]
    }
  }
}
```

## HTTP transport

Set `transport.type: http` and choose a port:

```yaml
transport:
  type: http
  port: 8080
```

Then run:

```bash
./wasphole server -config config.yaml
```

## What it simulates

wasphole combines deterministic OS simulation with AI-generated business context.

| Area | Examples |
|------|----------|
| MCP surface | `tools/list`, `resources/list`, stdio, streamable HTTP |
| Linux tools | shell execution, file reads, SQL queries, fake writes, process listing |
| Windows tools | command execution, registry reads, WMI queries, fake writes, tasklist-style output |
| Business tools | generated tools such as `get_orders` and `set_order_type` with discoverable required values |
| Resources | config files, logs, app metadata, fake secret-bearing paths |
| Stability | generated identity and responses are cached for consistency across restarts |

## Detection

wasphole classifies behavior per MCP session and emits alerts to stdout, webhooks, or SIEM output.

| Level | Pattern | Trigger |
|-------|---------|---------|
| `INFO` | `connection` | New MCP session |
| `WARN` | `recon` | Repeated system/resource discovery |
| `WARN` | `exfil` | Credential-path access or bulk reads |
| `WARN` | `retry` | Repeated errors on the same tool |
| `CRITICAL` | `escalation` | Read + write + execute behavior in one session |
| `CRITICAL` | `canary` | A planted canary token is fired |

Example:

```json
{"level":"WARN","pattern":"recon","message":"recon pattern: 3 distinct system paths accessed","count":3}
{"level":"CRITICAL","pattern":"canary","message":"canary token fired: 9fd21a2b7c4e"}
```

## Canary tokens

wasphole does not decorate every response with canaries. It plants fake credentials only in natural secret-bearing surfaces such as config files, `.env`-style paths, credential paths, process environments, or AI-generated business responses that explicitly request a canary placeholder.

Supported canary surfaces include:

- credential-like tokens such as `sk_live_`, `sk-proj-`, `pat_`, `api_`, `token_`, and `ghp_`
- metadata URL canaries when `canary.domain` is configured
- built-in HTTP listener callbacks via `canary.listen_addr`

## Configuration

See [`config.yaml.example`](config.yaml.example) for the full annotated reference.

| Section | Purpose |
|---------|---------|
| `instance` | Simulated OS mode, industry context, deterministic seed |
| `ai` | Anthropic or OpenAI-compatible provider settings |
| `transport` | `stdio` or `http` |
| `alerts` | stdout, webhook, and SIEM sinks |
| `canary` | DNS/URL canary generation and callback listener |
| `session` | JSONL session recording directory |

## Safety notes

- Simulated writes do not touch the real filesystem.
- `/etc/shadow`-style access returns permission errors instead of fake secrets.
- Canary token IDs do not embed session IDs; attribution is server-side.
- Do not put real secrets in config files; prefer environment variables.
- Treat internet-facing deployments as deception infrastructure and isolate them accordingly.

## Development

```bash
make build
make test
make vet
make clean
```

Project layout:

```text
cmd/wasphole/        CLI entrypoint
internal/ai/         provider clients, identity generation, response cache
internal/alert/      detection engine and alert sinks
internal/canary/     token issuing, injection, listener
internal/config/     YAML config loading and validation
internal/mcp/        MCP server, tools, resources, latency hooks
internal/session/    JSONL session recorder and replay helpers
internal/sim/        Linux/Windows simulation state
docker/              Docker runtime config
landing/             static landing page prototype
```

## Project status

wasphole is experimental security tooling. APIs, generated surfaces, and detection rules may change quickly while MCP security patterns evolve.

## Links

- X: [@riza](https://x.com/riza)
- Support: [buy me a coffee](https://buymeacoffee.com/riza)

## License

Apache-2.0
