# Contributing to omo

Thanks for helping improve omo. This guide is the short path from clone → useful PR.

## Development setup

```bash
git clone https://github.com/hatembentayeb/omo.git
cd omo
# Go 1.25+ required (see go.mod)
make all          # host + plugins → ~/.omo/plugins
./omo             # or: go run ./cmd/omo
```

Optional local stacks:

```bash
make dev-setup    # Docker-backed seeds (Redis, Kafka, …)
make dev-seed     # KeePass seeds without Docker where possible
```

## Before you open a PR

- [ ] `go mod tidy`
- [ ] `go vet ./...`
- [ ] Host builds: `go build -o /tmp/omo ./cmd/omo`
- [ ] Touched plugins build: `go build -o /tmp/p ./plugins/<name>/cmd/<name>`
- [ ] Or simply `make all` if you changed shared packages

CI runs the same class of checks on PRs (see `.github/workflows/ci.yml`).

## Project map

| Path | Role |
|------|------|
| `cmd/omo` | Host binary + `omo secrets` |
| `internal/host` | TUI host, RPC wiring, package manager |
| `pkg/pluginrpc` | Plugin RPC contract |
| `pkg/secrets` | KeePass vault |
| `pkg/ui` | Shared TUI primitives |
| `plugins/<name>` | Official plugins |
| `dev/` | Local test stacks + seed scripts |

## Adding or changing a plugin

1. Implement `pluginrpc.Plugin` (`Configure`, `GetView`, `DoAction`, …).
2. Entrypoint: `plugins/<name>/cmd/<name>`.
3. Register in `plugins.meta.yaml`.
4. Prefer returning data (`ViewData`); leave rendering to the host.
5. Ship `dev/<name>/setup.sh` (and KeePass seed) so reviewers can run it.
6. Never call back into host secrets from inside `GetView` over nested RPC.

Good reference: `plugins/redis/`.

## Commit / PR style

- Use focused commits and PRs (`feat/…`, `fix/…`, `docs/…`).
- Explain **why** in the PR body; list how you tested.
- Do not commit secrets, key files, or local `~/.omo` state.

## Security

If you find a vulnerability in credential handling or plugin isolation, please open a private security advisory or email the maintainer via the website rather than filing a public issue with exploit details.

## Code of conduct

Be kind, assume good intent, and keep discussion technical. Harassment or spam will be removed.
