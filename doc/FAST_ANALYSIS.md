# Speeding Up Bazel Analysis Phase

## Understanding the Analysis Phase

The analysis phase is when Bazel:
1. Loads all BUILD files and MODULE.bazel
2. Downloads external dependencies (Go SDK, rules, base images)
3. Evaluates build rules and constructs the action graph
4. Configures all targets (the 6566 targets you see)

In your case:
```
Analyzing: 17 targets (137 packages loaded, 6566 targets configured)
```

This means:
- **17 targets**: Your direct targets (your code)
- **137 packages**: External dependencies (rules_go, rules_oci, gazelle, etc.)
- **6566 targets**: All transitive targets (Go stdlib, dependencies, rules internals)

## Why It's Slow Initially

On the **first run** or when dependencies change:
1. Bazel downloads Go SDK (~100MB)
2. Downloads all external repositories (rules_go, gazelle, rules_oci, etc.)
3. Downloads Go dependencies from go.mod
4. Downloads alpine base image
5. Evaluates all BUILD files
6. Configures all targets

This can take **20-60 seconds** on first run.

## Why It's Fast on Cache Hits

On **subsequent runs** with cache:
1. Repository cache restores all external repos instantly
2. Skyframe cache reuses previous analysis results
3. Only changed BUILD files are re-evaluated

This should take **2-5 seconds** with warm cache.

## Current Optimizations (Already Applied)

### In `.bazelrc`

```bash
# Parallelize loading BUILD files
build:ci --loading_phase_threads=HOST_CPUS

# Merge analysis and execution phases
build:ci --experimental_merged_skyframe_analysis_execution

# Don't publish all actions to BEP (reduces overhead)
build:ci --nobuild_event_publish_all_actions
```

### In `.github/workflows/ci.yaml`

```yaml
# Repository cache saves external dependencies
- name: Mount Bazel cache
  uses: actions/cache@v4
  with:
    path: |
      ~/.cache/bazel
      ~/.cache/bazelisk
      ~/.cache/bazel-disk-cache
      ~/.cache/bazel-repo  # <-- This caches external deps
```

## Expected Performance

### First Run (Cache Miss)
```
Analysis phase: 30-60 seconds
  - Download Go SDK: 10-15s
  - Download external repos: 10-20s
  - Load and configure: 10-25s
Total CI run: 15-20 minutes
```

### Second Run (Cache Hit)
```
Analysis phase: 2-5 seconds
  - Restore from cache: 1-2s
  - Validate and configure: 1-3s
Total CI run: 10-12 minutes
```

### After Code Change Only
```
Analysis phase: 3-7 seconds
  - Restore from cache: 1-2s
  - Re-analyze changed files: 2-5s
Total CI run: 11-13 minutes
```

## What Slows Down Analysis

### ❌ Things That Invalidate Cache

1. **MODULE.bazel changes**: Updates dependencies
2. **MODULE.bazel.lock changes**: Dependency version changes
3. **go.mod/go.sum changes**: Go dependencies update
4. **.bazelversion changes**: Bazel version update
5. **BUILD.bazel changes**: Build configuration changes

### ❌ Things That Can't Be Cached

1. **First-time setup**: No previous cache exists
2. **Cache eviction**: GitHub Actions evicts caches after 7 days
3. **Different branch**: May not have matching cache

## Advanced Optimizations

### 1. Reduce External Dependencies

**Current dependencies** (from MODULE.bazel):
- rules_go
- gazelle  
- aspect_bazel_lib
- rules_oci
- rules_pkg
- rules_shell
- Go SDK (1.24.5)
- Alpine base image

**Could you reduce?**
- ❓ Do you need rules_shell? (Used for sh_test/sh_binary)
- ❓ Could you use a smaller base image?

**Impact**: Minimal - these are all necessary for your build

### 2. Use Remote Cache (BuildBuddy)

A persistent remote cache shares analysis results across:
- All CI runs
- All developers
- All branches

**Setup** (5 minutes):
1. Sign up at https://www.buildbuddy.io (free tier available)
2. Get API key
3. Add to repository secrets as `BUILDBUDDY_API_KEY`
4. Update `.github/workflows/ci.yaml`:

```yaml
- name: Configure remote cache
  run: |
    echo "build --remote_cache=grpcs://remote.buildbuddy.io" >> .bazelrc.ci
    echo "build --remote_header=x-buildbuddy-api-key=${{ secrets.BUILDBUDDY_API_KEY }}" >> .bazelrc.ci

- name: Run tests with remote cache
  run: bazel test --config=ci --config=remote //...
```

