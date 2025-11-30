#!/bin/bash
# Generate og-image.png from favicon.svg for social media sharing
# Usage: generate-share-image.sh <svg_file> <output_file>
# Tools (rsvg-convert and magick) are provided via data dependencies

set -euo pipefail

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail
set +e
f=bazel_tools/tools/bash/runfiles/runfiles.bash
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null ||
  source "$0.runfiles/$f" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null ||
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -d' ' -f2- -d' ')" 2>/dev/null ||
  {
    echo >&2 "ERROR: cannot find $f"
    exit 1
  }
f=
set -e
# --- end runfiles.bash initialization v3 ---

if [ $# -lt 2 ]; then
    echo "Usage: $0 <svg_file> <output_file>"
    exit 1
fi

SVG_FILE="$1"
OUTPUT_FILE="$2"
TEMP_FILE=$(mktemp)

# Use rlocation to find tools in runfiles
# Format: rlocation("repository_name/path/to/file")
# For external repos, use: rlocation("@repo_name//path/to/file")
RSVG_CONVERT=$(rlocation "+host_tools_ext+host_tools/rsvg_convert_bin")
MAGICK=$(rlocation "+host_tools_ext+host_tools/magick_bin")

# Validate tools exist and are executable
if [ ! -x "$RSVG_CONVERT" ]; then
    echo "Error: rsvg-convert not found or not executable: $RSVG_CONVERT" >&2
    exit 1
fi

if [ ! -x "$MAGICK" ]; then
    echo "Error: magick not found or not executable: $MAGICK" >&2
    exit 1
fi

# Convert SVG to PNG at high resolution (1200x800 for Open Graph)
# First render at 2x for better quality, then resize
"$RSVG_CONVERT" -w 2400 -h 1600 --background-color=white "$SVG_FILE" -o "$TEMP_FILE"

# Resize to final dimensions (1200x800) with high quality
"$MAGICK" "$TEMP_FILE" \
    -resize 1200x800 \
    -gravity center \
    -background white \
    -extent 1200x800 \
    -quality 95 \
    "$OUTPUT_FILE"

rm -f "$TEMP_FILE"

echo "Share image generated successfully at $OUTPUT_FILE"

