#!/usr/bin/env bash
set -euo pipefail

# Deploy lcc.live to Fly.io or local Docker using Bazel-built OCI images

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail
set +e
f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null ||
  source "$0.runfiles/$f" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -d' ' -f2- -d' ')" 2>/dev/null ||
  {
    echo >&2 "ERROR: cannot find $f"
    exit 1
  }
f=
set -e
# --- end runfiles.bash initialization v3 ---

# Configuration
IMAGE_NAME="lcc.live:latest"
CONTAINER_NAME="lcc-live"
MAX_IMAGE_LOAD_WAIT=120  # Maximum seconds to wait for image loading
MAX_CONTAINER_START_WAIT=30  # Maximum seconds to wait for container to be ready
HEALTH_CHECK_RETRIES=10  # Number of health check retries

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✅${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}❌${NC} $1"
}

# Check if Docker is available and running
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running. Please start Docker Desktop."
        exit 1
    fi
    
    log_success "Docker is available and running"
}

# Detect platform and show warning if needed
detect_platform() {
    local platform=$(uname -m)
    local docker_platform=$(docker version --format '{{.Server.Arch}}' 2>/dev/null || echo "unknown")
    
    if [[ "$platform" == "arm64" && "$docker_platform" == "x86_64" ]]; then
        log_warning "Running on Apple Silicon (ARM64) with x86_64 Docker - emulation may be slower"
        log_info "Consider building ARM64 images for better performance"
    fi
}

# Comprehensive cleanup of existing containers and images
cleanup_existing() {
    log_info "Cleaning up existing containers and images..."
    
    # Stop and remove named container
    if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        log_info "Stopping existing '${CONTAINER_NAME}' container..."
        docker stop "${CONTAINER_NAME}" 2>/dev/null || true
        docker rm "${CONTAINER_NAME}" 2>/dev/null || true
        log_success "Removed existing '${CONTAINER_NAME}' container"
    fi
    
    # Remove any containers using our image
    local existing_containers=$(docker ps -a -q --filter ancestor="${IMAGE_NAME}" 2>/dev/null || true)
    if [ -n "$existing_containers" ]; then
        log_info "Removing containers using ${IMAGE_NAME}..."
        echo "$existing_containers" | xargs docker stop 2>/dev/null || true
        echo "$existing_containers" | xargs docker rm 2>/dev/null || true
        log_success "Removed old containers"
    fi
    
    # Clean up dangling images (optional - be careful with this)
    local dangling_images=$(docker images -f "dangling=true" -q 2>/dev/null || true)
    if [ -n "$dangling_images" ]; then
        log_info "Cleaning up dangling images..."
        echo "$dangling_images" | xargs docker rmi 2>/dev/null || true
    fi
}

# Load image with timeout and better error handling
load_image() {
    log_info "Loading image into Docker..."
    
    # Always remove old image to ensure we get the latest from Bazel
    # This prevents stale images when Bazel cache says "up-to-date" but Docker has old content
    if docker image inspect "${IMAGE_NAME}" &> /dev/null; then
        log_info "Removing existing image ${IMAGE_NAME} to ensure fresh load..."
        docker rmi "${IMAGE_NAME}" 2>/dev/null || true
    fi
    
    # Load image with timeout
    log_info "Loading image (this may take a moment)..."
    if timeout "${MAX_IMAGE_LOAD_WAIT}" "$IMAGE_LOAD" 2>&1 | tee /tmp/image_load.log; then
        log_success "Image loaded successfully"
    else
        local exit_code=$?
        if [ $exit_code -eq 124 ]; then
            log_warning "Image loading timed out after ${MAX_IMAGE_LOAD_WAIT}s"
            log_info "This is a known issue with oci_load on some systems"
            log_info "Checking if image already exists and is usable..."
            
            # Check if we can use the existing image
            if docker image inspect "${IMAGE_NAME}" &> /dev/null; then
                log_success "Using existing image ${IMAGE_NAME}"
                return 0
            else
                log_error "No usable image found. Try these solutions:"
                log_info "1. Run: docker system prune -f && ./b deploy:local"
                log_info "2. Restart Docker Desktop"
                log_info "3. Use: ./b deploy:clean (includes cleanup)"
                exit 1
            fi
        else
            log_error "Image loading failed with exit code $exit_code"
            log_info "Image load output:"
            cat /tmp/image_load.log 2>/dev/null || true
            exit 1
        fi
    fi
    
    # Verify image was loaded
    if ! docker image inspect "${IMAGE_NAME}" &> /dev/null; then
        log_error "Image ${IMAGE_NAME} not found after loading"
        exit 1
    fi
}

