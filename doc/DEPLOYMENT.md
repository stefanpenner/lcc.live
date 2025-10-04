# Deployment Guide

This project uses Bazel with rules_oci to build container images for deployment to Fly.io.

## Overview

We use Bazel's `rules_oci` instead of traditional Dockerfiles for several benefits:
- **Better caching**: Bazel's intelligent caching speeds up builds
- **Reproducible builds**: Same inputs always produce the same outputs
- **Integrated tooling**: Build, test, and package in one system
- **Version stamping**: Automatically inject git commit and build time

## Prerequisites

- Bazel 8.0+ (via bazelisk)
- Docker (for loading images locally)
- Fly CLI (for deployment)

## Building

### Development Build

For local testing:
```bash
# Build just the binary
bazel build //:lcc-live

# Build and run tests
bazel test '...'
```

### Production Build

Build the OCI image with version stamping:
```bash
# Build OCI image
bazel build --config=opt //:image

# Load into local Docker
bazel run --config=opt //:image_load

# Verify the image
docker run --rm -p 3000:3000 lcc.live:latest
```

Or use the helper script:
```bash
./build      # Builds everything with optimization
```

## Deployment to Fly.io

### Quick Deploy

Use the deployment script:
```bash
./deploy.sh
```

This will:
1. Build the OCI image with Bazel
2. Load it into Docker
3. Deploy to Fly.io using the image

### Manual Deploy

```bash
# 1. Build the OCI image
bazel build --config=opt //:image

# 2. Load into Docker
bazel run --config=opt //:image_load

# 3. Deploy to Fly
fly deploy --local-only --image lcc.live:latest
```

### Verify Deployment

After deployment, check the version:
```bash
curl https://lcc.live/_/version
```

You should see the git commit hash and build timestamp in the response.

## OCI Image Details

The OCI image is built with:
- **Base image**: Alpine Linux (minimal, secure)
- **Binary**: Statically linked Go binary with embedded assets
- **Entry point**: `/usr/local/bin/lcc-live`
- **Exposed port**: 3000
- **Extras**: purge-cache.sh script for cache management

## Build Configuration

### .bazelrc Profiles

- `--config=fast`: Fast development builds (default)
- `--config=opt`: Optimized production builds with stamping
- `--config=debug`: Debug builds with symbols

### Version Stamping

The build automatically injects version info via `workspace_status.sh`:
- `STABLE_GIT_COMMIT`: Git commit hash (triggers rebuilds)
- `BUILD_TIMESTAMP`: Build timestamp (cached)

These are embedded in the binary at:
- `github.com/stefanpenner/lcc-live/server.Version`
- `github.com/stefanpenner/lcc-live/server.BuildTime`

## Troubleshooting

### Image won't build

If you get platform errors, ensure you're using the opt config:
```bash
bazel build --config=opt //:image
```

### Tests fail

Run tests without the image targets:
```bash
bazel test '...'
```

OCI image targets are tagged with `manual` so they don't build during tests.

### Cache issues

Clean the Bazel cache:
```bash
bazel clean --expunge
```

## CI/CD Integration

For automated deployments, add to your CI pipeline:

```yaml
# Example GitHub Actions
- name: Build and Deploy
  run: |
    bazel build --config=opt //:image
    bazel run --config=opt //:image_load
    fly deploy --local-only --image lcc.live:latest
```

## Comparison: Dockerfile vs rules_oci

### Previous (Dockerfile)
- ❌ Slower builds (no layer caching across machines)
- ❌ Less reproducible
- ❌ Separate tool (Docker required)
- ✅ Familiar to most developers

### Current (rules_oci)
- ✅ Fast builds with Bazel's caching
- ✅ Fully reproducible
- ✅ Integrated with Bazel ecosystem
- ✅ Better for mono-repos and complex builds
- ⚠️  Requires learning Bazel concepts

## Additional Resources

- [Bazel rules_oci documentation](https://github.com/bazel-contrib/rules_oci)
- [Fly.io deployment docs](https://fly.io/docs/languages-and-frameworks/dockerfile/)
- [Bazel workspace status](https://bazel.build/docs/user-manual#workspace-status)

