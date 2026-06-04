# Transports

wasphole supports two transport modes: `stdio` for direct process integration (Claude Desktop, IDE extensions) and `http` for network-accessible deployments. Set `transport.type` in `config.yaml` to choose.

---

## stdio

The MCP server communicates over stdin/stdout. No port is opened. This is the correct mode for Claude Desktop, Claude Code, and most IDE MCP integrations.

```yaml
transport:
  type: stdio
```

**Claude Desktop** — add wasphole to `claude_desktop_config.json`:

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

**Claude Code** — add to `.claude/settings.json` in your project:

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

---

## HTTP (plain)

The MCP server listens on a TCP port using the MCP streamable HTTP transport. Use this for remote agents, Docker deployments, or any scenario where you need a network endpoint.

```yaml
transport:
  type: http
  port: 8080
```

```bash
./wasphole server -config config.yaml
# wasphole ready  transport=http  addr=:8080
```

---

## HTTP + TLS (custom certificate)

Provide a PEM certificate and key to enable HTTPS. Works with self-signed certificates, certificates from a CA, or certificates issued by Let's Encrypt via an external tool (e.g. `certbot`).

```yaml
transport:
  type: http
  port: 8443
  tls:
    cert_file: /etc/wasphole/tls/cert.pem
    key_file:  /etc/wasphole/tls/key.pem
```

Both `cert_file` and `key_file` must be provided together. wasphole will not start if one is missing.

**Generating a self-signed certificate for local testing:**

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem \
  -days 365 -nodes -subj "/CN=localhost"
```

---

## HTTP + TLS (automatic Let's Encrypt)

wasphole can provision and automatically renew a certificate from Let's Encrypt using the ACME protocol. No manual certificate management required.

```yaml
transport:
  type: http
  tls:
    acme_domain:    honeypot.example.com
    acme_cache_dir: ./acme-cache        # optional, defaults to ./acme-cache
```

**Requirements:**

- The domain must resolve to the server's public IP before starting.
- Port `80` must be reachable from the internet (for the HTTP-01 challenge).
- Port `443` must be reachable from the internet (for HTTPS).

When `acme_domain` is set, the `port` field is ignored — the server always listens on `:443`. Port `80` is opened only to answer Let's Encrypt's ownership challenge and redirects all other traffic.

Certificates are cached in `acme_cache_dir`. On restart, the cached certificate is reused and renewed automatically before it expires.

**Docker with Let's Encrypt:**

```yaml
# docker/config.yaml
transport:
  type: http
  tls:
    acme_domain: honeypot.example.com
    acme_cache_dir: /data/acme-cache
```

```yaml
# docker-compose.yml (excerpt)
ports:
  - "80:80"
  - "443:443"
volumes:
  - acme-cache:/data/acme-cache
```

---

## Choosing a transport

| Scenario | Transport |
|----------|-----------|
| Claude Desktop / Claude Code integration | `stdio` |
| Local Docker testing | `http` on port 8080 |
| Internet-facing honeypot | `http` + TLS (custom cert or Let's Encrypt) |
| Behind a reverse proxy (nginx, Caddy) | `http` plain — let the proxy terminate TLS |
