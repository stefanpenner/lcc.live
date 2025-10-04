#!/bin/bash
# This script will be run by Bazel to provide stamping variables
# See: https://bazel.build/docs/user-manual#workspace-status

# Get git commit hash (short version)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")

# Get build timestamp
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S_UTC')

# Print stable status variables (these trigger rebuilds when changed)
echo "STABLE_GIT_COMMIT ${GIT_COMMIT}"

# Print volatile status variables (these don't trigger rebuilds)
echo "BUILD_TIMESTAMP ${BUILD_TIME}"

