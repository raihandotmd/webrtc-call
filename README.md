
# WebRTC Peer-to-Peer Audio Call

This project implements a **peer-to-peer WebRTC audio calling system** with a **GoFiber WebSocket-based signaling server**.

## Features & Architecture

- **Signaling**: GoFiber WebSocket server (`main.go`) for SDP/ICE exchange.
- **Media**: Direct P2P RTP audio between clients (no server relay).
- **NAT Traversal**: STUN (and optional TURN via Docker Compose) for connectivity.
- **Frontend**: Two HTML clients (`callerA.html`, `callerB.html`) and a diagnostics tool (`diagnostics.html`).

## Project Structure

- `main.go`: GoFiber server, WebSocket signaling, static file serving.
- `callerA.html`, `callerB.html`: WebRTC audio call clients.
- `diagnostics.html`: ICE connectivity diagnostics.
- `docker-compose.yml`: Coturn TURN server for NAT traversal.
- `credentials-example.go`: Example Go file (not used in main flow).
- `go.mod`, `go.sum`: Go dependencies.

## How to Run

1. **Install Go dependencies**
  ```bash
  go mod tidy
  ```
2. **Start the Go server**
  ```bash
  go run main.go
  ```
  - Server runs at `http://localhost:8080`
  - WebSocket endpoint: `ws://localhost:8080/ws?id=<client_id>`

3. **Open Clients**
  - Client A: `http://localhost:8080/callerA.html`
  - Client B: `http://localhost:8080/callerB.html`
  - Diagnostics: `http://localhost:8080/diagnostics.html`

4. **Make a Call**
  - Connect both clients to server, then call and accept/reject as desired.

## TURN Server (Optional)

- To enable TURN for restrictive NATs, run Coturn via Docker Compose:
  ```bash
  docker-compose up coturn
  ```
- TURN credentials are statically set in `main.go` and `docker-compose.yml` (`testuser:testpass`).

## Security & Production Notes

- ICE servers are provided via authenticated WebSocket (no HTTP endpoint).
- For production: use secure WebSocket (WSS), dynamic TURN credentials, and authentication.

## Extending

- The signaling server can be extended for user management, call history, notifications, etc.
