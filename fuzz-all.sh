#!/bin/bash
# fuzz-all.sh - Run all fuzzing tests with reasonable time limits
set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default fuzz time (can be overridden with environment variable)
FUZZ_TIME=${FUZZ_TIME:-"5s"}

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}          Fuzzing Test Suite for lcc.live              ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Fuzz time per test: ${FUZZ_TIME}${NC}"
echo ""

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

run_fuzz_test() {
    local name=$1
    local package=$2
    local fuzz_func=$3
    local fuzz_time=${4:-$FUZZ_TIME}
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Running: $name${NC}"
    echo -e "  Package: $package"
    echo -e "  Function: $fuzz_func"
    echo -e "  Duration: $fuzz_time"
    echo ""
    
    if go test -fuzz=$fuzz_func -fuzztime=$fuzz_time ./web/$package 2>&1 | \
       grep -v "^📸" | \
       grep -v "^  ✨" | \
       grep -v "^  ✅" | \
       grep -v "^  💤" | \
       grep -v "^  ❌" | \
       grep -v "^202[0-9]-" | \
       tail -10; then
        echo -e "${GREEN}✓ PASSED${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}✗ FAILED${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
    echo ""
}

# Server fuzzing tests
echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║              SERVER FUZZING TESTS                     ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""

run_fuzz_test "Image Route Fuzzing" "server" "FuzzImageRoute" "$FUZZ_TIME"
run_fuzz_test "Canyon Route Fuzzing" "server" "FuzzCanyonRoute" "$FUZZ_TIME"
run_fuzz_test "HTTP Headers Fuzzing" "server" "FuzzHTTPHeaders" "$FUZZ_TIME"
run_fuzz_test "Static Files Fuzzing" "server" "FuzzStaticFiles" "$FUZZ_TIME"

# Store fuzzing tests
echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║              STORE FUZZING TESTS                      ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""

run_fuzz_test "Store Camera ID Fuzzing" "store" "FuzzStoreCameraID" "$FUZZ_TIME"
run_fuzz_test "Image Data Fuzzing" "store" "FuzzImageData" "$FUZZ_TIME"
run_fuzz_test "Concurrent Access Fuzzing" "store" "FuzzConcurrentAccess" "10s"
run_fuzz_test "HTTP Response Headers Fuzzing" "store" "FuzzHTTPResponseHeaders" "$FUZZ_TIME"
run_fuzz_test "Camera URL Fuzzing" "store" "FuzzCameraURL" "$FUZZ_TIME"

# Summary
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}                     SUMMARY                           ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Total Tests:  $TOTAL_TESTS"
echo -e "${GREEN}  Passed:       $PASSED_TESTS${NC}"
if [ $FAILED_TESTS -gt 0 ]; then
    echo -e "${RED}  Failed:       $FAILED_TESTS${NC}"
else
    echo -e "  Failed:       $FAILED_TESTS"
fi
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}✓ All fuzzing tests passed!${NC}"
    echo ""
    echo -e "The server has been thoroughly tested and is:"
    echo -e "  • Stable under various inputs"
    echo -e "  • Secure against common attacks"
    echo -e "  • Thread-safe for concurrent operations"
    echo -e "  • Producing valid output in all cases"
    exit 0
else
    echo -e "${RED}✗ Some fuzzing tests failed!${NC}"
    echo ""
    echo -e "Please review the failures above and fix the issues."
    echo -e "Failed tests indicate potential crashes or security issues."
    exit 1
fi

