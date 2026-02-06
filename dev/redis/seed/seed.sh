#!/usr/bin/env bash
set -euo pipefail

REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"

auth_args=()
if [[ -n "${REDIS_PASSWORD}" ]]; then
  auth_args=(-a "${REDIS_PASSWORD}")
fi

redis_cmd() {
  redis-cli -h "${REDIS_HOST}" -p "${REDIS_PORT}" "${auth_args[@]}" "$@"
}

redis_cmd FLUSHDB

redis_cmd SET app:status "running"
redis_cmd SET user:1:name "Alice"
redis_cmd SET user:2:name "Bob"
redis_cmd HSET user:1 profile "admin" email "alice@example.com" team "platform"
redis_cmd HSET user:2 profile "developer" email "bob@example.com" team "core"
redis_cmd LPUSH jobs:queue "backup" "sync" "cleanup"
redis_cmd SADD feature:flags "dark_mode" "beta_access" "redis_demo"
redis_cmd ZADD leaderboard 120 "alice" 180 "bob" 90 "carol"

redis_cmd EXPIRE app:status 3600
redis_cmd EXPIRE user:1:name 86400
redis_cmd EXPIRE user:2:name 86400

redis_cmd SET metrics:requests 1200
redis_cmd INCRBY metrics:requests 350
redis_cmd SET cache:hit_rate "0.92"

SEED_KEYS="${SEED_KEYS:-1000}"
echo "Seeding ${SEED_KEYS} keys per type..."

for i in $(seq 1 "${SEED_KEYS}"); do
  redis_cmd SET "bench:string:${i}" "value-${i}"
  redis_cmd HSET "bench:hash:${i}" field "value-${i}" count "${i}"
  redis_cmd LPUSH "bench:list:${i}" "item-${i}" "item-$((i + 1))"
  redis_cmd SADD "bench:set:${i}" "member-${i}" "member-$((i + 1))"
  redis_cmd ZADD "bench:zset:${i}" "${i}" "member-${i}"
done

echo "Seed data loaded into Redis at ${REDIS_HOST}:${REDIS_PORT}"
