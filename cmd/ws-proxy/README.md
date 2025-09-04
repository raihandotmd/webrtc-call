# WebSocket Proxy Server

This WebSocket proxy server forwards connections to your backend server at `ws://localhost:8082/communication/v1/ws` while adding the `X-User-Id` header for user identification.

## Features

- **Header Injection**: Automatically adds `X-User-Id` header when connecting to the backend server
- **Bi-directional Proxy**: Forwards messages in both directions between client and backend
- **User ID Extraction**: Supports getting user ID from query parameters or headers
- **Health Check**: Provides a health endpoint at `/health`
- **CORS Support**: Enabled for cross-origin requests

## Usage

### Starting the Proxy Server

```bash
cd cmd/ws-proxy
go run main.go
# or
./ws-proxy
```

The proxy server will start on port `:8081` by default.

### Connecting from Client

Clients can connect to the proxy using either method:

1. **Query Parameter** (recommended):
   ```javascript
   const ws = new WebSocket('ws://localhost:8081/ws?userId=user123');
   ```

2. **Header** (for environments that support custom headers):
   ```javascript
   // This would need to be set at the HTTP upgrade level
   // Most browsers don't support custom headers for WebSocket connections
   ```

### Backend Connection

The proxy will automatically:
1. Extract the `userId` from the client connection
2. Open a connection to `ws://localhost:8082/communication/v1/ws`
3. Add the `X-User-Id: user123` header to the backend connection
4. Forward all messages bidirectionally

## Configuration

You can modify these constants in `main.go`:

- `BackendWSURL`: The backend WebSocket endpoint (default: `ws://localhost:8082/communication/v1/ws`)
- `ProxyPort`: The port the proxy listens on (default: `:8081`)

## API Endpoints

- `GET /ws?userId=<user_id>` - WebSocket endpoint for clients
- `GET /health` - Health check endpoint

## Error Handling

The proxy handles various error scenarios:
- Missing user ID (returns 400 error)
- Backend connection failures (closes client connection with error message)
- Connection drops (automatically closes both connections)
- Unexpected close errors (logged but handled gracefully)

## Example Client Code

```javascript
const userId = 'user123';
const ws = new WebSocket(`ws://localhost:8081/ws?userId=${userId}`);

ws.onopen = function(event) {
    console.log('Connected to proxy server');
    // Send your WebRTC signaling messages here
    ws.send(JSON.stringify({
        type: 'offer',
        to: 'user456',
        from: userId,
        data: { /* SDP offer */ }
    }));
};

ws.onmessage = function(event) {
    const message = JSON.parse(event.data);
    console.log('Received from backend:', message);
};

ws.onerror = function(error) {
    console.error('WebSocket error:', error);
};

ws.onclose = function(event) {
    console.log('Connection closed:', event.code, event.reason);
};
```

## Logs

The proxy provides detailed logging:
- Connection establishment and closure
- User ID extraction
- Backend connection status
- Error conditions
- Message forwarding (can be enabled for debugging)

## Development

To modify the proxy behavior:

1. Edit `main.go`
2. Rebuild: `go build -o ws-proxy main.go`
3. Restart the proxy

For development with auto-reload, you can use:
```bash
go run main.go
```
