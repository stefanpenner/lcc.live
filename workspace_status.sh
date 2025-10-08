#!/bin/bash
# This script will be run by Bazel to provide stamping variables
# See: https://bazel.build/docs/user-manual#workspace-status

# Get git commit hash (short version)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")

# Get a stable build timestamp tied to the current commit (ISO 8601)
# This ensures Bazel cache keys remain stable across reruns for the same commit.
COMMIT_TIME=$(git show -s --format=%cI HEAD 2>/dev/null || echo "1970-01-01T00:00:00Z")

# Print stable status variables (these trigger rebuilds when changed)
echo "STABLE_GIT_COMMIT ${GIT_COMMIT}"
echo "STABLE_BUILD_TIMESTAMP ${COMMIT_TIME}"

# Print volatile status variables (these don't trigger rebuilds)
# Kept for reference; not used in BUILD rules to avoid cache busting.
echo "BUILD_TIMESTAMP ${COMMIT_TIME}"

