# Canary tokens

wasphole plants fake credentials in places where real secrets naturally appear. When an agent reads, copies, or uses one of these values, the access is detected and attributed to the originating session.

---

## How canaries are planted

wasphole does not decorate every response with canary tokens. Instead, tokens are injected only into content where a real system would plausibly store secrets:

- Config files (`/etc/app/config.yaml`, `.env`, `appsettings.json`)
- Credential-bearing paths (`.aws/credentials`, `id_rsa`, `secrets/`)
- Process environment variables
- AI-generated business responses that explicitly include a credential placeholder

This selective approach reduces fingerprint risk — a blanket token on every response is easy to recognize.

---

## Token types

### Credential strings

Fake API keys injected directly into file content. The format varies to match the surrounding context:

| Prefix | Looks like |
|--------|-----------|
| `sk_live_` | Stripe-style secret key |
| `sk-proj-` | OpenAI-style project key |
| `pat_` | GitHub personal access token |
| `api_` | Generic API key |
| `token_` | Generic token |
| `ghp_` | GitHub personal access token |

When a file or response is returned, the canary is injected in a style that matches the content format (YAML `key: value`, env `KEY=value`, JSON `"api_key": "..."`, or plain text).

### URL canaries

When `canary.domain` is configured, every token also gets an AWS IMDS-style URL:

```
http://169.254.169.254/latest/meta-data/iam/security-credentials/<token-id>
```

If the domain is set to something like `canary.example.com`, the URL becomes:

```
http://canary.example.com/latest/meta-data/iam/security-credentials/<token-id>
```

An agent that fetches this URL (expecting real AWS credentials) triggers a `CRITICAL` canary alert and receives a believable fake credentials response to keep the deception going.

### DNS canaries

When `canary.domain` is configured, each token is also assigned a unique DNS subdomain:

```
<token-id>.canary.example.com
```

Any DNS lookup for this subdomain can be detected at the domain's authoritative nameserver, even if the HTTP listener is not reachable. DNS canaries extend coverage to environments where outbound HTTP is blocked but DNS is not.

---

## Canary listener

The built-in HTTP listener serves canary URLs and fires attribution callbacks. Enable it with `canary.listen_addr`:

```yaml
canary:
  listen_addr: ":9090"
```

When a canary URL is accessed:

1. The token ID is extracted from the URL path.
2. The token is looked up to find the originating session.
3. A `CRITICAL` alert is emitted via the alert engine.
4. If `canary.http_callback` is set, the callback is POSTed with `token_id`, `session_id`, `remote_addr`, and timestamp.
5. The response mimics the expected service. For IMDS-style paths, a fake AWS credentials JSON is returned to sustain the deception.

---

## Canary proxy

The optional reverse proxy scans outbound LLM requests for canary credentials. If an agent extracts a canary value and sends it to a language model (e.g. "here is the API key I found: sk_live_..."), the proxy detects it before the request is forwarded.

```yaml
proxy:
  listen_addr: ":8081"
  upstream_url: "https://api.openai.com"
```

Point the attacking agent's `LLM_BASE_URL` or `OPENAI_BASE_URL` at the proxy address. All traffic is forwarded transparently to `upstream_url` after scanning; or set `upstream_url` empty for a stub response (useful when you do not want to forward at all).

---

## Attribution

Token IDs are random hex values with no embedded session information. The mapping between token ID and session ID is held server-side in memory. This means:

- Captured tokens cannot be decoded to reveal session details.
- Attribution requires the wasphole server that issued the token to still be running.
- Restarting wasphole clears the token map; tokens issued in previous runs cannot be attributed.

If you need persistent attribution across restarts, configure `canary.http_callback` to record every issued token externally.
