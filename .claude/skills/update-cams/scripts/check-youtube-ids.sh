#!/usr/bin/env bash
# Check all YouTube embed IDs in data.json for liveness.
# Uses the YouTube oEmbed API — returns HTTP 200 for valid/public videos, 4xx for dead ones.
#
# Output: one line per camera: STATUS ALT_NAME YOUTUBE_ID
#   STATUS is ALIVE or DEAD

set -euo pipefail

DATA_FILE="${1:-data.json}"

if [[ ! -f "$DATA_FILE" ]]; then
  echo "ERROR: $DATA_FILE not found" >&2
  exit 1
fi

# Extract iframe cameras: pairs of (alt, youtube_id)
# Uses grep + sed to avoid jq dependency
# In data.json, "alt" appears on the line after "src", so use -A1
paste -d'|' \
  <(grep -A1 'youtube\.com/embed/' "$DATA_FILE" | grep '"alt"' | sed 's/.*"alt": *"//;s/".*//' ) \
  <(grep 'youtube\.com/embed/' "$DATA_FILE" | sed 's|.*youtube\.com/embed/||;s|[?"&].*||')  |
while IFS='|' read -r alt ytid; do
  # Query oEmbed API
  http_code=$(curl -s -o /dev/null -w '%{http_code}' \
    "https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=${ytid}&format=json" \
    2>/dev/null || echo "000")

  if [[ "$http_code" == "200" ]]; then
    echo "ALIVE  ${alt}  ${ytid}"
  else
    echo "DEAD   ${alt}  ${ytid}  (HTTP ${http_code})"
  fi
done
