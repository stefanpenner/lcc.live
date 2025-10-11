#!/usr/bin/env bash
# Container integration test - verifies the built OCI image works correctly

set -euo pipefail

# Colors
RED='\033[0;31m' GREEN='\033[0;32m' YELLOW='\033[1;33m' NC='\033[0m'
log_info() { echo -e "${GREEN}✓${NC} $1"; }
log_error() { echo -e "${RED}✗${NC} $1"; }
log_warn() { echo -e "${YELLOW}⚠${NC} $1"; }

# Config
IMAGE_NAME="lcc.live:latest"
CONTAINER_NAME="lcc-live-test-$$"
PORT=$((3001 + RANDOM % 1000))
MAX_WAIT=30

# Cleanup
cleanup() { 
    [ -n "${CONTAINER_ID:-}" ] && docker stop "$CONTAINER_ID" >/dev/null 2>&1 || true
    [ -n "${CONTAINER_ID:-}" ] && docker rm "$CONTAINER_ID" >/dev/null 2>&1 || true
}
trap cleanup EXIT

log_info "Starting container integration test..."

# Ensure Docker is available and running
if ! command -v docker &>/dev/null; then
    log_error "Docker is not installed. Install from: https://www.docker.com/products/docker-desktop"
    exit 1
fi

if ! docker info >/dev/null 2>&1; then
    log_warn "Docker daemon is not running, attempting to start..."
    if [[ "$OSTYPE" == "darwin"* ]] && [ -e "/Applications/Docker.app" ]; then
        open -a Docker
        for i in {1..30}; do
            docker info >/dev/null 2>&1 && { log_info "Docker started"; break; }
            sleep 2
        done
    elif [[ "$OSTYPE" == "linux-gnu"* ]] && command -v systemctl &>/dev/null; then
        sudo systemctl start docker 2>/dev/null && sleep 2
    fi
    docker info >/dev/null 2>&1 || { log_error "Failed to start Docker"; exit 1; }
fi

# Load image if needed
if ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
    log_info "Loading image $IMAGE_NAME..."
    LOADER="${RUNFILES_DIR:-}/_main/image_load.sh"
    [ ! -f "$LOADER" ] && [ -n "${RUNFILES_MANIFEST_FILE:-}" ] && \
        LOADER=$(grep "_main/image_load.sh$" "$RUNFILES_MANIFEST_FILE" | cut -d' ' -f2)
    [ ! -f "$LOADER" ] && LOADER="$(dirname "$0")/image_load.sh"
    
    if [ -x "$LOADER" ]; then
        "$LOADER" || { log_error "Failed to load image"; exit 1; }
    else
        log_error "Image not found. Build with: bazel run //:image_load"
        exit 1
    fi
fi

# Start container
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
log_info "Starting container on port $PORT..."
CONTAINER_ID=$(docker run -d --name "$CONTAINER_NAME" -p "$PORT:3000" -e PORT=3000 "$IMAGE_NAME")
[ -z "$CONTAINER_ID" ] && { log_error "Failed to start container"; exit 1; }
log_info "Container started: ${CONTAINER_ID:0:12}"

# Wait for ready
log_info "Waiting for container (max ${MAX_WAIT}s)..."
for i in $(seq 1 $MAX_WAIT); do
    if docker ps --format '{{.ID}}' | grep -q "^${CONTAINER_ID:0:12}"; then
        curl -sf "http://localhost:$PORT/healthcheck" >/dev/null 2>&1 && { 
            log_info "Ready after ${i}s"; 
            break; 
        }
    else
        log_error "Container stopped"
        docker logs "$CONTAINER_ID" 2>&1 | tail -30
        exit 1
    fi
    [ $i -eq $MAX_WAIT ] && { log_error "Timeout"; exit 1; }
    sleep 1
done

# Run tests
run_test() {
    local name=$1 cmd=$2 expected=$3
    result=$(eval "$cmd" 2>/dev/null || echo "")
    if [[ "$result" == *"$expected"* ]]; then
        log_info "$name: PASSED"
        return 0
    else
        log_error "$name: FAILED"
        return 1
    fi
}

FAILED=0
run_test "Healthcheck" "curl -s http://localhost:$PORT/healthcheck" "OK" || FAILED=1
run_test "Status code" "curl -sw '%{http_code}' -o /dev/null http://localhost:$PORT/healthcheck" "200" || FAILED=1
run_test "Version JSON" "curl -s http://localhost:$PORT/_/version" '"version"' || FAILED=1
run_test "X-Version header" "curl -sI http://localhost:$PORT/healthcheck" "X-Version:" || FAILED=1
run_test "Root HTML" "curl -s http://localhost:$PORT/" "Cottonwood Canyon" || FAILED=1
run_test "Metrics" "curl -s http://localhost:$PORT/_/metrics" "go_info" || FAILED=1

# Security check (warning only)
USER_ID=$(docker exec "$CONTAINER_ID" id -u 2>/dev/null || echo "0")
[ "$USER_ID" != "0" ] && log_info "Security: non-root (UID $USER_ID)" || log_warn "Security: running as root"

# Summary
echo ""
if [ $FAILED -eq 0 ]; then
    log_info "All tests PASSED ✨"
    exit 0
else
    log_error "Tests FAILED"
    docker logs "$CONTAINER_ID" 2>&1 | tail -30
    exit 1
fi
