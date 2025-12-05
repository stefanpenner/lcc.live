#!/usr/bin/env bash
# Bazel helper script for common operations

set -euo pipefail

COMMAND="${1:-help}"

case "$COMMAND" in
build)
  echo "🔨 Building binary..."
  bazel build //:lcc-live
  echo "✅ Build complete: bazel-bin/lcc-live_/lcc-live"
  ;;

test)
  echo "🔄 Updating BUILD files with Gazelle..."
  bazel run //:gazelle
  echo "✅ BUILD files updated"
  echo "🧪 Running tests..."
  bazel test //...
  ;;

run)
  export DEV_MODE=1
  echo "🚀 Running server in dev mode (hot reload enabled)..."
  bazel run //:lcc-live
  ;;


clean)
  echo "🧹 Cleaning build artifacts..."
  bazel clean
  ;;

gazelle)
  echo "🔄 Regenerating BUILD files..."
  bazel run //:gazelle
  echo "✅ BUILD files updated"
  ;;

deps)
  echo "📦 Updating dependencies..."
  go mod tidy
  bazel mod deps
  echo "✅ Dependencies updated"
  ;;

opt)
  echo "🚀 Building optimized binary..."
  bazel build --config=opt //:lcc-live
  echo "✅ Optimized build complete: bazel-bin/lcc-live_/lcc-live"
  ;;

deploy)
  echo "🔄 Updating BUILD files with Gazelle..."
  bazel run //:gazelle
  echo "✅ BUILD files updated"
  echo "🧪 Running tests..."
  bazel test //...
  echo "🚀 Deploying..."
  bazel run --config=opt //scripts:deploy
  ;;

deploy:local)
  echo "🚀 Deploying... locally"
  bazel run --config=opt //scripts:deploy -- local
  ;;

deploy:clean)
  echo "🧹 Cleaning up before deployment..."
  ./scripts/cleanup.sh
  echo "🚀 Deploying... locally"
  bazel run --config=opt //scripts:deploy -- local
  ;;

cleanup)
  echo "🧹 Cleaning up Docker containers and images..."
  ./scripts/cleanup.sh
  ;;

cleanup:aggressive)
  echo "🧹 Aggressive cleanup of Docker resources..."
  ./scripts/cleanup.sh --aggressive
  ;;

logs)
  echo "📋 Viewing Fly.io logs..."
  fly logs
  ;;

metrics)
  echo "📊 Opening metrics endpoint..."
  open "https://lcc.live/_/metrics" 2>/dev/null ||
    xdg-open "https://lcc.live/_/metrics" 2>/dev/null ||
    echo "Visit: https://lcc.live/_/metrics"
  ;;

graphana)
  echo "📊 Opening Graphana dashboard..."
  open https://fly-metrics.net/d/fly-app/fly-app?orgId=115526
  ;;

console)
  echo "🖥️  Opening Fly.io console..."
  fly console
  ;;

purge-cache)
  echo "🗑️  Purging Cloudflare cache..."
  fly ssh console -C "/usr/local/bin/lcc-live purge-cache"
  ;;

dashboard)
  echo "📊 Opening Fly.io dashboard..."
  fly dashboard
  ;;

help | *)
  cat <<EOF
Bazel helper script for lcc.live

Usage: ./b <command>

Commands:
  build        - Build the binary
  test         - Run all tests
  run          - Run server in dev mode (hot reload enabled)
  clean        - Clean build artifacts
  gazelle      - Regenerate BUILD files
  deps         - Update dependencies from go.mod
  opt          - Build optimized binary for production
  deploy       - Deploy to Fly.io
  deploy:local - Build, load, and run image in local Docker
  deploy:clean - Clean up Docker resources before deploying locally
  cleanup      - Clean up Docker containers and images
  cleanup:aggressive - Aggressive cleanup of Docker resources
  logs         - View Fly.io logs
  metrics      - Open metrics endpoint
  dashboard    - Open Fly.io dashboard
  console      - Open Fly.io console
  purge-cache  - Test Cloudflare cache purge
  help         - Show this help message

Examples:
  ./b build        # Build the binary
  ./b test         # Run all tests
  ./b run          # Run server in dev mode (hot reload)
  ./b deploy:local # Deploy to local Docker
  ./b deploy       # Deploy to Fly.io
  ./b logs         # View Fly.io logs

For more details, see doc/BAZEL.md
EOF
  ;;
esac