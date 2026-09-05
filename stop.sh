#!/usr/bin/env bash
# ==============================================================================
# High-Scale Thai-English Entity Resolution & Data Matching Engine
# Stop Script
# Usage:
#   ./stop.sh           # Stop both Docker containers and local background processes
#   ./stop.sh docker    # Stop Docker Compose services
#   ./stop.sh local     # Stop local background processes
# ==============================================================================

MODE="${1:-all}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_DIR="${SCRIPT_DIR}/.run"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

collect_descendants() {
  local parent="$1"
  local child
  for child in $(pgrep -P "$parent" 2>/dev/null); do
    collect_descendants "$child"
    echo "$child"
  done
}

get_docker_compose() {
  if command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
  elif docker compose version >/dev/null 2>&1; then
    echo "docker compose"
  else
    echo ""
  fi
}

stop_docker() {
  DOCKER_COMPOSE_CMD=$(get_docker_compose)
  if [ -n "$DOCKER_COMPOSE_CMD" ]; then
    echo -e "${BLUE}==>${NC} Stopping Docker Compose services..."
    cd "${SCRIPT_DIR}"
    $DOCKER_COMPOSE_CMD down
    echo -e "${GREEN}✓ Docker services stopped.${NC}"
  else
    echo -e "${YELLOW}Docker compose command not found, skipping Docker shutdown.${NC}"
  fi
}

stop_local_process() {
  local name="$1"
  local pid_file="${PID_DIR}/${name}.pid"

  if [ -f "$pid_file" ]; then
    local pid
    pid=$(cat "$pid_file")
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      echo -e "${BLUE}==>${NC} Stopping ${name} (PID: ${pid})..."

      # Collect descendants only if pgrep is available
      local descendants=()
      if command -v pgrep >/dev/null 2>&1; then
        mapfile -t descendants < <(collect_descendants "$pid")
      fi

      # Send TERM signal to all descendants first
      local descendant
      for descendant in "${descendants[@]}"; do
        kill "$descendant" 2>/dev/null || true
      done

      # Send TERM signal to parent
      kill "$pid" 2>/dev/null || true

      sleep 1

      # Check and send KILL to descendants if still alive
      for descendant in "${descendants[@]}"; do
        if kill -0 "$descendant" 2>/dev/null; then
          kill -9 "$descendant" 2>/dev/null || true
        fi
      done

      # Check and send KILL to parent if still alive
      if kill -0 "$pid" 2>/dev/null; then
        kill -9 "$pid" 2>/dev/null || true
      fi

      echo -e "${GREEN}✓ ${name} stopped.${NC}"
    else
      echo -e "${YELLOW}${name} (PID: ${pid}) is not running.${NC}"
    fi
    rm -f "$pid_file"
  fi
}

stop_local() {
  echo -e "${BLUE}==>${NC} Stopping local processes..."
  stop_local_process "backend"
  stop_local_process "frontend"

  echo -e "${GREEN}✓ Local processes stopped.${NC}"
}

case "$MODE" in
  docker)
    stop_docker
    ;;
  local)
    stop_local
    ;;
  all)
    stop_docker
    stop_local
    ;;
  help|-h|--help)
    echo "Usage: $0 [docker|local|all]"
    echo "  all    : Stop both Docker services and local processes (default)"
    echo "  docker : Stop Docker Compose services"
    echo "  local  : Stop local background processes"
    exit 0
    ;;
  *)
    echo -e "${RED}Unknown mode: $MODE${NC}"
    echo "Usage: $0 [docker|local|all]"
    exit 1
    ;;
esac
