#!/usr/bin/env bash
# Bazel helper script for common operations

set -euo pipefail

COMMAND="${1:-help}"

# Check if Doppler is available, exit with error if not found
require_doppler() {
  if ! command -v doppler &> /dev/null; then
    echo "⚠️  Doppler not found. Install: https://docs.doppler.com/docs/install-cli" >&2
    exit 1
  fi
}

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
  export DEV_MODE=1
  echo "🚀 Running server in dev mode (hot reload enabled)..."
  require_doppler
  echo "🔐 Using Doppler to inject secrets..."
  # Explicitly specify project and config to avoid parent directory interference
  doppler run --project lcc-live --config dev -- bazel run //:lcc-live
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

secrets)
  echo "🔐 Showing Doppler secrets..."
  require_doppler
  echo ""
  echo "All secrets (including Doppler metadata):"
  doppler secrets --project lcc-live --config dev
  echo ""
  echo "Application secrets only (excluding Doppler metadata):"
  doppler secrets --project lcc-live --config dev --only-names | grep -v "^DOPPLER_" || doppler secrets --project lcc-live --config dev --only-names
  ;;

psql)
  echo "🗄️  Connecting to Neon database..."
  if ! command -v psql &> /dev/null; then
    echo "⚠️  psql not found. Install PostgreSQL client tools."
    exit 1
  fi
  require_doppler
  doppler run --project lcc-live --config dev -- bash -c 'psql "$NEON_DATABASE_URL" '"$(printf '%q ' "${@:2}")"
  ;;

sh)
  echo "🐚 Starting shell with Doppler environment variables..."
  require_doppler
  # Start a shell with Doppler secrets injected
  doppler run --project lcc-live --config dev -- "${SHELL:-/bin/bash}" "${@:2}"
  ;;

seed)
  echo "🌱 Seeding Neon database..."
  require_doppler
  doppler run --project lcc-live --config dev -- go run ./cmd/seed-neon --data ./seed.json
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
  secrets      - Show Doppler secrets
  psql         - Connect to Neon database using psql
  sh           - Start shell with Doppler environment variables
  seed         - Seed Neon database from seed.json
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