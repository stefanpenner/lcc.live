#!/bin/bash
# Output runner for graceful server restarts during development
# This script is called by ibazel when files change

set -e

# PID file location
PID_FILE="/tmp/lcc-live-dev.pid"

# Function to kill existing server process using PID file
kill_existing_server() {
    if [[ -f "$PID_FILE" ]]; then
        local pid=$(cat "$PID_FILE")
        if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
            echo "🔄 Stopping existing server (PID: $pid)..."
            kill "$pid" 2>/dev/null || true
            # Wait for graceful shutdown
            local count=0
            while kill -0 "$pid" 2>/dev/null && [[ $count -lt 10 ]]; do
                sleep 0.1
                ((count++))
            done
            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                echo "🔄 Force stopping server (PID: $pid)..."
                kill -9 "$pid" 2>/dev/null || true
            fi
        fi
        rm -f "$PID_FILE"
    fi
}

# Function to cleanup on exit
cleanup() {
    echo "🛑 Cleaning up..."
    kill_existing_server
    rm -f "$PID_FILE"
    exit 0
}

# Set up signal handlers for cleanup
trap cleanup SIGINT SIGTERM EXIT

# Kill any existing server process
kill_existing_server

# Set development mode environment variable
export DEV_MODE=true

# Start the new server and capture its PID
"$@" &
SERVER_PID=$!

# Save PID to file
echo "$SERVER_PID" > "$PID_FILE"
echo "🚀 Started server (PID: $SERVER_PID)"

# Wait for the server process
wait $SERVER_PID
