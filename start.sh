#!/usr/bin/env bash
# ==============================================================================
# High-Scale Thai-English Entity Resolution & Data Matching Engine
# Start Script
# Usage:
#   ./start.sh          # Start via Docker Compose (default, recommended)
#   ./start.sh docker   # Start via Docker Compose
#   ./start.sh local    # Start locally (Go backend on :8085 + Vite frontend on :3000)
# ==============================================================================

set -e

MODE="${1:-docker}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_DIR="${SCRIPT_DIR}/.run"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Determine docker compose command
get_docker_compose() {
  if command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
  elif docker compose version >/dev/null 2>&1; then
    echo "docker compose"
  else
    echo ""
  fi
}

start_docker() {
  DOCKER_COMPOSE_CMD=$(get_docker_compose)
  if [ -z "$DOCKER_COMPOSE_CMD" ]; then
    echo -e "${RED}Error: neither 'docker compose' nor 'docker-compose' found.${NC}"
    echo -e "${YELLOW}Please install Docker Compose or run in local mode: ./start.sh local${NC}"
    exit 1
  fi

  echo -e "${BLUE}==>${NC} Starting Entity Matcher stack with Docker Compose..."
  cd "${SCRIPT_DIR}"
  $DOCKER_COMPOSE_CMD up -d --build

  echo -e "${BLUE}==>${NC} Waiting for services to become healthy..."
  local max_attempts=30
  local attempt=1
  local backend_ready=0

  while [ $attempt -le $max_attempts ]; do
    if curl -s -f http://localhost:8085/api/health >/dev/null 2>&1; then
      backend_ready=1
      break
    fi
    sleep 1
    attempt=$((attempt + 1))
  done

  echo ""
  if [ $backend_ready -eq 1 ]; then
    echo -e "${GREEN}✓ System started successfully!${NC}"
  else
    echo -e "${YELLOW}! System started (backend health check took longer than expected).${NC}"
  fi

  echo -e "--------------------------------------------------------"
  echo -e " Frontend UI : ${GREEN}http://localhost:3000${NC}"
  echo -e " Backend API : ${GREEN}http://localhost:8085${NC} (Health: http://localhost:8085/api/health)"
  echo -e " Postgres DB : ${GREEN}localhost:5432${NC}"
  echo -e "--------------------------------------------------------"
  echo -e "To view logs: ${YELLOW}$DOCKER_COMPOSE_CMD logs -f${NC}"
  echo -e "To stop:      ${YELLOW}./stop.sh${NC}"
}

start_local() {
  echo -e "${BLUE}==>${NC} Starting Entity Matcher in Local Development Mode..."

  # Check Go
  if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}Error: 'go' is not installed or not in PATH.${NC}"
    exit 1
  fi

  # Check Node / npm
  if ! command -v npm >/dev/null 2>&1; then
    echo -e "${RED}Error: 'npm' is not installed or not in PATH.${NC}"
    exit 1
  fi

  mkdir -p "${PID_DIR}"

  # Check if already running
  if [ -f "${PID_DIR}/backend.pid" ] && kill -0 "$(cat "${PID_DIR}/backend.pid")" 2>/dev/null; then
    echo -e "${YELLOW}Backend is already running with PID $(cat "${PID_DIR}/backend.pid")${NC}"
  else
    echo -e "${BLUE}==>${NC} Starting Go backend on port 8085..."
    cd "${SCRIPT_DIR}/backend"
    export PORT=8085
    export JWT_SECRET="${JWT_SECRET:-dev-secret-change-me-in-production}"
    # If no DATABASE_URL is provided, backend automatically falls back to high-performance in-memory store
    nohup go run . > "${PID_DIR}/backend.log" 2>&1 &
    echo $! > "${PID_DIR}/backend.pid"
    echo -e "${GREEN}✓ Backend process started (PID: $(cat "${PID_DIR}/backend.pid"))${NC}"
  fi

  # Install frontend deps if needed and start Vite
  cd "${SCRIPT_DIR}/frontend"
  if [ ! -d "node_modules" ]; then
    echo -e "${BLUE}==>${NC} Installing frontend dependencies..."
    npm install
  fi

  if [ -f "${PID_DIR}/frontend.pid" ] && kill -0 "$(cat "${PID_DIR}/frontend.pid")" 2>/dev/null; then
    echo -e "${YELLOW}Frontend is already running with PID $(cat "${PID_DIR}/frontend.pid")${NC}"
  else
    echo -e "${BLUE}==>${NC} Starting Vite frontend on port 3000..."
    nohup npm run dev > "${PID_DIR}/frontend.log" 2>&1 &
    echo $! > "${PID_DIR}/frontend.pid"
    echo -e "${GREEN}✓ Frontend process started (PID: $(cat "${PID_DIR}/frontend.pid"))${NC}"
  fi

  echo -e "${BLUE}==>${NC} Waiting for services to be ready..."
  sleep 2

  echo ""
  echo -e "${GREEN}✓ Local environment running!${NC}"
  echo -e "--------------------------------------------------------"
  echo -e " Frontend UI : ${GREEN}http://localhost:3000${NC}"
  echo -e " Backend API : ${GREEN}http://localhost:8085${NC} (Health: http://localhost:8085/api/health)"
  echo -e " Backend Log : ${YELLOW}${PID_DIR}/backend.log${NC}"
  echo -e " Frontend Log: ${YELLOW}${PID_DIR}/frontend.log${NC}"
  echo -e "--------------------------------------------------------"
  echo -e "To stop: ${YELLOW}./stop.sh local${NC}"
}

case "$MODE" in
  docker)
    start_docker
    ;;
  local)
    start_local
    ;;
  help|-h|--help)
    echo "Usage: $0 [docker|local]"
    echo "  docker : Start using Docker Compose (default)"
    echo "  local  : Start using local Go and Node.js runtimes"
    exit 0
    ;;
  *)
    echo -e "${RED}Unknown mode: $MODE${NC}"
    echo "Usage: $0 [docker|local]"
    exit 1
    ;;
esac
