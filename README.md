<p align="center">
  <img src="logo.svg" alt="omo" width="400">
</p>

<h3 align="center">Your entire infrastructure. One terminal.</h3>

<p align="center">
  <a href="https://github.com/hatembentayeb/omo/releases"><img src="https://img.shields.io/github/v/release/hatembentayeb/omo?style=flat-square&color=22d3ee" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/hatembentayeb/omo"><img src="https://goreportcard.com/badge/github.com/hatembentayeb/omo?style=flat-square" alt="Go Report Card"></a>
  <a href="https://github.com/hatembentayeb/omo/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue?style=flat-square" alt="License"></a>
  <a href="https://github.com/hatembentayeb/omo/releases"><img src="https://img.shields.io/github/downloads/hatembentayeb/omo/total?style=flat-square&color=4ade80" alt="Downloads"></a>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey?style=flat-square" alt="Platform">
</p>

<p align="center">
  <a href="https://oh-myops.com">Website</a> &bull;
  <a href="#quick-start">Quick Start</a> &bull;
  <a href="#plugins">Plugins</a> &bull;
  <a href="#keepass-setup">KeePass Setup</a> &bull;
  <a href="#contributing">Contributing</a>
</p>

---

## What is omo?

**omo** is an operations dashboard that runs entirely in your terminal. Instead of juggling browser tabs, CLI tools, and dashboards, omo gives you a single keyboard-driven interface to manage your entire infrastructure.

One binary. One KeePass file. Every service you run.

---

## Quick Start

### 1. Install omo

```bash
curl -fsSL https://raw.githubusercontent.com/hatembentayeb/omo/main/install.sh | bash
```

This downloads the latest release binary and creates the `~/.omo/` directory.

### 2. Launch omo

```bash
omo
```

On first launch, omo **automatically**:
- Generates a master key file at `~/.omo/keys/omo.key`
- Creates a KeePass database at `~/.omo/secrets/omo.kdbx` (secured by the key file)

No password prompts, no manual setup. The key file authenticates everything.

> **Important:** Back up `~/.omo/keys/omo.key` — it's the only way to unlock your secrets database. If you lose it, you lose access to all stored credentials.

### 3. Install plugins

Once inside omo:

1. Press `p` from the plugins list (or **Tab** to the bottom actions list and select **Package Manager**)
2. Press `S` to sync the plugin index from GitHub
3. Press `A` to install all plugins
4. Press `Q` to go back

### 4. Add your first connection

All plugin configurations live in the KeePass database. The path structure is:

```
<plugin-name>/<environment>/<instance-name>
```

Open the KeePass database with KeePassXC (using the key file for authentication):

1. Open `~/.omo/secrets/omo.kdbx` in KeePassXC
2. When prompted, select **Key File** and point to `~/.omo/keys/omo.key`
3. Inside the `omo` root group, create a group hierarchy: e.g. `redis` → `development`
4. Create an entry named `local` with:
   - **URL**: `localhost` (the host)
   - **Username**: Redis ACL username (leave empty if none)
   - **Password**: Redis password (leave empty if none)
   - Add a custom attribute `port` = `6379`

Plugins also auto-create placeholder entries on first run to guide you.

### 5. Use the plugin

Select the plugin from the sidebar — it auto-discovers your KeePass entries and connects.

---

## How It Works

```
┌──────────────────────────────────────────────┐
│  KeePass (secrets/omo.kdbx + keys/omo.key)   │
│  └── redis/production/cache-01               │
│  └── docker/development/local                │
│  └── kafka/staging/cluster-1                 │
│  └── ...                                     │
└──────────────┬───────────────────────────────┘
               │ pluginapi.Secrets()
┌──────────────▼───────────────────────────────┐
│  omo (host binary)                           │
│  ├── Auto-bootstrap KeePass on first run     │
│  ├── Plugin loader                           │
│  ├── Tab/Shift+Tab panel cycling             │
│  └── Per-plugin logging (~/.omo/logs/)       │
└──────────────┬───────────────────────────────┘
               │ plugin.Open()
┌──────────────▼───────────────────────────────┐
│  Plugins (.so shared libraries)              │
│  redis.so  docker.so  kafka.so  ...          │
└──────────────────────────────────────────────┘
```

---

## Plugins

omo ships with 12 official plugins:

| Plugin | Description | KeePass Path |
|--------|-------------|-------------|
| **docker** | Containers, images, networks, volumes, compose | `docker/<env>/<host>` |
| **redis** | Keys, memory, clients, slowlog, pub/sub | `redis/<env>/<instance>` |
| **kafka** | Brokers, topics, partitions, consumer groups | `kafka/<env>/<cluster>` |
| **rabbitmq** | Queues, exchanges, bindings, connections | `rabbitmq/<env>/<instance>` |
| **postgres** | Databases, users, queries, replication | `postgres/<env>/<instance>` |
| **ssh** | Remote servers, execution, system monitoring | `ssh/<env>/<server>` |
| **argocd** | Applications, projects, accounts, RBAC | `argocd/<env>/<instance>` |
| **k8suser** | Kubernetes user & certificate management | `k8suser/<env>/<cluster>` |
| **awsCosts** | Cost explorer, budgets, forecasts | `awsCosts/<env>/<profile>` |
| **s3** | Buckets, objects, upload, download | `s3/<env>/<profile>` |
| **git** | Repositories, branches, commits, diffs | `git/<env>/<repo>` |
| **sysprocess** | Processes, CPU, memory, disk, ports | *(no config needed)* |

