#!/bin/bash
# Generate favicon.png from favicon.svg
# Usage: generate-favicon.sh <svg_file> <output_file>
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

# Convert SVG to PNG with transparent background
"$RSVG_CONVERT" -w 1024 -h 1024 --background-color=none "$SVG_FILE" -o "$TEMP_FILE"

# Ensure the base image has an alpha channel and transparency is preserved
"$MAGICK" "$TEMP_FILE" -alpha on -background none "$TEMP_FILE"

cp "$TEMP_FILE" "$OUTPUT_FILE"
rm -f "$TEMP_FILE"

echo "Favicon generated successfully at $OUTPUT_FILE"
