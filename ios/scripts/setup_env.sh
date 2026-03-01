#!/bin/bash
# Script to setup local development environment

set -e

echo "🔧 Setting up LCC.live development environment..."

# Create .env.local if it doesn't exist
if [ ! -f .env.local ]; then
    cat > .env.local << EOF
# Local Development Environment Variables
# Copy this file to .env.local and customize for your setup

# API Configuration
LCC_API_BASE_URL=https://lcc.live
# LCC_API_BASE_URL=http://localhost:3000  # Uncomment for local backend

# Metrics Configuration  
GRAFANA_METRICS_URL=https://lcc.live/api/metrics
# GRAFANA_METRICS_URL=http://localhost:3000/api/metrics  # Uncomment for local backend

# Feature Flags
METRICS_ENABLED=true
DEBUG_LOGGING=true

# Timeouts & Intervals (in seconds)
NETWORK_TIMEOUT=30
IMAGE_REFRESH_INTERVAL=5
API_CHECK_INTERVAL=30

# Set to "1" to use localhost in debug builds
USE_LOCALHOST=0
EOF
    echo "✅ Created .env.local"
else
    echo "ℹ️  .env.local already exists"
fi

# Install Fastlane if not already installed
if ! command -v fastlane &> /dev/null; then
    echo "📦 Installing Fastlane..."
    gem install fastlane
else
    echo "✅ Fastlane already installed"
fi

# Install Bundler dependencies
if [ -f Gemfile ]; then
    echo "📦 Installing Ruby dependencies..."
    bundle install
fi

# Install SwiftLint if not already installed
if ! command -v swiftlint &> /dev/null; then
    echo "📦 Installing SwiftLint..."
    brew install swiftlint
else
    echo "✅ SwiftLint already installed"
fi

echo "✅ Development environment setup complete!"
echo ""
echo "Next steps:"
echo "1. Open lcc.xcodeproj in Xcode"
echo "2. Configure signing in Xcode (Signing & Capabilities)"
echo "3. Run the app on simulator or device"
echo ""
echo "Optional:"
echo "- Edit .env.local to customize environment variables"
echo "- Run 'fastlane test' to run tests"
echo "- Run 'fastlane ios build' to create a build"

