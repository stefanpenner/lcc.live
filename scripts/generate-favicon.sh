#!/bin/bash
# Generate favicon.png from app icon PNG
# Usage: generate-favicon.sh <png_file> <output_file>

set -euo pipefail

if [ $# -lt 2 ]; then
    echo "Usage: $0 <png_file> <output_file>"
    exit 1
fi

PNG_FILE="$1"
OUTPUT_FILE="$2"

# The iOS app icon has white corners from its rounded-rect shape.
# Create a matching rounded-rect mask to make those corners transparent,
# then resize to 32x32 for the favicon.
SIZE=1024
RADIUS=224

magick "$PNG_FILE" \
    \( -size ${SIZE}x${SIZE} xc:none -fill white -draw "roundrectangle 0,0,$((SIZE-1)),$((SIZE-1)),$RADIUS,$RADIUS" \) \
    -alpha off -compose CopyOpacity -composite \
    -resize 32x32 \
    "$OUTPUT_FILE"

echo "Favicon generated successfully at $OUTPUT_FILE"
