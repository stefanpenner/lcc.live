#!/bin/bash
# Generate og-image.png from app icon PNG for social media sharing
# Usage: generate-share-image.sh <png_file> <output_file>

set -euo pipefail

if [ $# -lt 2 ]; then
    echo "Usage: $0 <png_file> <output_file>"
    exit 1
fi

PNG_FILE="$1"
OUTPUT_FILE="$2"

# Apply rounded-rect mask, then center on a dark background for OG image
SIZE=1024
RADIUS=224

magick -size 1200x630 xc:"#1a2332" \
    \( "$PNG_FILE" \
        \( -size ${SIZE}x${SIZE} xc:none -fill white -draw "roundrectangle 0,0,$((SIZE-1)),$((SIZE-1)),$RADIUS,$RADIUS" \) \
        -alpha off -compose CopyOpacity -composite \
        -resize 400x400 \
    \) \
    -gravity center -composite \
    -quality 95 \
    "$OUTPUT_FILE"

echo "Share image generated successfully at $OUTPUT_FILE"
