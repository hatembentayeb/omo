<p align="center">
  <img src="logo.svg" alt="OhMyOps Logo" width="300">
</p>

<p align="center">
  <strong>Open Source Operations Platform</strong>
</p>

<p align="center">
  <a href="https://oh-myops.com">Website</a> | 
  <a href="#installation">Installation</a> | 
  <a href="#plugins">Plugins</a>
</p>

<p align="center">
  <img src="assets/screenshot.png" alt="OhMyOps Docker plugin screenshot" width="700">
</p>

---

**OhMyOps (omo)** is your complete operations dashboard, right in your terminal. Manage your infrastructure without leaving the command line.

## Features

- 🖥️ **Unified Dashboard**: Fast, keyboard-driven interface built in Go.
- 🔌 **Plugin System**: Extensible architecture with built-in Package Manager.
- 🔐 **Secure Secrets**: Integrates with KeePass (KDBX4) to keep credentials out of config files.
- 🚀 **Fast**: Written in Go, launches instantly.

## Plugins

- **ArgoCD**: Applications, projects, sync status.
- **AWS Costs**: Cost explorer, budgets, forecasts.
- **Docker**: Containers, images, networks, volumes.
- **Git**: Repositories, branches, commits, diffs.
- **K8s User**: Manage Kubeconfig users.
- **Kafka**: Brokers, topics, partitions, consumer groups.
- **Postgres**: Databases, schemas, tables, connections, replication.
- **Redis**: Keys, clients, memory, slowlog, pubsub.
- **S3**: Buckets, objects, previews.
- **Sysprocess**: Processes, CPU, memory, disk, ports, metrics.

## Installation

```bash
git clone https://github.com/ohmyops/omo.git
cd omo
make all
./omo
```

See [oh-myops.com](https://oh-myops.com) for more details.
