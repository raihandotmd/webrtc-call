#!/bin/bash

# WebSocket Proxy Start Script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 Starting WebSocket Proxy Server..."
echo "📍 Working directory: $SCRIPT_DIR"
echo "🎯 Backend target: ws://localhost:8082/communication/v1/ws"
echo "🌐 Proxy listening on: http://localhost:8081"
echo ""
echo "📋 Available endpoints:"
echo "   • WebSocket: ws://localhost:8081/ws?userId=<your-user-id>"
echo "   • Health check: http://localhost:8081/health"
echo "   • Test client: Open test-client.html in your browser"
echo ""
echo "Press Ctrl+C to stop the server"
echo "----------------------------------------"

# Build if binary doesn't exist or source is newer
if [ ! -f "ws-proxy" ] || [ "main.go" -nt "ws-proxy" ]; then
    echo "🔨 Building proxy server..."
    go build -o ws-proxy main.go
    if [ $? -ne 0 ]; then
        echo "❌ Build failed!"
        exit 1
    fi
    echo "✅ Build successful!"
    echo ""
fi

# Start the proxy server
./ws-proxy
