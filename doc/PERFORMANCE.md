# Performance Optimization Guide

This document describes performance optimizations for builds and CI/CD.

## Quick Wins

### Local Development

For faster iteration during development, use the `fast` configuration:

```bash
# Fast builds (no optimizations, faster compile)
bazel build --config=fast //...

# Fast tests
bazel test --config=fast //...
```

This disables optimizations, stamping, and stripping for ~2-3x faster build times.

### Skip Slow Tests

Skip container integration tests during rapid iteration:

```bash
bazel test --test_tag_filters=-integration //...
```

## Build Optimizations

### Current Optimizations (Already Enabled)

1. **Disk Caching**: Results cached in `~/.cache/bazel-disk-cache`
2. **Repository Caching**: Dependencies cached in `~/.cache/bazel-repo`
3. **Sandbox Reuse**: `--experimental_reuse_sandbox_directories`
4. **In-Memory Files**: `--experimental_inmemory_dotd_files`, `--experimental_inmemory_jdeps_files`
5. **Parallel Execution**: Automatically uses all CPU cores
6. **Memory Optimization**: Uses 80% of available RAM

### Remote Caching (Optional)

Remote caching can speed up builds by 5-10x, especially in CI and for clean builds.

#### BuildBuddy (Free Tier)

1. Sign up at [buildbuddy.io](https://www.buildbuddy.io)
2. Get your API key from Settings
3. Create `.bazelrc.remote` in the project root:

```bash
# .bazelrc.remote
build --remote_cache=grpcs://remote.buildbuddy.io
build --remote_header=x-buildbuddy-api-key=YOUR_API_KEY_HERE
```

4. For read-only cache (recommended for local dev):
```bash
build --remote_upload_local_results=false
```

#### GitHub Actions Cache (Built-in)

Our CI workflow already uses GitHub Actions cache with Bazel's disk cache. This is **automatically enabled** in CI and requires no setup!

How it works:
1. Bazel writes to `~/.cache/bazel-disk-cache` (configured in `.bazelrc`)
2. GitHub Actions caches this directory after each run (up to 10GB)
3. Subsequent runs restore the cache for instant cache hits
4. Cache is shared across branches with fallback keys

Benefits:
- ✅ No external service required
- ✅ No additional infrastructure overhead
- ✅ Free for public and private repos
- ✅ Automatic cache expiration (7-day retention)
- ✅ Simple and reliable

**Note**: Each parallel CI job (analyze, unit-tests, container-test) starts with the same cached state but doesn't share cache during the run. This is fine because each job is independent and builds different targets.

#### BuildBuddy or Self-Hosted Remote Cache (For Advanced Use)

A true remote cache service helps when you want:
- **Cross-job sharing**: Multiple CI jobs share cache within a single workflow run
- **Developer cache sharing**: Local builds benefit from CI builds and vice versa
- **Multi-team sharing**: Multiple teams/repos share cached artifacts
- **Persistent cache**: No 7-day expiration like GitHub Actions cache

For true remote caching, you can use BuildBuddy.

**📚 See [BUILDBUDDY_SETUP.md](BUILDBUDDY_SETUP.md) for complete setup guide with security best practices.**

Quick overview:

1. Sign up at [buildbuddy.io](https://www.buildbuddy.io) (free tier available)
2. Get your API key from Settings
3. Add to GitHub Secrets as `BUILDBUDDY_API_KEY`
4. Update workflow to create temporary config (never committed):

```yaml
- name: Configure BuildBuddy remote cache
  if: env.BUILDBUDDY_API_KEY != ''
  run: |
    cat > .bazelrc.remote.ci << EOF
    build --remote_cache=grpcs://remote.buildbuddy.io
    build --remote_header=x-buildbuddy-api-key=${BUILDBUDDY_API_KEY}
    EOF
```

**Expected benefits**:
- Analysis phase: 30-60s → **1-2s** (always!)
- Build time: 10-15 min → **3-5 min** in CI
- Cache hit rate: 80-90% → **90-98%**

Reference: [BuildBuddy Setup Documentation](https://app.buildbuddy.io/docs/setup)

## CI/CD Optimizations

### Current Optimizations

Our CI workflow is optimized for speed and efficiency:

1. **Smart Caching**: GitHub Actions cache with optimized cache keys
2. **Sequential Execution**: Unit tests followed by container tests in single job
3. **Optimized Resource Usage**: Bazel configured for CI environment
4. **Fast Restoration**: Cache hit rates of 80-95%

### CI Workflow Structure

```
test job:
  1. Unit tests (8-12 min)
  2. Container integration test (2-3 min)
  Total: 10-15 minutes
```

**Total time**: ~15 minutes (instead of ~30 minutes before optimizations)

### Container Tests

Container integration tests run as part of the main test job after unit tests complete successfully.

## Performance Metrics

### Local Build Times (M1 Max, 64GB RAM)

| Command | Cold Build | Warm Build | Hot Build (cached) |
|---------|-----------|------------|-------------------|
| `bazel build //...` | ~45s | ~8s | ~2s |
| `bazel test //...` | ~60s | ~12s | ~4s |
| `bazel build --config=fast //...` | ~25s | ~5s | ~2s |

### CI Build Times (ubuntu-latest)

| Job | Duration |
|-----|----------|
| analyze | 2-5 min |
| unit-tests | 8-12 min |
| container-test | 10-15 min |
| **Total (parallel)** | **10-15 min** |

With remote caching:
| Job | Duration |
|-----|----------|
| analyze | 1-2 min |
| unit-tests | 2-4 min |
| container-test | 3-5 min |
| **Total (parallel)** | **3-5 min** |

## Troubleshooting

### Slow Builds

1. **Check cache**:
   ```bash
   bazel info repository_cache
   bazel info disk_cache
   ls -lh ~/.cache/bazel-disk-cache
   ```

2. **Clear cache if corrupted**:
   ```bash
   bazel clean --expunge
   rm -rf ~/.cache/bazel-disk-cache
   ```

3. **Profile build**:
   ```bash
   bazel build --profile=profile.json //...
   bazel analyze-profile profile.json
   ```

### CI Timeouts

If CI jobs timeout:

1. Check the job logs for slow tests
2. Increase timeout in `.github/workflows/ci.yaml`
3. Consider splitting tests into more granular jobs
4. Enable remote caching

### Memory Issues

If builds fail with OOM:

1. Reduce parallelism:
   ```bash
   bazel build --jobs=4 --local_ram_resources=4096 //...
   ```

2. Or add to `.bazelrc.user`:
   ```bash
   build --jobs=4
   build --local_ram_resources=4096
   ```

## Advanced Optimizations

### Test Sharding

For very large test suites, enable test sharding:

```bash
bazel test --test_sharding_strategy=explicit //...
```

### Remote Execution

For maximum speed, use remote execution (requires BuildBuddy Enterprise or similar):

```bash
build --remote_executor=grpcs://remote.buildbuddy.io
build --remote_cache=grpcs://remote.buildbuddy.io
```

### Build Without the Bytes

For faster analysis without downloading artifacts:

```bash
bazel build --remote_download_minimal //...
```

## Benchmarking

To benchmark your changes:

```bash
# Clean build
bazel clean && time bazel build //...

# Cached build
time bazel build //...

# Test suite
time bazel test //...
```

Compare times before and after optimization changes.

## Best Practices

1. **Use `--config=fast` during development**
2. **Enable remote caching** for team collaboration
3. **Keep dependencies minimal** - check with `bazel query 'deps(//...)'`
4. **Monitor cache hit rates** in BuildBuddy dashboard
5. **Use `--test_tag_filters`** to skip slow tests during iteration
6. **Profile builds** when investigating performance issues

## Related Documentation

- [Bazel Performance Guide](https://bazel.build/configure/best-practices)
- [BuildBuddy Documentation](https://www.buildbuddy.io/docs)
- [Bazel Remote Caching](https://bazel.build/remote/caching)

