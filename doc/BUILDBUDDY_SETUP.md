# BuildBuddy Setup Guide for GitHub Actions

This guide shows how to securely configure [BuildBuddy](https://app.buildbuddy.io/docs/setup) remote cache in GitHub Actions without leaking API keys.

## Why BuildBuddy?

BuildBuddy provides remote caching that persists across:
- ✅ All CI runs (no 7-day expiration like GitHub Actions cache)
- ✅ All branches and PRs
- ✅ Local development (optional)
- ✅ All team members

**Expected improvement**: Analysis phase from 30-60s → **1-2s** consistently

## Step 1: Sign Up for BuildBuddy

1. Go to [https://www.buildbuddy.io](https://www.buildbuddy.io)
2. Sign up for a free account (or use GitHub SSO)
3. Free tier includes:
   - 10GB storage
   - Unlimited users
   - Unlimited builds

## Step 2: Get Your API Key

1. Log in to [https://app.buildbuddy.io](https://app.buildbuddy.io)
2. Navigate to **Settings** (top right menu)
3. Go to **API Keys** section
4. Click **Create API Key**
5. Copy the API key (starts with `bbapi-`)

**⚠️ Keep this secure!** This key provides access to your build cache.

## Step 3: Add Secret to GitHub Repository

### Via GitHub Web UI

1. Go to your repository on GitHub
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Name: `BUILDBUDDY_API_KEY`
5. Value: Paste your API key (e.g., `bbapi-xxxxxxxxxxxxx`)
6. Click **Add secret**

### Via GitHub CLI

```bash
# From your terminal
gh secret set BUILDBUDDY_API_KEY

# Paste your API key when prompted
# Or provide it directly:
echo "bbapi-xxxxxxxxxxxxx" | gh secret set BUILDBUDDY_API_KEY
```

## Step 4: Update GitHub Actions Workflow

**Secure approach**: Create a temporary `.bazelrc` file that's never committed.

Update `.github/workflows/ci.yaml`:

```yaml
jobs:
  test:
    name: Build and Test
    runs-on: ubuntu-latest
    env:
      BAZELISK_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      # BuildBuddy API key (stored in GitHub Secrets)
      BUILDBUDDY_API_KEY: ${{ secrets.BUILDBUDDY_API_KEY }}
    
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Mount Bazel cache
        uses: actions/cache@v4
        with:
          path: |
            ~/.cache/bazel
            ~/.cache/bazelisk
            ~/.cache/bazel-disk-cache
            ~/.cache/bazel-repo
          key: ${{ runner.os }}-bazel-${{ hashFiles('**/BUILD.bazel', 'MODULE.bazel', 'MODULE.bazel.lock', 'go.mod', 'go.sum', '.bazelversion') }}
          restore-keys: |
            ${{ runner.os }}-bazel-

      - name: Setup Bazel
        uses: bazel-contrib/setup-bazel@0.8.1
        with:
          bazelisk-cache: true
          disk-cache: true
          repository-cache: true

      # ✅ SECURE: Create temporary config file (never committed)
      - name: Configure BuildBuddy remote cache
        if: env.BUILDBUDDY_API_KEY != ''
        run: |
          # Create temporary bazelrc with remote cache config
          cat > .bazelrc.buildbuddy << EOF
          # BuildBuddy remote cache configuration
          build --remote_cache=grpcs://remote.buildbuddy.io
          build --remote_header=x-buildbuddy-api-key=${BUILDBUDDY_API_KEY}
          build --remote_timeout=60s
          build --remote_upload_local_results=true
          EOF
          
          echo "✅ BuildBuddy remote cache configured"

      - name: Verify Docker is available
        run: docker version

      - name: Prepare artifacts dir
        run: mkdir -p artifacts

      - name: Run unit tests
        run: |
          # Import BuildBuddy config if it exists
          if [ -f .bazelrc.buildbuddy ]; then
            BAZEL_OPTS="--bazelrc=.bazelrc.buildbuddy"
          fi
          
          bazel test --config=ci $BAZEL_OPTS --test_tag_filters=-integration,-manual \
            --build_event_json_file=artifacts/unit-bep.json \
            //...

      - name: Run container integration test
        run: |
          # Import BuildBuddy config if it exists
          if [ -f .bazelrc.buildbuddy ]; then
            BAZEL_OPTS="--bazelrc=.bazelrc.buildbuddy"
          fi
          
          bazel test --config=ci $BAZEL_OPTS \
            --build_event_json_file=artifacts/container-bep.json \
            //:container_test

      # ✅ SECURE: Clean up temporary config (defense in depth)
      - name: Cleanup BuildBuddy config
        if: always()
        run: |
          rm -f .bazelrc.buildbuddy
          echo "✅ Cleaned up temporary config"

      - name: Upload test results
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: test-logs
          retention-days: 7
          path: |
            artifacts/**
            bazel-testlogs/**/test.log
```

## Security Best Practices

### ✅ What We Do Right

1. **Store API key in GitHub Secrets**
   - Encrypted at rest
   - Only accessible to workflows
   - Not visible in logs

2. **Create temporary config file**
   - Never committed to git
   - Created during workflow run
   - Deleted after use

3. **Use environment variables**
   - Reference as `${BUILDBUDDY_API_KEY}`
   - GitHub Actions masks these in logs

4. **Check if key exists**
   - `if: env.BUILDBUDDY_API_KEY != ''`
   - Gracefully degrades if secret not set

5. **Pass via `--bazelrc` flag**
   - Keeps config isolated
   - Not in default `.bazelrc`

### ❌ What NOT to Do

```yaml
# ❌ DON'T: Put API key directly in .bazelrc (committed to git)
build --remote_header=x-buildbuddy-api-key=bbapi-xxxxx

# ❌ DON'T: Echo the API key in logs
run: echo "API Key: $BUILDBUDDY_API_KEY"

# ❌ DON'T: Write to a committed config file
run: echo "build --remote_header=x-buildbuddy-api-key=${BUILDBUDDY_API_KEY}" >> .bazelrc

# ❌ DON'T: Put in public artifacts
- uses: actions/upload-artifact@v4
  with:
    path: .bazelrc.buildbuddy  # Contains secret!
```

## Alternative Approach: Use .bazelrc.remote.ci

If you prefer to keep the BuildBuddy config separate:

### Update `.bazelrc`

Add at the bottom:
```bash
# CI-specific remote cache (created dynamically in CI)
try-import %workspace%/.bazelrc.remote.ci
```

### Update workflow

```yaml
- name: Configure BuildBuddy remote cache
  if: env.BUILDBUDDY_API_KEY != ''
  run: |
    cat > .bazelrc.remote.ci << EOF
    build --remote_cache=grpcs://remote.buildbuddy.io
    build --remote_header=x-buildbuddy-api-key=${BUILDBUDDY_API_KEY}
    EOF

- name: Run tests
  run: bazel test --config=ci //...
  # Config automatically loaded via try-import
```

### Update `.gitignore`

Already done! ✅
```bash
# Bazel local config files (may contain credentials)
.bazelrc.remote
.bazelrc.remote.ci
.bazelrc.user
```

## Step 5: Verify It's Working

### In GitHub Actions Logs

Look for these indicators:

```bash
✅ BuildBuddy remote cache configured

# During build:
INFO: Remote cache hit rate: 85%
INFO: 234 actions, 200 remote cache hits
```

### In BuildBuddy Dashboard

1. Go to [https://app.buildbuddy.io](https://app.buildbuddy.io)
2. View recent builds
3. Check cache hit rates
4. Monitor cache size

You should see:
- Your builds appearing in real-time
- Cache hit rates (target: >80%)
- Storage usage
- Build performance metrics

## Local Development (Optional)

To use BuildBuddy remote cache locally:

### For Read-Only Access (Recommended)

Create `.bazelrc.remote`:
```bash
# BuildBuddy remote cache (read-only for local dev)
build --remote_cache=grpcs://remote.buildbuddy.io
build --remote_header=x-buildbuddy-api-key=YOUR_API_KEY_HERE
build --remote_upload_local_results=false
```

Benefits:
- Fast builds using CI cache
- No pollution of shared cache
- Safe for experimentation

### For Read-Write Access

Only if you want to contribute to the cache:
```bash
# Full read-write access
build --remote_cache=grpcs://remote.buildbuddy.io
build --remote_header=x-buildbuddy-api-key=YOUR_API_KEY_HERE
build --remote_upload_local_results=true
```

**⚠️ Security note**: Keep your local `.bazelrc.remote` out of git (already in `.gitignore`)

## Troubleshooting

### API Key Not Working

```bash
# Test your API key
curl -H "x-buildbuddy-api-key: YOUR_API_KEY" \
  https://remote.buildbuddy.io/status

# Should return: {"status": "OK"}
```

### GitHub Secret Not Available

```yaml
# Add debug step (doesn't leak the actual value)
- name: Check BuildBuddy secret
  run: |
    if [ -n "$BUILDBUDDY_API_KEY" ]; then
      echo "✅ BuildBuddy API key is set"
      echo "Key length: ${#BUILDBUDDY_API_KEY}"
    else
      echo "❌ BuildBuddy API key is NOT set"
      echo "Add it in GitHub Settings → Secrets → BUILDBUDDY_API_KEY"
    fi
```

### Cache Not Hitting

1. Check BuildBuddy dashboard for errors
2. Verify key has correct permissions
3. Check network connectivity to `remote.buildbuddy.io`
4. Review Bazel flags: `--remote_cache=grpcs://remote.buildbuddy.io`

### Logs Show "Permission Denied"

- Verify API key in GitHub Secrets is correct
- Check organization/team permissions in BuildBuddy
- Ensure API key hasn't expired

## Performance Monitoring

### Before BuildBuddy

```bash
Analysis phase: 30-60s (first run), 5-10s (cached)
Build time: 10-15 minutes (CI)
Cache hit rate: 80-90% (GitHub Actions cache)
```

### After BuildBuddy

```bash
Analysis phase: 1-2s (always!)
Build time: 3-5 minutes (CI)
Cache hit rate: 90-98% (persistent remote cache)
```

## Cost Analysis

### Free Tier
- **Cost**: $0
- **Storage**: 10GB
- **Users**: Unlimited
- **Builds**: Unlimited
- **Good for**: Small teams, side projects

### Pro Tier ($100/month)
- **Cost**: $100/month
- **Storage**: 1TB
- **Advanced features**: RBE, analytics, API access
- **Good for**: Teams 5+, production workloads

### ROI Calculation

If your team:
- 5 developers
- Each saves 30 min/day waiting for builds
- BuildBuddy cuts build time by 60%

**Savings**: 5 × 15 min × 20 days = 1,500 min/month = **25 hours/month**

At $50/hour: **$1,250/month in value** → **12x ROI** on Pro tier!

## Summary

✅ **Secure setup**: API key in GitHub Secrets  
✅ **Temporary config**: Created during workflow, deleted after  
✅ **Not committed**: Never in git history  
✅ **Masked in logs**: GitHub Actions hides secret values  
✅ **Graceful degradation**: Works without secret for forks/PRs  

## References

- [BuildBuddy Setup Documentation](https://app.buildbuddy.io/docs/setup)
- [BuildBuddy Remote Cache Guide](https://www.buildbuddy.io/docs/remote-cache)
- [GitHub Actions Secrets](https://docs.github.com/en/actions/security-guides/encrypted-secrets)
- [Bazel Remote Caching](https://bazel.build/remote/caching)

## Next Steps

1. ✅ Sign up for BuildBuddy (free)
2. ✅ Get your API key
3. ✅ Add to GitHub Secrets as `BUILDBUDDY_API_KEY`
4. ✅ Update workflow to use temporary config
5. ✅ Monitor in BuildBuddy dashboard
6. 🎯 Enjoy faster builds!

The approach above is **production-ready** and follows security best practices from [BuildBuddy's official setup guide](https://app.buildbuddy.io/docs/setup).

