# Redis Dev Environment

This directory provides a local Redis instance and a seed script for demo data.

## Start Redis

```bash
docker compose -f dev/redis/docker-compose.yml up -d
```

## Seed Data

```bash
bash dev/redis/seed/seed.sh
```

## Stop Redis

```bash
docker compose -f dev/redis/docker-compose.yml down -v
```
