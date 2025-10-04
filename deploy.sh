#!/usr/bin/env bash
set -euo pipefail

# Deploy lcc.live to Fly.io using Bazel-built OCI images

echo "🏗️  Building OCI image with Bazel..."
bazel build --config=opt //:image

echo "📦 Loading image into Docker..."
bazel run --config=opt //:image_load

echo "🚀 Deploying to Fly.io..."
fly deploy --local-only --image lcc.live:latest

echo "✅ Deployment complete!"
echo "🔍 Check version at: https://lcc.live/_/version"

