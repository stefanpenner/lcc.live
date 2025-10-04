# Automatic Cloudflare Cache Purging on Deploy

This document describes how automatic Cloudflare cache purging is configured for lcc.live deployments.

## Overview

When you deploy to Fly.io, the cache is automatically purged from Cloudflare. This ensures that users immediately see the new version without waiting for cache TTLs to expire.

## How It Works

1. **Fly.io Release Command**: After a successful deployment, Fly.io runs `/usr/local/bin/purge-cache.sh`
2. **Cache Purge Script**: The script calls the Cloudflare API to purge all cached content
3. **Version Tracking**: The `X-Version` header changes with each deploy, helping verify the purge worked

## Setup Instructions

### 1. Get Cloudflare Credentials

#### Get your Zone ID:
```bash
# Find your zone ID in Cloudflare Dashboard:
# 1. Log into Cloudflare
# 2. Select your domain (lcc.live)
# 3. Zone ID is on the right sidebar under "API"
```

#### Create an API Token:
```bash
# 1. Go to Cloudflare Dashboard → Profile → API Tokens
# 2. Click "Create Token"
# 3. Use "Edit zone DNS" template OR create custom token with:
#    - Permissions: Zone > Cache Purge > Purge
#    - Zone Resources: Include > Specific zone > lcc.live
# 4. Copy the token (shown only once!)
```

### 2. Set Fly.io Secrets

Store your Cloudflare credentials as Fly.io secrets:

```bash
# Set the zone ID
fly secrets set CLOUDFLARE_ZONE_ID=your-zone-id-here

# Set the API token
fly secrets set CLOUDFLARE_API_TOKEN=your-api-token-here
```

These secrets will be available as environment variables to the release command.

### 3. Deploy

```bash
fly deploy
```

The deployment process will:
1. Build the Docker image
2. Deploy the new version
3. Automatically run the cache purge script
4. Health check the new deployment

### 4. Verify

Check that the cache was purged:

```bash
# Check the version header (should show new git commit)
curl -I https://lcc.live/ | grep X-Version

# Check the version endpoint
curl https://lcc.live/_/version

# Check Fly.io logs for cache purge confirmation
fly logs
```

You should see a message like:
```
✓ Cloudflare cache purged successfully
```

## Troubleshooting

### Cache Not Purging

**Check if secrets are set:**
```bash
fly secrets list
```

You should see `CLOUDFLARE_ZONE_ID` and `CLOUDFLARE_API_TOKEN`.

**Check deployment logs:**
```bash
fly logs --app lcc-live-dark-paper-70
```

Look for messages from the purge-cache.sh script.

**Verify API token permissions:**
- Token must have "Cache Purge" permission
- Token must apply to the correct zone

### Release Command Failing

The release command is configured to not fail the deployment even if cache purging fails. This prevents deployment issues due to Cloudflare API problems.

If you see cache purge failures in logs:
1. Verify your Cloudflare credentials are correct
2. Check that your API token hasn't expired
3. Verify the zone ID matches your domain

## Alternative: Manual Cache Purge

If you need to manually purge the cache:

```bash
# Using curl directly
curl -X POST "https://api.cloudflare.com/client/v4/zones/${CLOUDFLARE_ZONE_ID}/purge_cache" \
  -H "Authorization: Bearer ${CLOUDFLARE_API_TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"purge_everything":true}'

# Or run the script locally
export CLOUDFLARE_ZONE_ID=your-zone-id
export CLOUDFLARE_API_TOKEN=your-token
./purge-cache.sh
```

## Selective Cache Purging (Advanced)

The current setup purges all cache. For more granular control, you can modify `purge-cache.sh` to purge specific files or tags:

### Purge specific URLs:
```json
{
  "files": [
    "https://lcc.live/",
    "https://lcc.live/canyon/index.html"
  ]
}
```

### Purge by cache tag:
```json
{
  "tags": ["version-123"]
}
```

See [Cloudflare API documentation](https://developers.cloudflare.com/api/operations/zone-purge) for more options.

## Files Modified

- `fly.toml`: Added `release_command` to run cache purge
- `Dockerfile`: Added curl and purge-cache.sh script
- `purge-cache.sh`: Script that calls Cloudflare API
- `doc/CACHE_AND_VERSION.md`: Documents the overall caching strategy

## Related Documentation

- [CACHE_AND_VERSION.md](CACHE_AND_VERSION.md) - Cache control headers and version tracking
- [Fly.io Release Commands](https://fly.io/docs/reference/configuration/#run-one-off-commands-before-releasing-a-deployment)
- [Cloudflare Cache Purge API](https://developers.cloudflare.com/api/operations/zone-purge)

