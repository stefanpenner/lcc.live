#!/bin/bash
# Generate og-image.png from favicon.svg for social media sharing
# Usage: generate-share-image.sh <svg_file> <output_file>
# Tools (rsvg-convert and magick) are provided via data dependencies

set -euo pipefail

# PATH should be set by the caller (genrule or sh_binary with data deps)
# If running directly, ensure tools are in PATH
if ! command -v rsvg-convert >/dev/null 2>&1 || ! command -v magick >/dev/null 2>&1; then
    # Fallback: try runfiles if available
    if [ -d "$0.runfiles" ]; then
        export PATH="$0.runfiles/tools_rsvg_convert:$0.runfiles/tools_magick:$PATH"
    fi
fi

if [ $# -lt 2 ]; then
    echo "Usage: $0 <svg_file> <output_file>"
    exit 1
fi

SVG_FILE="$1"
OUTPUT_FILE="$2"
TEMP_FILE=$(mktemp)

# Convert SVG to PNG at high resolution (1200x800 for Open Graph)
# First render at 2x for better quality, then resize
rsvg-convert -w 2400 -h 1600 --background-color=white "$SVG_FILE" -o "$TEMP_FILE"

# Resize to final dimensions (1200x800) with high quality
magick "$TEMP_FILE" \
    -resize 1200x800 \
    -gravity center \
    -background white \
    -extent 1200x800 \
    -quality 95 \
    "$OUTPUT_FILE"

rm -f "$TEMP_FILE"

echo "Share image generated successfully at $OUTPUT_FILE"


