# Configuration reference

wasphole is configured via a single YAML file. Copy `config.yaml.example` to `config.yaml` and fill in the required fields.

```bash
cp config.yaml.example config.yaml
```

All sections and their fields are described below. Fields marked **required** must be present; everything else has a default or is optional.

---

## `instance`

Controls the simulated environment.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `mode` | string | yes | Simulated OS: `linux` or `windows` |
| `industry` | string | no | Industry context for AI-generated identity and responses. Examples: `fintech`, `healthcare`, `saas`, `ecommerce` |
| `seed` | string | no | Fixed seed for deterministic identity generation. Leave empty to generate a new identity on first startup. Two deployments with different seeds produce distinct identities. |

```yaml
instance:
  mode: linux
  industry: fintech
  seed: ""
```

---

## `ai`

Controls the AI provider used for identity generation and tool responses.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_type` | string | yes | API protocol: `anthropic` or `openai` |
| `model` | string | yes | Model ID to request |
| `api_key` | string | no | API key. Falls back to `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` env var |
| `base_url` | string | no | Custom API endpoint. Leave empty for the provider's standard URL |
| `cache_dir` | string | no | Directory for caching AI responses and identity. Default: `./cache` |

```yaml
ai:
  api_type: anthropic
  model: claude-sonnet-4-6
  api_key: ""        # or set ANTHROPIC_API_KEY env var
  base_url: ""
  cache_dir: ./cache
```

**OpenAI-compatible providers** (DeepSeek, MiniMax, GLM, etc.):

```yaml
ai:
  api_type: openai
  base_url: https://api.deepseek.com/v1
  model: deepseek-chat
```

---

## `transport`

Controls how the MCP server accepts connections. See [transports.md](transports.md) for setup guides.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | yes | `stdio` or `http` |
| `port` | int | no | TCP port when `type: http`. Default: `8080` |
| `tls.cert_file` | string | no | Path to PEM certificate file |
| `tls.key_file` | string | no | Path to PEM private key file |
| `tls.acme_domain` | string | no | Domain for automatic Let's Encrypt certificate |
| `tls.acme_cache_dir` | string | no | Directory to store ACME certificates. Default: `./acme-cache` |

```yaml
transport:
  type: http
  port: 8080
  tls:
    cert_file: ""
    key_file: ""
    acme_domain: ""
    acme_cache_dir: ""
```

---

## `alerts`

One or more alert sinks. Each sink receives alert events independently. See [detection.md](detection.md) for alert patterns and output formats.

```yaml
alerts:
  sinks:
    - type: stdout
      enabled: true

    - type: webhook
      url: https://hooks.example.com/wasphole-alerts
      webhook_secret: ""   # base64 HMAC signing secret (Standard Webhooks spec)
      enabled: false

    - type: siem
      format: cef           # "cef" (ArcSight CEF) or "json"
      path: ""              # output file; empty = stderr
      enabled: false
```

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `stdout`, `webhook`, or `siem` |
| `enabled` | bool | Whether this sink is active |
| `url` | string | Webhook endpoint URL (`type: webhook` only) |
| `webhook_secret` | string | Base64-encoded HMAC signing secret for Standard Webhooks signature headers |
| `format` | string | `cef` or `json` (`type: siem` only) |
| `path` | string | Output file path (`type: siem` only); empty writes to stderr |

---

## `canary`

Controls canary token embedding and callback delivery. See [canary-tokens.md](canary-tokens.md) for how canaries work.

| Field | Type | Description |
|-------|------|-------------|
| `domain` | string | Base domain for DNS/URL canary tokens, e.g. `canary.example.com`. Leave empty to disable. |
| `http_callback` | string | URL to POST when any canary fires. Receives `token_id`, `session_id`, `remote_addr`, and timestamp. |
| `listen_addr` | string | Address for the built-in canary HTTP listener, e.g. `:9090`. Leave empty to disable. |

```yaml
canary:
  domain: canary.example.com
  http_callback: https://hooks.example.com/canary
  listen_addr: ""
```

---

## `proxy`

Optional OpenAI-compatible reverse proxy. When an attacking agent points its `LLM_BASE_URL` here, wasphole scans every outbound chat completion request for known canary credentials.

| Field | Type | Description |
|-------|------|-------------|
| `listen_addr` | string | Address to listen on, e.g. `:8081`. Leave empty to disable. |
| `upstream_url` | string | Real LLM API to forward requests to after scanning. Leave empty for a stub response. |

```yaml
proxy:
  listen_addr: ""
  upstream_url: ""
```

---

## `session`

Controls where session logs are written.

| Field | Type | Description |
|-------|------|-------------|
| `log_dir` | string | Directory for per-session JSONL logs. Each session writes to `{log_dir}/{session_id}.jsonl`. Default: `./sessions` |

```yaml
session:
  log_dir: ./sessions
```
