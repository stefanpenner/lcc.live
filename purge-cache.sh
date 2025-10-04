#!/bin/sh
# Purge Cloudflare cache on deploy
set -e

# These should be set as Fly.io secrets
# fly secrets set CLOUDFLARE_ZONE_ID=your-zone-id
# fly secrets set CLOUDFLARE_API_TOKEN=your-api-token

if [ -z "$CLOUDFLARE_ZONE_ID" ] || [ -z "$CLOUDFLARE_API_TOKEN" ]; then
  echo "Warning: CLOUDFLARE_ZONE_ID or CLOUDFLARE_API_TOKEN not set. Skipping cache purge."
  exit 0
fi

echo "Purging Cloudflare cache for zone: $CLOUDFLARE_ZONE_ID"

# Purge everything - use this for simplicity
# For more granular control, you can purge specific URLs or tags
response=$(curl -s -X POST \
  "https://api.cloudflare.com/client/v4/zones/${CLOUDFLARE_ZONE_ID}/purge_cache" \
  -H "Authorization: Bearer ${CLOUDFLARE_API_TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"purge_everything":true}')

# Check if successful
if echo "$response" | grep -q '"success":true'; then
  echo "✓ Cloudflare cache purged successfully"
  exit 0
else
  echo "✗ Failed to purge Cloudflare cache:"
  echo "$response"
  # Don't fail the deploy if cache purge fails
  exit 0
fi

