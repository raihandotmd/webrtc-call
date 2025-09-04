#!/bin/bash

echo "🚀 Starting WebSocket Proxy with Debug Logging..."
echo "📝 This will show all WebSocket messages passing through the proxy"
echo ""
echo "Expected log format:"
echo "  👤 User connection info"
echo "  📋 Headers being sent to backend"
echo "  ➡️ Client → Backend messages"
echo "  ⬅️ Backend → Client messages"
echo "  🔌 Connection close events"
echo ""
echo "Press Ctrl+C to stop"
echo "==========================================="

cd "$(dirname "$0")"
./ws-proxy