# Wait for container to be healthy
wait_for_health() {
    local container_id="$1"
    local port="$2"
    local retries=0
    
    log_info "Waiting for container to be healthy..."
    
    while [ $retries -lt $HEALTH_CHECK_RETRIES ]; do
        if curl -s -f "http://localhost:${port}/healthcheck" >/dev/null 2>&1; then
            log_success "Container is healthy"
            return 0
        fi
        
        retries=$((retries + 1))
        log_info "Health check attempt ${retries}/${HEALTH_CHECK_RETRIES}..."
        sleep 2
    done
    
    log_error "Container failed health checks after ${HEALTH_CHECK_RETRIES} attempts"
    log_info "Container logs:"
    docker logs "${container_id}" 2>&1 | tail -20
    return 1
}

# First argument is the image_load executable from Bazel (via $(location))
IMAGE_LOAD="${1:?Missing image_load location}"
# Second argument is the deployment target (fly or local)
TARGET="${2:-fly}"

# Pre-deployment checks and cleanup
check_docker
detect_platform
cleanup_existing
load_image

if [ "$TARGET" = "local" ]; then
  log_success "Image loaded into Docker!"

  # Run the new container with dynamic port allocation
  log_info "Starting container with dynamic port allocation..."
  CONTAINER_ID=$(docker run -d -p 0:3000 --name "${CONTAINER_NAME}" "${IMAGE_NAME}")
  log_success "Container started: ${CONTAINER_ID:0:12}"

  # Wait for container to be ready
  sleep 2

  # Check if it's running and get the assigned port
  if docker ps | grep -q "${CONTAINER_NAME}"; then
    # Get the dynamically assigned port
    HOST_PORT=$(docker port "${CONTAINER_NAME}" 3000 | cut -d: -f2)
    
    # Wait for health check
    if wait_for_health "${CONTAINER_ID}" "${HOST_PORT}"; then
      echo ""
      log_success "Deployment complete!"
      echo "🌐 Server running on: http://localhost:${HOST_PORT}"
      echo "🔍 Health check: http://localhost:${HOST_PORT}/healthcheck"
      echo "🔍 Version: http://localhost:${HOST_PORT}/_/version"
      echo ""
      echo "📊 View logs: docker logs -f ${CONTAINER_NAME}"
      echo "🛑 Stop: docker stop ${CONTAINER_NAME}"
    else
      log_error "Container failed health checks"
      exit 1
    fi
  else
    log_error "Container failed to start. Check logs:"
    docker logs "${CONTAINER_NAME}"
    exit 1
  fi
else
  log_info "Deploying to Fly.io..."
  fly deploy --local-only --image "${IMAGE_NAME}"

  if [ $? -eq 0 ]; then
    log_success "Deployment complete!"
    echo "🔍 Check version at: https://lcc.live/_/version"
    
    # Purge Cloudflare cache after successful deployment
    log_info "Purging Cloudflare cache..."
    fly ssh console -C "/usr/local/bin/lcc-live purge-cache"
    
    log_success "Deployment and cache purge complete!"
  else
    log_error "Deployment failed"
    exit 1
  fi
fi