# Detection

wasphole classifies behavior per MCP session and emits structured alerts when suspicious patterns are detected. Alerts are delivered to one or more configured sinks (stdout, webhook, SIEM).

---

## Alert levels

| Level | Meaning |
|-------|---------|
| `INFO` | Normal session lifecycle event |
| `WARN` | Suspicious behavior worth investigating |
| `CRITICAL` | High-confidence malicious activity |

---

## Behavioral patterns

| Level | Pattern | Trigger |
|-------|---------|---------|
| `INFO` | `connection` | A new MCP session was established |
| `WARN` | `recon` | 3 or more distinct system paths accessed in one session (`/etc/`, `/proc/`, `.ssh`, `HKEY_`, etc.) |
| `WARN` | `exfil` | A credential-bearing path was read (`password`, `secret`, `api_key`, `.env`, `DATABASE_URL`, etc.), or 5+ reads within 60 seconds |
| `WARN` | `retry` | 3 or more consecutive errors on the same tool |
| `CRITICAL` | `escalation` | Read, write, and execute actions all occurred within the same session |
| `CRITICAL` | `canary` | A planted canary token was accessed or used |

Each pattern fires at most once per session to avoid alert fatigue.

---

## Alert payload

Every alert event is a JSON object:

```json
{
  "level": "WARN",
  "ts": "2025-10-01T14:23:11.452Z",
  "session_id": "abc123",
  "pattern": "recon",
  "message": "recon pattern: 3 distinct system paths accessed",
  "count": 3
}
```

| Field | Description |
|-------|-------------|
| `level` | `INFO`, `WARN`, or `CRITICAL` |
| `ts` | RFC 3339 timestamp |
| `session_id` | MCP session identifier |
| `pattern` | Pattern kind (see table above) |
| `message` | Human-readable description |
| `count` | Relevant count when applicable (reads, errors, paths) |

---

## Sinks

### stdout

Prints one JSON line per event to stdout. Always useful as a baseline — pair with `jq` for filtering:

```bash
./wasphole server -config config.yaml | jq 'select(.level == "CRITICAL")'
```

```yaml
alerts:
  sinks:
    - type: stdout
      enabled: true
```

### webhook

POSTs the JSON event payload to a URL. Optionally signs the request using the [Standard Webhooks](https://www.standardwebhooks.com/) specification (HMAC-SHA256).

```yaml
alerts:
  sinks:
    - type: webhook
      url: https://hooks.example.com/wasphole-alerts
      webhook_secret: ""   # base64-encoded signing secret; empty = no signature
      enabled: true
```

When `webhook_secret` is set, each request includes:

```
webhook-id: <uuid>
webhook-signature: v1,<base64-hmac>
webhook-timestamp: <unix-seconds>
```

### SIEM

Writes structured events to a file or stderr for ingestion by a SIEM (Splunk, Elasticsearch, QRadar, etc.).

```yaml
alerts:
  sinks:
    - type: siem
      format: cef          # "cef" or "json"
      path: /var/log/wasphole/siem.log
      enabled: true
```

**CEF format** (ArcSight Common Event Format):

```
CEF:0|wasphole|wasphole|1.0|WARN|recon pattern: 3 distinct system paths accessed|7|sessionId=abc123 msg=recon pattern: 3 distinct system paths accessed pattern=recon cnt=3
```

**JSON format** — same payload as the webhook sink, one object per line.

Multiple sinks can be active simultaneously. A `CRITICAL` canary alert, for example, can go to stdout for immediate visibility, to a webhook for PagerDuty/Slack notification, and to a SIEM file for long-term retention — all at the same time.
