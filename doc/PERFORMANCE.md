# Performance Guide

## Quick Tips

```bash
# Fast builds during development
bazel build --config=fast //...

# Skip slow tests
bazel test --test_tag_filters=-integration //...
```

## Optimizations

Current `.bazelrc` optimizations:
- Disk & repository caching
- Parallel execution (all CPU cores)
- In-memory file handling
- 80% RAM utilization

## Remote Cache (Optional)

For even faster builds, use BuildBuddy:

1. Sign up at [buildbuddy.io](https://www.buildbuddy.io)
2. Get API key from Settings
3. Add to GitHub Secrets as `BUILDBUDDY_API_KEY`
4. CI will automatically use it (already configured)

Local setup in `.bazelrc.remote`:
```bash
build --remote_cache=grpcs://remote.buildbuddy.io
build --remote_header=x-buildbuddy-api-key=YOUR_KEY
build --remote_upload_local_results=false  # read-only
```

## CI Performance

Typical times:
- Unit tests: 8-12 min
- Container test: 2-3 min
- Total: 10-15 min

With BuildBuddy remote cache: 3-5 min total

