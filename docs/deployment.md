# Deployment

---

## Production checklist

**Network isolation**

- Run wasphole in a dedicated VPC, subnet, or VM. It is deception infrastructure — treat it as untrusted.
- Do not co-locate wasphole with production systems or systems that hold real credentials.
- Firewall inbound access to only the ports wasphole needs to operate.

**TLS**

- Always use TLS for internet-facing deployments. See [transports.md](transports.md) for setup options.
- If terminating TLS at a reverse proxy (nginx, Caddy, Traefik), run wasphole in plain HTTP mode on a loopback or private network address.

**Secrets**

- Never put real API keys in `config.yaml`. Use the `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` environment variable instead.
- The `ai.api_key` field exists for convenience in isolated environments; prefer env vars in production.

**Canary domain**

- Use a dedicated domain or subdomain for canary tokens (`canary.yourdomain.com`).
- Configure the domain's DNS to point to a server running wasphole's canary listener, or to an external callback service.
- Monitor the domain's DNS query logs for DNS canary hits even when the HTTP listener is not reachable.

**Session logs**

- Session JSONL files contain the full content of every MCP request and response, including any canary credentials that were returned.
- Treat session logs as sensitive. Apply appropriate file permissions and rotate or archive them regularly.

**Let's Encrypt**

- Ensure ports 80 and 443 are open before starting. Let's Encrypt's HTTP-01 challenge requires inbound access on port 80.
- The `acme_cache_dir` must be persistent across restarts.
- Let's Encrypt enforces [rate limits](https://letsencrypt.org/docs/rate-limits/). Use a stable domain and avoid repeatedly deleting the cache directory.

---

## Safety notes

- Simulated writes (`write_file`, `run_command` output) do not touch the real filesystem.
- `/etc/shadow`-style paths return permission-denied errors rather than fake secrets, to avoid false positives from security scanners.
- Canary token IDs are random and contain no session metadata; attribution is server-side only.
- The canary proxy forwards traffic to a real upstream LLM by default. If you do not want wasphole to forward any traffic, set `proxy.upstream_url` to empty.

---

## Reverse proxy setup

Running wasphole behind nginx, Caddy, or Traefik is a common production pattern. The reverse proxy handles TLS, and wasphole runs on a plain HTTP port internally.

**nginx example:**

```nginx
server {
    listen 443 ssl;
    server_name honeypot.example.com;

    ssl_certificate     /etc/nginx/tls/cert.pem;
    ssl_certificate_key /etc/nginx/tls/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**Caddy example:**

```
honeypot.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

With `X-Forwarded-For` or `X-Real-IP` set by the proxy, wasphole correctly logs the original client IP in session connection events.
