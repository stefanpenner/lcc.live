#!/usr/bin/env bash
set -euo pipefail

# Deploy LCC.live iOS app to TestFlight
# Replaces: bundle exec fastlane beta

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✅${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}❌${NC} $1"
}

# Configuration
SCHEME="lcc"
PROJECT="lcc.xcodeproj"
BUILD_DIR="build"
ARCHIVE_PATH="${BUILD_DIR}/lcc.xcarchive"
IPA_DIR="${BUILD_DIR}"
EXPORT_OPTIONS="scripts/ExportOptions.plist"
DEFAULT_ISSUER_ID="69a6de74-20d1-47e3-e053-5b8c7c11a4d1"

# Ensure we're in the ios directory
if [ ! -d "$PROJECT" ]; then
    log_error "Must be run from the ios/ directory (cannot find ${PROJECT})"
    exit 1
fi

# Clean previous build artifacts
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# --- Step 1: Increment build number ---
BUILD_NUMBER="${GITHUB_RUN_NUMBER:-$(date +%s)}"
log_info "Setting build number to ${BUILD_NUMBER}"
agvtool new-version -all "$BUILD_NUMBER" > /dev/null
log_success "Build number set to ${BUILD_NUMBER}"

# --- Step 2: Build archive ---
log_info "Building archive..."
xcodebuild archive \
    -scheme "$SCHEME" \
    -project "$PROJECT" \
    -archivePath "$ARCHIVE_PATH" \
    -destination "generic/platform=iOS" \
    -allowProvisioningUpdates \
    COMPILER_INDEX_STORE_ENABLE=NO \
    | tail -1

if [ ! -d "$ARCHIVE_PATH" ]; then
    log_error "Archive failed — ${ARCHIVE_PATH} not found"
    exit 1
fi
log_success "Archive created at ${ARCHIVE_PATH}"

# --- Step 3: Export IPA ---
log_info "Exporting IPA..."
xcodebuild -exportArchive \
    -archivePath "$ARCHIVE_PATH" \
    -exportPath "$IPA_DIR" \
    -exportOptionsPlist "$EXPORT_OPTIONS" \
    -allowProvisioningUpdates \
    | tail -1

IPA_FILE=$(find "$IPA_DIR" -name "*.ipa" -maxdepth 1 | head -1)
if [ -z "$IPA_FILE" ]; then
    log_error "Export failed — no .ipa found in ${IPA_DIR}"
    exit 1
fi
log_success "IPA exported: ${IPA_FILE}"

# --- Step 4: Resolve App Store Connect API key ---
KEY_ID="${APP_STORE_CONNECT_API_KEY_ID:-}"
ISSUER_ID="${APP_STORE_CONNECT_API_ISSUER_ID:-$DEFAULT_ISSUER_ID}"
KEY_FILE=""

if [ -z "$KEY_ID" ]; then
    # Auto-detect from ~/.appstoreconnect/AuthKey_*.p8
    KEY_FILE=$(find ~/.appstoreconnect -name "AuthKey_*.p8" 2>/dev/null | head -1 || true)
    if [ -z "$KEY_FILE" ]; then
        log_error "No App Store Connect API key found."
        log_error "Set APP_STORE_CONNECT_API_KEY_ID or place AuthKey_*.p8 in ~/.appstoreconnect/"
        exit 1
    fi
    KEY_ID=$(basename "$KEY_FILE" .p8 | sed 's/AuthKey_//')
    log_info "Using API key ${KEY_ID} from ${KEY_FILE}"
else
    # When KEY_ID is set via env, look for the matching .p8 file
    KEY_FILE=$(find ~/.appstoreconnect -name "AuthKey_${KEY_ID}.p8" 2>/dev/null | head -1 || true)
    if [ -z "$KEY_FILE" ]; then
        KEY_FILE="${HOME}/.appstoreconnect/AuthKey_${KEY_ID}.p8"
    fi
    log_info "Using API key ${KEY_ID} from environment"
fi

# --- Step 5: Upload to TestFlight ---
# altool searches for .p8 keys in CWD/private_keys, ~/private_keys,
# ~/.private_keys, or ~/.appstoreconnect/private_keys — not ~/.appstoreconnect/
# Stage the key where altool expects it.
PRIVATE_KEYS_DIR="private_keys"
mkdir -p "$PRIVATE_KEYS_DIR"
cleanup() { rm -rf "$PRIVATE_KEYS_DIR"; }
trap cleanup EXIT
ln -sf "$KEY_FILE" "${PRIVATE_KEYS_DIR}/AuthKey_${KEY_ID}.p8"

log_info "Uploading to TestFlight..."
xcrun altool --upload-app \
    -f "$IPA_FILE" \
    --type ios \
    --apiKey "$KEY_ID" \
    --apiIssuer "$ISSUER_ID"

log_success "Successfully uploaded to TestFlight!"
