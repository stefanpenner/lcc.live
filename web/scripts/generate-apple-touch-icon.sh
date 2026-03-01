#!/bin/bash
# Generate apple-touch-icon.png from app icon PNG
# Usage: generate-apple-touch-icon.sh <png_file> <output_file>

set -euo pipefail

if [ $# -lt 2 ]; then
    echo "Usage: $0 <png_file> <output_file>"
    exit 1
fi

PNG_FILE="$1"
OUTPUT_FILE="$2"

# Apply rounded-rect mask to make white corners transparent,
# then resize to 180x180 (Apple Touch Icon standard size).
# iOS will apply its own mask on top, but this avoids white corner artifacts.
SIZE=1024
RADIUS=224

magick "$PNG_FILE" \
    \( -size ${SIZE}x${SIZE} xc:none -fill white -draw "roundrectangle 0,0,$((SIZE-1)),$((SIZE-1)),$RADIUS,$RADIUS" \) \
    -alpha off -compose CopyOpacity -composite \
    -resize 180x180 \
    "$OUTPUT_FILE"

echo "Apple Touch Icon generated successfully at $OUTPUT_FILE"
