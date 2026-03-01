#!/bin/bash
# Script to increment build number

set -e

PROJECT_FILE="lcc.xcodeproj/project.pbxproj"

# Get current build number
CURRENT_BUILD=$(/usr/libexec/PlistBuddy -c "Print :objects:E21BA5D62DC2C9F9008638D3:buildSettings:CURRENT_PROJECT_VERSION" "$PROJECT_FILE" 2>/dev/null || echo "1")

# Increment
NEW_BUILD=$((CURRENT_BUILD + 1))

echo "Incrementing build number from $CURRENT_BUILD to $NEW_BUILD"

# Update all configurations (Debug and Release)
sed -i '' "s/CURRENT_PROJECT_VERSION = ${CURRENT_BUILD};/CURRENT_PROJECT_VERSION = ${NEW_BUILD};/g" "$PROJECT_FILE"

echo "✅ Build number updated to $NEW_BUILD"

