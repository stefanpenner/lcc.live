# Remote Cache: When It Helps and When It Doesn't

This document explains the different caching strategies for Bazel and when each one is useful.

## The Three Types of Caching

### 1. Local Disk Cache (What We Use)

**Configuration**: `--disk_cache=~/.cache/bazel-disk-cache`

**How it works**:
- Bazel writes build outputs to a local directory
- Subsequent builds on the same machine reuse these outputs
- In CI: GitHub Actions caches this directory between workflow runs

**Pros**:
- ✅ Simple, no external services
- ✅ Fast (local filesystem)
- ✅ Free
- ✅ Works offline

**Cons**:
- ❌ Not shared across machines
- ❌ Each CI job starts fresh (no intra-workflow sharing)
- ❌ Local devs don't benefit from CI builds

**When to use**: 
- Small teams
- Simple projects
- Cost-sensitive projects
- This is the **default** and **recommended** for most projects

### 2. Remote Cache Server (Advanced)

**Configuration**: `--remote_cache=grpcs://remote.buildbuddy.io`

**How it works**:
- Bazel uploads build outputs to a remote server
- Any machine/job can download these cached outputs
- Persistent across all builds, machines, and developers

**Pros**:
- ✅ Shared across all machines and developers
- ✅ CI jobs can share cache within a workflow
- ✅ Local devs benefit from CI builds
- ✅ Massive speedups for clean builds

**Cons**:
- ❌ Requires external service (cost or self-hosting)
- ❌ Network latency for uploads/downloads
- ❌ More complex setup
- ❌ Requires authentication/authorization

**When to use**:
- Large teams (5+ developers)
- Frequent clean builds
- Long build times (>5 minutes)
- Multi-repo projects with shared dependencies

### 3. Ephemeral Local Cache Server (❌ Not Recommended)

**Configuration**: Running `bazel-remote` locally in each CI job

**Why it doesn't help in CI**:
```
┌─────────────┐
│ CI Job      │
│             │
│ bazel-remote│ ← Started fresh each run
│ (ephemeral) │ ← No persistence between runs
└─────────────┘
      ↓
   GitHub Actions Cache
   (works same as disk cache)
```

The cache server is ephemeral and doesn't persist between CI runs. The GitHub Actions cache restoration works the same whether you use bazel-remote or Bazel's disk cache.

**This approach adds**:
- 5-10 seconds to start cache server
- Docker container overhead
- Unnecessary complexity

**With no benefit over**: Just using `--disk_cache`

## Current Setup Analysis

### What We Have (Good! ✅)

```yaml
# In .bazelrc
build --disk_cache=~/.cache/bazel-disk-cache

# In .github/workflows/ci.yaml
- name: Mount Bazel cache
  uses: actions/cache@v4
  with:
    path: ~/.cache/bazel-disk-cache
```

**How it works in CI**:

1. **First run** (cache miss):
   ```
   Build everything → Write to disk cache → Upload cache to GitHub
   Duration: ~10-15 minutes
   ```

2. **Second run** (cache hit):
   ```
   Download cache from GitHub → Bazel reads from disk cache → Fast!
   Duration: ~2-4 minutes
   ```

3. **After code change**:
   ```
   Download cache → Rebuild changed files only → Update cache
   Duration: ~3-6 minutes
   ```

### Cache Hit Rates

With GitHub Actions cache:
- **Same branch, no changes**: ~95% cache hit
- **After small Go change**: ~80% cache hit (only changed packages rebuild)
- **After MODULE.bazel change**: ~20% cache hit (many dependencies rebuild)
- **Different branch, same code**: ~90% cache hit (fallback keys work)

## When to Consider Remote Cache

Consider BuildBuddy or self-hosted remote cache if:

1. **Your team is large**: 5+ developers
2. **Build times are slow**: >5 minutes for full build
3. **Many clean builds**: Developers frequently run `bazel clean`
4. **CI is slow**: >10 minutes even with current caching
5. **Multiple repos**: Shared dependencies across projects

### Cost-Benefit Analysis

**BuildBuddy Free Tier**:
- Cost: Free
- Storage: 10GB
- Worth it if: Build times > 10 minutes

**BuildBuddy Pro** ($100/month):
- Cost: $100/month
- Storage: 1TB
- Worth it if: Team saves > 4 hours/month waiting for builds

**Self-hosted bazel-remote**:
- Cost: ~$20/month (small VPS)
- Storage: Configurable
- Worth it if: You have ops time to maintain it

### Quick Calculation

If your team is:
- 5 developers
- Each waits 30 minutes/day for builds
- Remote cache cuts this by 50%
- Savings: 5 × 15 min × 20 days = **1,500 minutes/month = 25 hours/month**

At $50/hour, that's **$1,250/month in value** → BuildBuddy Pro pays for itself 12x over!

## Recommendation

**For most projects**: Stick with disk cache + GitHub Actions cache (current setup)

**Upgrade to remote cache when**:
- Team > 5 developers OR
- Build times > 10 minutes OR
- You're spending > $100/month in CI compute time

## Testing Remote Cache

Want to test if remote cache would help? Try BuildBuddy free tier:

1. Sign up: https://www.buildbuddy.io
2. Get API key
3. Create `.bazelrc.remote`:
   ```bash
   build --remote_cache=grpcs://remote.buildbuddy.io
   build --remote_header=x-buildbuddy-api-key=YOUR_API_KEY
   ```
4. Add to CI workflow
5. Monitor the BuildBuddy dashboard for hit rates

If you see >80% cache hit rates and significant speedups, it's worth keeping!

## Related Resources

- [Bazel Remote Caching Documentation](https://bazel.build/remote/caching)
- [BuildBuddy Pricing](https://www.buildbuddy.io/pricing)
- [bazel-remote (self-hosted)](https://github.com/buchgr/bazel-remote)