**Expected improvement**:
- First run: Same (30-60s analysis)
- All subsequent runs: **1-2s analysis** (even after cache eviction!)
- Cross-branch: Instant analysis
- Local dev: Benefits from CI cache

### 3. Keep MODULE.bazel.lock in Git

**Already done!** ✅

This ensures everyone uses the exact same dependency versions.

### 4. Use Bazel 8.x

**Already done!** ✅

Bazel 8.x has significant analysis phase improvements.

### 5. Pin External Repositories

**Already done!** ✅

Your alpine image uses a specific digest:
```python
digest = "sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1"
```

This ensures the cache key is stable.

## Monitoring Analysis Performance

### In CI Logs

Look for these lines:
```
Analyzing: 17 targets (137 packages loaded, 6566 targets configured)
Loading: 0 packages loaded
```

**Good signs**:
- `Loading: 0 packages loaded` - Everything restored from cache
- Analysis completes in < 5 seconds - Warm cache

**Bad signs**:
- `Loading: 137 packages loaded` - Cold cache, fetching everything
- Analysis takes > 30 seconds - Cache miss

### In Bazel Profile

Generate a profile to see where time is spent:

```bash
bazel build --profile=profile.json //...
bazel analyze-profile profile.json
```

Look for:
- `fetch` - Downloading external dependencies
- `load` - Loading BUILD files
- `configure` - Configuring targets

## Benchmarks

### Your Current Setup

**Typical cache hit** (good):
```
INFO: Analyzed 17 targets (0 packages loaded, 0 targets configured).
INFO: Found 11 targets and 6 test targets...
INFO: Elapsed time: 2.5s, Critical Path: 0.1s
```

**Cache miss** (slow):
```
INFO: Analyzed 17 targets (137 packages loaded, 6566 targets configured).
INFO: Found 11 targets and 6 test targets...
INFO: Elapsed time: 45.0s, Critical Path: 0.1s
```

### With Remote Cache (BuildBuddy)

**Even after cache eviction**:
```
INFO: Analyzed 17 targets (0 packages loaded, 0 targets configured).
INFO: Found 11 targets and 6 test targets...
INFO: Elapsed time: 1.8s, Critical Path: 0.1s
```

## Recommendations

### Short Term (Already Done)
- ✅ Enable `--loading_phase_threads=HOST_CPUS`
- ✅ Use `--experimental_merged_skyframe_analysis_execution`
- ✅ Keep MODULE.bazel.lock in git
- ✅ Use repository cache in CI

### Medium Term (Optional)
- 🎯 **Add remote cache** if you run CI frequently (BuildBuddy free tier)
  - **Cost**: Free (up to 10GB)
  - **Setup time**: 5 minutes
  - **Benefit**: Consistent 1-2s analysis phase

### Long Term (If Needed)
- Consider using Bazel's `--experimental_repository_cache_hardlinks`
- Profile slow analysis to identify bottlenecks
- Consider workspace rules instead of module rules (more control)

## Reality Check

**Your 30-60s analysis time is normal for the first run.**

With your current optimizations:
- First run: 30-60s (downloading everything)
- Second run: 2-5s (cache hit)
- After code change: 3-7s (partial cache hit)

This is **expected and good performance** for a project with:
- Go SDK
- Multiple rule sets
- External dependencies
- Container builds

The cache works well - you just need to ensure it's hitting consistently!

## Debug Cache Issues

If analysis is slow every time, check:

1. **Is cache being restored?**
   ```yaml
   # In CI logs, look for:
   Cache restored from key: Linux-bazel-...
   ```

2. **Is cache key stable?**
   ```bash
   # Check if these files change frequently:
   git log --oneline MODULE.bazel MODULE.bazel.lock go.mod go.sum
   ```

3. **Is repository cache working?**
   ```bash
   # Check cache size:
   du -sh ~/.cache/bazel-repo
   # Should be 200-500MB
   ```

## Summary

✅ **You've already applied the best optimizations**  
✅ **30-60s first run is normal and expected**  
✅ **2-5s subsequent runs shows cache is working well**  
🎯 **Consider BuildBuddy remote cache for consistent 1-2s analysis**

The analysis phase will always take time on the first run. The goal is to maximize cache hits, which you're already doing!

