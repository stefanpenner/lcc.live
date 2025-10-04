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
  echo "📦 Updating dependencies from go.mod..."
  bazel run //:gazelle-update-repos
  bazel run //:gazelle
  echo "✅ Dependencies updated"
  ;;

opt)
  echo "🚀 Building optimized binary..."
  bazel build --config=opt //:lcc-live
  echo "✅ Optimized build complete: bazel-bin/lcc-live_/lcc-live"
  ;;

help | *)
  cat <<EOF
Bazel helper script for lcc.live

Usage: ./b <command>

Commands:
  build    - Build the binary
  test     - Run all tests
  run      - Run the server
  clean    - Clean build artifacts
  gazelle  - Regenerate BUILD files
  deps     - Update dependencies from go.mod
  opt      - Build optimized binary for production
  help     - Show this help message

Examples:
  ./b build
  ./b test
  ./b run

For more details, see doc/BAZEL.md
EOF
  ;;
esac
