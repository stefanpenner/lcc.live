#!/usr/bin/env bash
# Container integration test - verifies the built OCI image works correctly

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}✓${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Configuration
IMAGE_NAME="lcc.live:test"
CONTAINER_NAME="lcc-live-test-$$"
# Use a random port to avoid conflicts (especially in parallel test runs)
PORT=$((3001 + RANDOM % 1000))
MAX_WAIT=30  # Maximum seconds to wait for container to be ready

cleanup() {
    if [ -n "${CONTAINER_ID:-}" ]; then
        log_info "Cleaning up container..."
        docker stop "$CONTAINER_ID" >/dev/null 2>&1 || true
        docker rm "$CONTAINER_ID" >/dev/null 2>&1 || true
    fi
}

# Ensure cleanup happens on exit
trap cleanup EXIT

log_info "Starting container integration test..."

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not available"
    exit 1
fi

# Check if the image exists
if ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
    log_error "Image $IMAGE_NAME not found. Build it first with: bazel run //:image_load"
    exit 1
fi

log_info "Image $IMAGE_NAME found"

# Clean up any existing test containers with the same name
docker stop "$CONTAINER_NAME" 2>/dev/null || true
docker rm "$CONTAINER_NAME" 2>/dev/null || true

# Clean up any orphaned test containers (best effort)
docker ps -a --filter "name=lcc-live-test" --filter "status=created" -q 2>/dev/null | xargs docker rm 2>/dev/null || true

# Start the container
log_info "Starting container $CONTAINER_NAME on port $PORT..."
# Capture stderr separately and only keep stdout (container ID)
CONTAINER_ID=$(docker run -d \
    --name "$CONTAINER_NAME" \
    -p "$PORT:3000" \
    -e PORT=3000 \
    "$IMAGE_NAME" 2>/dev/null)

# Check if docker run succeeded
DOCKER_EXIT=$?
if [ $DOCKER_EXIT -ne 0 ]; then
    # Re-run with stderr to show the error
    ERROR_OUTPUT=$(docker run -d --name "${CONTAINER_NAME}-debug" -p "$PORT:3000" -e PORT=3000 "$IMAGE_NAME" 2>&1 || true)
    docker rm "${CONTAINER_NAME}-debug" 2>/dev/null || true
    log_error "Failed to start container (exit code: $DOCKER_EXIT): $ERROR_OUTPUT"
    exit 1
fi

# Validate container ID
if [ -z "$CONTAINER_ID" ]; then
    log_error "Failed to get container ID"
    exit 1
fi

# Strip any whitespace/newlines
CONTAINER_ID=$(echo "$CONTAINER_ID" | tr -d '[:space:]')

if [ "${#CONTAINER_ID}" -lt 12 ]; then
    log_error "Invalid container ID: '$CONTAINER_ID'"
    exit 1
fi

log_info "Container started: ${CONTAINER_ID:0:12}"

# Wait for container to be ready
log_info "Waiting for container to be ready (max ${MAX_WAIT}s)..."
START_TIME=$(date +%s)
while true; do
    ELAPSED=$(( $(date +%s) - START_TIME ))
    
    # Check if we've exceeded max wait time
    if [ $ELAPSED -ge $MAX_WAIT ]; then
        log_error "Container did not become ready within ${MAX_WAIT}s"
        log_info "Container logs:"
        docker logs "$CONTAINER_ID" 2>&1 | tail -30
        exit 1
    fi
    
    # Check if container is still running
    if ! docker ps --format '{{.ID}}' | grep -q "^${CONTAINER_ID:0:12}"; then
        log_error "Container stopped unexpectedly after ${ELAPSED}s"
        log_info "Container logs:"
        docker logs "$CONTAINER_ID" 2>&1 | tail -50
        exit 1
    fi
    
    # Try healthcheck
    if curl -s -f "http://localhost:$PORT/healthcheck" >/dev/null 2>&1; then
        log_info "Container is ready after ${ELAPSED}s"
        break
    fi
    
    sleep 1
done

# Run tests
FAILED=0

# Test 1: Healthcheck endpoint
log_info "Testing healthcheck endpoint..."
RESPONSE=$(curl -s "http://localhost:$PORT/healthcheck")
if [ "$RESPONSE" = "OK" ]; then
    log_info "Healthcheck: PASSED"
else
    log_error "Healthcheck: FAILED (got: '$RESPONSE', expected: 'OK')"
    FAILED=1
fi

# Test 2: Healthcheck returns 200
log_info "Testing healthcheck status code..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$PORT/healthcheck")
if [ "$STATUS" = "200" ]; then
    log_info "Healthcheck status: PASSED (200)"
else
    log_error "Healthcheck status: FAILED (got: $STATUS, expected: 200)"
    FAILED=1
fi

# Test 3: Version endpoint returns JSON
log_info "Testing version endpoint..."
VERSION_JSON=$(curl -s "http://localhost:$PORT/_/version")
if echo "$VERSION_JSON" | grep -q '"version"'; then
    log_info "Version endpoint: PASSED"
else
    log_error "Version endpoint: FAILED (invalid JSON: $VERSION_JSON)"
    FAILED=1
fi

# Test 4: X-Version header is present
log_info "Testing X-Version header..."
VERSION_HEADER=$(curl -s -I "http://localhost:$PORT/healthcheck" | grep -i "X-Version:" || true)
if [ -n "$VERSION_HEADER" ]; then
    log_info "X-Version header: PASSED ($VERSION_HEADER)"
else
    log_error "X-Version header: FAILED (not found)"
    FAILED=1
fi

# Test 5: Root endpoint (/) returns HTML
log_info "Testing root endpoint..."
ROOT_RESPONSE=$(curl -s "http://localhost:$PORT/")
if echo "$ROOT_RESPONSE" | grep -q "Cottonwood Canyon"; then
    log_info "Root endpoint: PASSED (HTML content found)"
else
    log_error "Root endpoint: FAILED (no expected content)"
    FAILED=1
fi

# Test 6: Container is running as non-root user
log_info "Testing container security (non-root user)..."
USER_ID=$(docker exec "$CONTAINER_ID" id -u 2>/dev/null || echo "error")
if [ "$USER_ID" != "0" ]; then
    log_info "Security: PASSED (running as UID $USER_ID, not root)"
else
    log_warn "Security: WARNING (running as root)"
    # Don't fail on this, just warn
fi

# Test 7: Metrics endpoint is accessible
log_info "Testing metrics endpoint..."
METRICS_RESPONSE=$(curl -s "http://localhost:$PORT/_/metrics")
if echo "$METRICS_RESPONSE" | grep -q "go_info"; then
    log_info "Metrics endpoint: PASSED"
else
    log_error "Metrics endpoint: FAILED"
    FAILED=1
fi

# Summary
echo ""
if [ $FAILED -eq 0 ]; then
    log_info "All container tests PASSED ✨"
    exit 0
else
    log_error "Some container tests FAILED"
    echo ""
    log_info "Container logs:"
    docker logs "$CONTAINER_ID" 2>&1 | tail -50
    exit 1
fi