---

## KeePass Setup

omo creates and manages the KeePass database automatically. You only need to interact with it when adding or editing connections.

### Database location

| File | Purpose |
|------|---------|
| `~/.omo/secrets/omo.kdbx` | KeePass KDBX4 database (all credentials) |
| `~/.omo/keys/omo.key` | Master key file (auto-generated, **back this up!**) |

To open the database in KeePassXC, select "Key File" authentication and point to `~/.omo/keys/omo.key`.

### Entry fields

Each KeePass entry maps to a connection. Plugins use standard KeePass fields plus custom attributes:

| KeePass Field | Used For |
|---------------|----------|
| **Title** | Display name |
| **URL** | Host / endpoint |
| **UserName** | Username |
| **Password** | Password / token |
| **Notes** | Description |
| Custom attributes | Plugin-specific (e.g. `port`, `database`, `ssl_mode`) |

Empty fields are ignored — only fill in what your service needs.

### Example: Redis

| Field | Value |
|-------|-------|
| Path | `redis/production/cache-main` |
| Title | `cache-main` |
| URL | `redis.example.com` |
| Password | `your-redis-password` |
| Custom: `port` | `6379` |
| Custom: `database` | `0` |

### Example: PostgreSQL

| Field | Value |
|-------|-------|
| Path | `postgres/production/app-db` |
| Title | `app-db` |
| URL | `db.example.com` |
| UserName | `admin` |
| Password | `your-db-password` |
| Custom: `port` | `5432` |
| Custom: `database` | `myapp` |
| Custom: `ssl_mode` | `require` |

### Example: Docker

| Field | Value |
|-------|-------|
| Path | `docker/development/local` |
| Title | `local` |
| URL | `unix:///var/run/docker.sock` |

### Example: SSH

| Field | Value |
|-------|-------|
| Path | `ssh/production/web-01` |
| Title | `web-01` |
| URL | `10.0.1.50` |
| UserName | `deploy` |
| Custom: `port` | `22` |
| Custom: `auth_method` | `key` |
| Custom: `private_key_path` | `~/.ssh/id_ed25519` |

---

## Keyboard Shortcuts

### Global

| Key | Action |
|-----|--------|
| **Tab** | Cycle focus: plugins → main → actions |
| **Shift+Tab** | Cycle focus in reverse |
| **↑ / ↓** | Navigate lists |
| **Enter** | Select item |
| **r** | Refresh plugins *(when sidebar focused)* |
| **p** | Open Package Manager *(when sidebar focused)* |

### Inside plugins

| Key | Action |
|-----|--------|
| **Ctrl+T** | Switch instance / connection |
| **R** | Refresh data |
| **/** | Filter / search |
| **?** | Show plugin help |
| **Esc** | Go back |

Each plugin has its own keybindings — press `?` inside any plugin to see them.

---

## Directory Structure

```
~/.omo/
  secrets/
    omo.kdbx            KeePass database (all credentials)
  keys/
    omo.key             Master key file (auto-generated)
  index.yaml            Plugin index (auto-managed)
  installed.yaml        Installed plugin versions
  logs/
    omo.log             Main app log
    redis.log           Per-plugin logs
    docker.log
    ...
  plugins/
    redis/redis.so      Plugin binaries
    docker/docker.so
    ...
```

---

## Building from Source

```bash
git clone https://github.com/hatembentayeb/omo.git
cd omo
make all
```

This builds the `omo` binary (with version injected) and all 12 plugins, then installs everything to `~/.omo/`.

### Requirements

- **Linux** (required for Go plugin `.so` support)
- **Go 1.25+** (only if building from source)
- **KeePassXC** (optional — only needed to manually view/edit `secrets/omo.kdbx`; omo manages the database automatically)

### Development setup

```bash
# Start Redis + Kafka containers and seed KeePass entries
make dev-setup

# Or seed only non-Docker plugins
make dev-seed
```

---

## Build Matrix

| OS | Arch | Binary | Plugins |
|----|------|--------|---------|
| Linux | amd64 | ✅ | ✅ `.so` |
| Linux | arm64 | ✅ | ✅ `.so` |
| macOS | amd64 | ✅ | ❌ |
| macOS | arm64 | ✅ | ❌ |
| Windows | amd64 | ✅ | ❌ |

> Go's `plugin` build mode only supports Linux. macOS and Windows can run the core binary but cannot load `.so` plugins.

---

## Roadmap

- [ ] `omo secrets` CLI for managing KeePass entries without a GUI
- [ ] Plugin SDK v2 with richer lifecycle hooks
- [ ] Remote plugin loading (WASM or gRPC)
- [ ] Prometheus / Grafana plugin
- [ ] Theme and color customization

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-change`)
3. Ensure `go vet ./...` and `go build ./...` pass
4. Submit a pull request

For plugin contributions, include a `dev/` setup script and KeePass seed data so reviewers can test locally.

---

## License

Apache License 2.0. See [LICENSE](LICENSE) for the full text.

---

<p align="center">
  <sub>Built by <a href="https://oh-myops.com">Hatem Ben Tayeb</a></sub>
</p>
