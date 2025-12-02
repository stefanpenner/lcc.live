#!/bin/bash
# Generate favicon.png from favicon.svg
# Usage: generate-favicon.sh <svg_file> <output_file>
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

# Convert SVG to PNG with transparent background
rsvg-convert -w 1024 -h 1024 --background-color=none "$SVG_FILE" -o "$TEMP_FILE"

# Ensure the base image has an alpha channel and transparency is preserved
magick "$TEMP_FILE" -alpha on -background none "$TEMP_FILE"

cp "$TEMP_FILE" "$OUTPUT_FILE"
rm -f "$TEMP_FILE"

echo "Favicon generated successfully at $OUTPUT_FILE"
