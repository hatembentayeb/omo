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

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Redis PubSub Test ===${NC}"
echo ""

# Function to cleanup background processes
cleanup() {
  echo -e "\n${YELLOW}Cleaning up subscribers...${NC}"
  jobs -p | xargs -r kill 2>/dev/null || true
  wait 2>/dev/null || true
  echo -e "${GREEN}Done!${NC}"
}
trap cleanup EXIT

# Start subscribers in background (these create the active channels)
echo -e "${YELLOW}Starting subscribers on channels...${NC}"

# Subscribe to multiple channels (runs in background)
redis_cmd SUBSCRIBE events:user events:system events:metrics > /dev/null 2>&1 &
SUB1_PID=$!
echo -e "  ${GREEN}✓${NC} Subscriber 1: events:user, events:system, events:metrics"

redis_cmd SUBSCRIBE notifications alerts > /dev/null 2>&1 &
SUB2_PID=$!
echo -e "  ${GREEN}✓${NC} Subscriber 2: notifications, alerts"

# Pattern subscriber
redis_cmd PSUBSCRIBE "app:*" "logs:*" > /dev/null 2>&1 &
SUB3_PID=$!
echo -e "  ${GREEN}✓${NC} Subscriber 3: app:* (pattern), logs:* (pattern)"

# Give subscribers time to connect
sleep 1

echo ""
echo -e "${GREEN}Channels are now active!${NC}"
echo -e "Open your app and press ${YELLOW}B${NC} to view PubSub channels."
echo ""
echo -e "${BLUE}Publishing test messages every 2 seconds...${NC}"
echo -e "(Press Ctrl+C to stop)"
echo ""

# Publish messages in a loop
counter=1
while true; do
  timestamp=$(date +"%H:%M:%S")
  
  # Publish to various channels
  redis_cmd PUBLISH events:user "{\"action\":\"login\",\"user\":\"user_${counter}\",\"time\":\"${timestamp}\"}" > /dev/null
  redis_cmd PUBLISH events:system "{\"type\":\"heartbeat\",\"count\":${counter}}" > /dev/null
  redis_cmd PUBLISH notifications "New notification #${counter}" > /dev/null
  redis_cmd PUBLISH app:orders "Order #${counter} created" > /dev/null
  redis_cmd PUBLISH logs:debug "Debug message ${counter}" > /dev/null
  
  echo -e "  [${timestamp}] Published batch #${counter} to 5 channels"
  
  counter=$((counter + 1))
  sleep 2
done
