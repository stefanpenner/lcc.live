#!/usr/bin/env bash
set -euo pipefail

# Deploy lcc.live to Fly.io or local Docker using Bazel-built OCI images

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail; set +e; f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null || \
  source "$0.runfiles/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  { echo>&2 "ERROR: cannot find $f"; exit 1; }; f=; set -e
# --- end runfiles.bash initialization v3 ---

# First argument is the image_load executable from Bazel (via $(location))
IMAGE_LOAD="${1:?Missing image_load location}"
# Second argument is the deployment target (fly or local)
TARGET="${2:-fly}"

echo "📦 Loading image into Docker..."
"$IMAGE_LOAD"

if [ "$TARGET" = "local" ]; then
    echo "✅ Image loaded into Docker!"
    
    # Stop and remove existing containers
    echo "🛑 Stopping existing containers..."
    
    # Remove named container if it exists
    if docker ps -a --format '{{.Names}}' | grep -q '^lcc-live$'; then
        docker stop lcc-live 2>/dev/null || true
        docker rm lcc-live 2>/dev/null || true
        echo "   Removed existing 'lcc-live' container"
    fi
    
    # Remove any other containers using this image
    EXISTING_CONTAINERS=$(docker ps -a -q --filter ancestor=lcc.live:latest)
    if [ -n "$EXISTING_CONTAINERS" ]; then
        docker stop $EXISTING_CONTAINERS 2>/dev/null || true
        docker rm $EXISTING_CONTAINERS 2>/dev/null || true
        echo "   Removed old containers"
    fi
    
    # Run the new container
    echo "🐳 Starting container..."
    CONTAINER_ID=$(docker run -d -p 3000:3000 --name lcc-live lcc.live:latest)
    echo "   Container started: ${CONTAINER_ID:0:12}"
    
    # Wait a moment for the container to start
    sleep 2
    
    # Check if it's running
    if docker ps | grep -q lcc-live; then
        echo ""
        echo "✅ Deployment complete!"
        echo "🔍 Health check: http://localhost:3000/healthcheck"
        echo "🔍 Version: http://localhost:3000/_/version"
        echo ""
        echo "📊 View logs: docker logs -f lcc-live"
        echo "🛑 Stop: docker stop lcc-live"
    else
        echo ""
        echo "❌ Container failed to start. Check logs:"
        docker logs lcc-live
        exit 1
    fi
else
    echo "🚀 Deploying to Fly.io..."
    fly deploy --local-only --image lcc.live:latest
    
    echo "✅ Deployment complete!"
    echo "🔍 Check version at: https://lcc.live/_/version"
fi

