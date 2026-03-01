---
disable-model-invocation: true
description: Check YouTube webcam IDs in data.json and replace dead ones with current IDs from OnTheSnow
user-invocable: true
---

# Update Webcam YouTube IDs

YouTube live stream IDs go stale when ski resorts restart their streams. This skill checks all YouTube embed cameras in `data.json`, identifies dead ones, and finds replacements.

## Steps

### 1. Run the checker script

Run `bash .claude/skills/update-cams/scripts/check-youtube-ids.sh` to get a status report of all YouTube IDs. This outputs each camera's name, YouTube ID, and whether it's alive or dead.

### 2. Analyze results

If all cameras are alive, report that and stop.

If any cameras are dead, proceed to step 3.

### 3. Find replacement IDs

For each dead camera, determine which resort it belongs to based on the camera name:

- **Solitude** cameras: Honeycomb Canyon, Powderhorn, Sunshine Bowl, Solitude Snow Stake, Moonbeam Parking Lot, Moonbeam Express, Apex Express and Sunrise Lift Lines, POWDERHORN II LIFT LINE, Link Lift Line
- **Brighton** cameras: Bottom of Majestic Lift, Brighton Lot, Molly Greens, Crest 6

Scrape the appropriate OnTheSnow webcam page to find current YouTube IDs:

- Solitude: `https://www.onthesnow.com/utah/solitude-mountain-resort/webcams`
- Brighton: `https://www.onthesnow.com/utah/brighton-resort/webcams`
- Alta: `https://www.onthesnow.com/utah/alta-ski-area/webcams`
- Snowbird: `https://www.onthesnow.com/utah/snowbird/webcams`

Use `WebFetch` to retrieve each relevant page. Extract YouTube video IDs from the page content (look for `youtube.com/embed/` URLs or YouTube video ID patterns).

### 4. Match cameras to replacements

For each dead camera, try to match it with a replacement from OnTheSnow by camera name. OnTheSnow names may differ slightly from `data.json` names — use fuzzy matching (e.g., "Honeycomb" matches "Honeycomb Canyon").

If a match is found, note the new YouTube ID. If no match is found, flag it for manual review.

### 5. Update data.json

For each matched dead camera, update the `src` field in `data.json` with the new YouTube ID, preserving the URL format:
```
https://www.youtube.com/embed/{NEW_ID}?autoplay=1&mute=1&controls=0
```

Use the Edit tool to make precise replacements of each old URL with the new one.

### 6. Report

Summarize:
- Total YouTube cameras checked
- How many were alive vs dead
- Which dead cameras were updated (old ID -> new ID)
- Which dead cameras could not be matched (need manual intervention)
