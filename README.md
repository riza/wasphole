# wasphole

An MCP honeypot that impersonates a real server to detect and profile malicious AI agents. wasphole generates a unique fake Linux or Windows environment, injects canary tokens into every response, detects behavioral patterns (recon, escalation, exfiltration, retry), and fires alerts to configurable sinks — all over the standard MCP protocol.

## How it works

- Presents as a genuine MCP server: tools/list, resources/list, and tool calls all respond normally
- OS environment (files, processes, registry, network state) generated from a deterministic seed — consistent across restarts, unique per deployment
- AI-generated server identity and tool responses (Anthropic or OpenAI-compatible)
- Canary tokens embedded in every response; token fires are attributed back to the originating session
- Behavioral patterns detected per session; alerts fire to stdout, webhook, or SIEM

## Prerequisites

- Go 1.25+
- An Anthropic or OpenAI-compatible API key

## Installation

```bash
git clone https://github.com/riza/wasphole
cd wasphole
go build -o wasphole ./cmd/wasphole
cp config.yaml.example config.yaml
# Edit config.yaml: set ai.api_key or export ANTHROPIC_API_KEY
```

## Running

```bash
# HTTP transport — AI agents connect via HTTP+SSE (default)
./wasphole -config config.yaml

# stdio transport — for Claude Desktop or direct MCP client integration
# In Claude Desktop's MCP server config, set the command to:
#   /path/to/wasphole -config /path/to/config.yaml
```

## Configuration

See `config.yaml.example` for the full annotated reference. Key sections:

| Section | Purpose |
|---------|---------|
| `instance` | OS mode (`linux`/`windows`), industry context, random seed |
| `ai` | API provider, model, cache directory |
| `transport` | `stdio` or `http` (port configurable) |
| `alerts` | Sink types: stdout JSON, signed webhook, SIEM CEF |
| `canary` | DNS canary domain, HTTP callback URL, built-in listener address |
| `session` | Log directory for JSONL session recordings |

### Alert sinks

```yaml
alerts:
  sinks:
    - type: stdout      # JSON to stdout — useful for piping to jq
      enabled: true
    - type: webhook
      url: https://hooks.example.com/alerts
      webhook_secret: ""   # optional Base64 HMAC secret (Standard Webhooks spec)
      enabled: false
    - type: siem
      format: cef          # "cef" or "json"
      path: /var/log/wasphole/siem.log   # empty = stderr
      enabled: false
```

## Alert levels and patterns

| Level | Pattern | Trigger |
|-------|---------|---------|
| `INFO` | `connection` | Any new MCP session connected |
| `WARN` | `recon` | 3+ distinct system-path reads in one session |
| `WARN` | `exfil` | 5+ reads within 60s, or any read of a credential path |
| `WARN` | `retry` | 3+ consecutive errors on the same tool |
| `CRITICAL` | `escalation` | Read + write + execute tools all used in same session |
| `CRITICAL` | `canary` | Canary token fired (exfiltrated and accessed externally) |

## Example alert output

Stdout JSON format (one JSON object per line):

```json
{"level":"INFO","ts":"2026-06-03T12:00:00Z","session_id":"abc12345","pattern":"connection","message":"MCP session connected"}
{"level":"WARN","ts":"2026-06-03T12:00:05Z","session_id":"abc12345","pattern":"recon","message":"recon pattern: 3 distinct system paths accessed","count":3}
{"level":"WARN","ts":"2026-06-03T12:00:08Z","session_id":"abc12345","pattern":"exfil","message":"exfiltration pattern: credential path accessed via read_file"}
{"level":"CRITICAL","ts":"2026-06-03T12:00:10Z","session_id":"abc12345","pattern":"canary","message":"canary token fired: wh-abc12345-3f9d1a2b"}
```

CEF format (SIEM sink, `format: cef`):

```
CEF:0|wasphole|wasphole|1.0|WARN|recon pattern: 3 distinct system paths accessed|7|sessionId=abc12345 msg=recon pattern: 3 distinct system paths accessed pattern=recon cnt=3
```

## Session replay

Sessions are recorded as JSONL in the configured `session.log_dir` (default `./sessions/`). Each file is named `{session_id}.jsonl` and contains timestamped request/response entries.

```bash
ls sessions/
cat sessions/abc12345.jsonl | jq .
```

Each entry includes `session_id`, `type` (request/response/error), `method`, `ts_ms`, and the raw params or result.

## Canary tokens

Three token types are injected into responses:

- **HTTP canary** — a URL like `http://canary.example.com/c/wh-{session}-{rand}` that fires when accessed
- **Credential canary** — a fake API key like `wh_{session8}_{rand16}` embedded as a credential
- **DNS canary** — a subdomain like `{session}.canary.example.com` embedded in responses

Configure `canary.listen_addr` (e.g. `":9090"`) to enable the built-in HTTP listener, which attributes token fires to sessions without needing an external callback server.
