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

dev)
  echo "🌄 LCC Live Development Server (Bazel Native)"
  echo "📁 Using ibazel for file watching"
  echo "🔄 Auto-restart on file changes"
  echo "⚡ Browser auto-reload via polling"
  echo "📝 Watches: .go files, templates, and static files"
  echo ""
  
  # Check if ibazel is available
  if ! command -v ibazel >/dev/null 2>&1; then
    echo "❌ ibazel not found. Install with:"
    echo "   go install github.com/bazelbuild/bazel-watcher/cmd/ibazel@latest"
    echo "   or: brew install bazel-watcher"
    exit 1
  fi
  
  echo "✅ ibazel found, starting development server..."
  echo ""
  
  # Create output runner script for graceful server restarts
  OUTPUT_RUNNER="scripts/dev_output_runner.sh"
  
  # Use ibazel with output runner for graceful restarts
  # The output runner handles killing the old process before starting the new one
  # Trigger browser reload after any change is detected
  ibazel run //:lcc-live -- --output_runner="$OUTPUT_RUNNER" --run_command_after="scripts/trigger_reload.sh"
  ;;

dev:stop)
  echo "🛑 Stopping development server..."
  ./scripts/dev_stop.sh
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
  dev          - Run development server with live reload (ibazel)
  dev:stop     - Stop the development server
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