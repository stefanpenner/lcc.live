#!/usr/bin/env bash
# Pre-deployment cleanup script for lcc.live
# This script ensures a clean state before deployment

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Configuration
IMAGE_NAME="lcc.live:latest"
CONTAINER_NAME="lcc-live"

log_info "Starting cleanup process..."

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running. Please start Docker Desktop."
    exit 1
fi

log_success "Docker is available and running"

# Stop and remove all containers using our image
log_info "Stopping and removing containers using ${IMAGE_NAME}..."
EXISTING_CONTAINERS=$(docker ps -a -q --filter ancestor="${IMAGE_NAME}" 2>/dev/null || true)
if [ -n "$EXISTING_CONTAINERS" ]; then
    echo "$EXISTING_CONTAINERS" | xargs docker stop 2>/dev/null || true
    echo "$EXISTING_CONTAINERS" | xargs docker rm 2>/dev/null || true
    log_success "Removed containers using ${IMAGE_NAME}"
else
    log_info "No containers found using ${IMAGE_NAME}"
fi

# Stop and remove named container specifically
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_info "Stopping and removing '${CONTAINER_NAME}' container..."
    docker stop "${CONTAINER_NAME}" 2>/dev/null || true
    docker rm "${CONTAINER_NAME}" 2>/dev/null || true
    log_success "Removed '${CONTAINER_NAME}' container"
else
    log_info "No '${CONTAINER_NAME}' container found"
fi

# Clean up dangling images
log_info "Cleaning up dangling images..."
DANGLING_IMAGES=$(docker images -f "dangling=true" -q 2>/dev/null || true)
if [ -n "$DANGLING_IMAGES" ]; then
    echo "$DANGLING_IMAGES" | xargs docker rmi 2>/dev/null || true
    log_success "Removed dangling images"
else
    log_info "No dangling images found"
fi

# Optional: Clean up unused images (be careful with this)
if [ "${1:-}" = "--aggressive" ]; then
    log_warning "Running aggressive cleanup (removing unused images)..."
    docker image prune -f 2>/dev/null || true
    log_success "Removed unused images"
fi

# Optional: Clean up build cache
if [ "${1:-}" = "--aggressive" ]; then
    log_warning "Cleaning Docker build cache..."
    docker builder prune -f 2>/dev/null || true
    log_success "Cleaned build cache"
fi

log_success "Cleanup complete! Ready for deployment."
