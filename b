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
  echo "🧪 Running tests..."
  bazel test //...
  ;;

run)
  echo "🚀 Running server..."
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
  echo "🧪 Running tests..."
  bazel test //...
  echo "🚀 Deploying..."
  bazel run --config=opt //:deploy
  ;;

deploy:local)
  echo "🚀 Deploying... locally"
  bazel run --config=opt //:deploy -- local
  ;;

deploy:clean)
  echo "🧹 Cleaning up before deployment..."
  ./scripts/cleanup.sh
  echo "🚀 Deploying... locally"
  bazel run --config=opt //:deploy -- local
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
  echo "📊 Opening Graphana dashboard..."
  fly console
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
  run          - Run the server
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
  help         - Show this help message

Examples:
  ./b build
  ./b test
  ./b run
  ./b dev             # Development server with live reload
  ./b dev:stop        # Stop development server
  ./b deploy:local    # Deploy to local Docker
  ./b deploy:clean    # Clean up and deploy to local Docker
  ./b cleanup         # Clean up Docker resources
  ./b deploy          # Deploy to Fly.io
  ./b logs            # View Fly.io logs
  ./b dashboard       # Open Fly.io dashboard
  ./b Graphana        # Open Graphana
  ./b console         # Open Grap

For more details, see doc/BAZEL.md
EOF
  ;;
esac