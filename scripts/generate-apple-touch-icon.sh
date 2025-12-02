#!/bin/bash
# Generate apple-touch-icon.png from SVG
# Usage: generate-apple-touch-icon.sh <svg_file> <output_file>
# Tools (rsvg-convert and magick) are provided via data dependencies

set -euo pipefail

# Add binaries to PATH from runfiles
export PATH="$0.runfiles/tools_rsvg_convert:$0.runfiles/tools_magick:$PATH"

if [ $# -lt 2 ]; then
    echo "Usage: $0 <svg_file> <output_file>"
    exit 1
fi

SVG_FILE="$1"
OUTPUT_FILE="$2"
TEMP_FILE=$(mktemp)

# Convert SVG to PNG at 180x180 (Apple Touch Icon standard size)
# Use white background for iOS (Apple Touch Icons should not be transparent)
rsvg-convert -w 180 -h 180 --background-color=white "$SVG_FILE" -o "$TEMP_FILE"

# Ensure solid background and convert to PNG
magick "$TEMP_FILE" \
    -background white \
    -alpha remove \
    -alpha off \
    "$OUTPUT_FILE"

rm -f "$TEMP_FILE"

echo "Apple Touch Icon generated successfully at $OUTPUT_FILE"



