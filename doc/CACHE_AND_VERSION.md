# Cache Control and Version Improvements

## Overview

This document describes the improvements made to cache control and version tracking for lcc.live.

## Changes Made

### 1. Improved Cache Control for HTML Content

**Problem**: Cloudflare wasn't properly busting the cache for index.html.

**Solution**: Changed the cache control strategy for HTML pages from:
```
Cache-Control: public, max-age=60
```

To:
```
Cache-Control: public, no-cache, must-revalidate
```

**What this does**:
- `no-cache`: Forces CDN/browsers to revalidate with the origin server before serving cached content
- `must-revalidate`: Ensures that stale content is not served without revalidation
- Still allows caching via ETags, so if content hasn't changed, the server responds with 304 Not Modified
- The ETag validation now properly returns 304 when content hasn't changed

**Files Changed**:
- `server/canyon_route.go`: Updated cache headers and added ETag validation

### 2. Version Information

**Added Features**:
1. **`/_/version` Endpoint**: Returns JSON with build information:
   ```json
   {
     "version": "e8d71ef",           // Git commit hash
     "build_time": "2025-10-04_17:24:52_UTC",
     "go_version": "go1.23.3",
     "uptime": "2h34m12s"
   }
   ```

2. **`X-Version` HTTP Header**: All responses now include this header with the version
   ```
   X-Version: e8d71ef
   ```
   (Shows version and Go version in dev mode)

**Implementation**:
- `server/version.go`: Version information structure and getters
- `server/version_route.go`: HTTP endpoint handler
- `server/server.go`: Middleware to add X-Version header to all responses
- `build.sh`: Updated to inject git commit and build time via ldflags

**Build Time Injection**:

The version information is injected at build time:

**With build.sh** (using Go's `-ldflags`):
```bash
./build.sh
# Injects: Version=<git-sha> BuildTime=<timestamp>
```

**With Bazel** (using `x_defs` and workspace status):
```bash
# Development build (no stamping)
bazel build //:lcc-live

# Production build (with stamping)
bazel build --stamp //:lcc-live
# or
bazel build --config=opt //:lcc-live
```

The workspace status script (`workspace_status.sh`) provides:
- `STABLE_GIT_COMMIT`: Current git commit hash (triggers rebuilds when changed)
- `BUILD_TIMESTAMP`: Build timestamp (doesn't trigger unnecessary rebuilds)

### 3. Testing

All changes are thoroughly tested:
- `server/version_route_test.go`: Tests for version endpoint and headers
- Updated existing tests to expect new cache control headers
- Added tests for ETag validation and 304 responses

## Benefits

1. **Better Cache Control**: HTML is always fresh because CDN checks with origin, but still efficient via ETags
2. **Version Tracking**: Easy to verify which version is deployed
3. **Debugging**: Version in headers makes it easy to see what's running without hitting the endpoint
4. **Monitoring**: The `/_/version` endpoint can be used for deployment verification
5. **Clean URL Structure**: Internal/admin endpoints organized under `/_/` prefix

## Usage

### Checking Version

**Via curl**:
```bash
# Get version from endpoint
curl https://lcc.live/_version

# Get version from header
curl -I https://lcc.live/ | grep X-Version
```

**Via browser**:
- Visit `https://lcc.live/_/version`
- Or check the `X-Version` header in browser DevTools Network tab

**Other internal endpoints**:
- `/_/metrics` - Prometheus metrics endpoint

### Cache Behavior

The HTML pages now:
1. Are cached by CDN/browsers
2. Always validate with origin before serving
3. Use ETags to avoid re-sending unchanged content (304 responses)
4. Result: Always fresh content, minimal bandwidth usage

## Deployment

### With Bazel
For production builds with Bazel, use:
```bash
bazel build --config=opt //:lcc-live
# The binary will be at: bazel-bin/lcc-live_/lcc-live
```

Or use the convenience script:
```bash
./build      # Wrapper script for Bazel builds
```

### Automatic Cache Purging

When deploying to Fly.io, Cloudflare cache is automatically purged via a release command. This ensures users immediately see the new version. See [CLOUDFLARE_CACHE_PURGE.md](CLOUDFLARE_CACHE_PURGE.md) for setup details.

