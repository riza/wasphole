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

## Use cases

- **Prompt-injection labs** — watch an injected agent enumerate tools, read resources, and hunt for secrets.
- **Agent security evaluation** — compare how different models or agent frameworks behave against the same MCP surface.
- **MCP deception** — expose believable fake tools, files, databases, processes, logs, registry keys, and business workflows.
- **Canary monitoring** — plant fake credentials only where secrets naturally belong and alert when they are used.
- **Red-team telemetry** — record complete MCP sessions as JSONL for replay, research, and detection tuning.

## Quickstart

```bash
git clone https://github.com/riza/wasphole
cd wasphole

export ANTHROPIC_API_KEY="sk-ant-..."
docker compose up --build
```

The MCP server starts on port `8080`. Point your agent or MCP client at `http://localhost:8080`.

**From source:**

```bash
make build
./wasphole setup
./wasphole server -config config.yaml
```

## Documentation

| Topic | Description |
|-------|-------------|
| [Configuration](docs/configuration.md) | Full YAML reference for all config sections |
| [Transports](docs/transports.md) | stdio, HTTP, TLS with custom cert or Let's Encrypt |
| [Detection](docs/detection.md) | Alert patterns, levels, and sink configuration |
| [Canary tokens](docs/canary-tokens.md) | How canaries are planted, attributed, and delivered |
| [Deployment](docs/deployment.md) | Docker, production checklist, reverse proxy setup |
| [Development](docs/development.md) | Build, test, project layout, extending wasphole |

## Links

- X: [@rizasabuncu](https://x.com/rizasabuncu)
- Support: [buy me a coffee](https://buymeacoffee.com/rizasabuncu)

## License

Apache-2.0
