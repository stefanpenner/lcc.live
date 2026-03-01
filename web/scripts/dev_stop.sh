#!/bin/bash
# Stop the development server using PID file

set -e

PID_FILE="/tmp/lcc-live-dev.pid"

if [[ -f "$PID_FILE" ]]; then
    pid=$(cat "$PID_FILE")
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
        echo "🛑 Stopping development server (PID: $pid)..."
        kill "$pid" 2>/dev/null || true
        
        # Wait for graceful shutdown
        count=0
        while kill -0 "$pid" 2>/dev/null && [[ $count -lt 10 ]]; do
            sleep 0.1
            ((count++))
        done
        
        # Force kill if still running
        if kill -0 "$pid" 2>/dev/null; then
            echo "🛑 Force stopping server (PID: $pid)..."
            kill -9 "$pid" 2>/dev/null || true
        fi
        
        echo "✅ Development server stopped"
    else
        echo "❌ Development server not running or PID file invalid"
    fi
    rm -f "$PID_FILE"
else
    echo "❌ No PID file found - development server may not be running"
fi
