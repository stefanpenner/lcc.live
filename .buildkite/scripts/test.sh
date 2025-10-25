#!/bin/bash
set -euo pipefail

# Create non-root user for running Bazel (rules_python requires non-root)
if [ "$(id -u)" -eq 0 ]; then
  echo "🔧 Running as root, setting up environment..."
  
  # Always check and install Docker (do this as root before switching users)
  if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
    echo "🐳 Installing Docker..."
    apt-get update
    apt-get install -y ca-certificates curl gnupg lsb-release
    
    # Add Docker repository
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) stable" | \
      tee /etc/apt/sources.list.d/docker.list > /dev/null
    
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    
    echo "✅ Docker installed"
  fi
  
  # Start Docker daemon if not running
  if ! docker info >/dev/null 2>&1; then
    echo "🚀 Starting Docker daemon..."
    dockerd > /tmp/dockerd.log 2>&1 &
    sleep 5
  fi
  
  # Create buildkite user
  USERNAME="buildkite"
  useradd -m -s /bin/bash "$USERNAME" || true
  
  # Add buildkite user to docker group (create group if needed)
  groupadd docker 2>/dev/null || true
  usermod -aG docker "$USERNAME" || true
  
  # Setup environment for non-root user
  chown -R "$USERNAME:$USERNAME" "$PWD" || true
  
  # Run remaining commands as non-root user
  exec su -c "$0" "$USERNAME"
fi

# Setup environment
export PATH="$PWD/.buildkite/bin:$PATH"
mkdir -p .buildkite/bin ~/.cache/bazel ~/.cache/bazelisk ~/.cache/bazel-disk-cache ~/.cache/bazel-repo

# Install Bazelisk
echo "📦 Installing Bazelisk..."
curl -sSL --retry 3 -o .buildkite/bin/bazelisk https://github.com/bazelbuild/bazelisk/releases/latest/download/bazelisk-linux-amd64
chmod +x .buildkite/bin/bazelisk
ln -sf bazelisk .buildkite/bin/bazel
echo "✅ Bazelisk installed"

bazel --version

# Configure BuildBuddy remote cache
if [ -n "${BUILDBUDDY_API_KEY:-}" ]; then
  echo "✅ BuildBuddy API key is set"
  printf 'common --remote_cache=grpcs://remote.buildbuddy.io\n' > .bazelrc.remote.ci
  printf 'common --remote_header=x-buildbuddy-api-key=%s\n' "${BUILDBUDDY_API_KEY}" >> .bazelrc.remote.ci
  printf 'common --remote_timeout=60s\n' >> .bazelrc.remote.ci
  printf 'common --remote_upload_local_results=true\n' >> .bazelrc.remote.ci
  printf 'common --bes_results_url=https://app.buildbuddy.io/invocation/\n' >> .bazelrc.remote.ci
  printf 'common --remote_executor=grpcs://remote.buildbuddy.io\n' >> .bazelrc.remote.ci
  printf 'common --remote_local_fallback\n' >> .bazelrc.remote.ci
  printf 'common --strategy=TestRunner=remote,local\n' >> .bazelrc.remote.ci
  printf 'common --strategy=GoLink=remote,local\n' >> .bazelrc.remote.ci
  printf 'common --strategy=GoCompile=remote,local\n' >> .bazelrc.remote.ci
  printf 'common --remote_cache_compression\n' >> .bazelrc.remote.ci
  printf 'common --remote_download_toplevel\n' >> .bazelrc.remote.ci
  echo "✅ BuildBuddy remote cache configured"
  echo "📊 Monitor builds at: https://app.buildbuddy.io"
else
  echo "⚠️  BuildBuddy API key not set - skipping remote cache"
fi

# Verify Docker is working
if command -v docker >/dev/null 2>&1; then
  echo "🐳 Verifying Docker is operational..."
  if docker info >/dev/null 2>&1; then
    echo "✅ Docker is running and accessible"
    docker version
  else
    echo "⚠️  Docker daemon not accessible, trying to start..."
    dockerd > /tmp/dockerd.log 2>&1 &
    sleep 5
  fi
fi

# Run tests
echo "🧪 Running all tests..."
bazel test --config=ci --test_output=errors //...

# Cleanup
rm -f .bazelrc.remote.ci || true

echo "✅ All tests passed"
