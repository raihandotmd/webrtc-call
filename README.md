# WebRTC Peer-to-Peer Audio Call

This project implements a **peer-to-peer WebRTC audio calling system** with a WebSocket-based signaling server.

## Architecture Overview

```
Client A ←─── Direct P2P Audio ────→ Client B
    ↓                                   ↓  
    └─── WebSocket Signaling ─────────→ Go Server
```

- **Signaling**: WebSocket messages for SDP offers/answers and ICE candidates
- **Media**: Direct peer-to-peer RTP audio streams (no server relay)
- **NAT Traversal**: STUN servers for discovering public IP addresses

## Key Changes from Previous Version

### Before (Server-Relayed):
- ❌ All audio went through Go server
- ❌ HTTP polling for ICE candidates
- ❌ Higher latency and server load

### After (P2P):
- ✅ Direct audio connection between clients
- ✅ WebSocket real-time signaling  
- ✅ Lower latency, better quality
- ✅ Scalable (server only handles signaling)

## How to Run

### 1. Start the Server
```bash
go run main.go
```
Server starts on: `http://localhost:8080`

### 2. Open Clients
- **Client A**: Open `http://localhost:8080/callerA.html`
- **Client B**: Open `http://localhost:8080/callerB.html` 

### 3. Make a Call
1. **Both clients**: Click "Connect to Server"
2. **Caller**: Click "Call Client A/B"  
3. **Receiver**: See incoming call notification with Accept/Reject buttons
4. **Receiver**: Click "Accept" to start the call or "Reject" to decline
5. **Start talking**: Direct P2P audio connection established!
6. **End call**: Either party can click "Hang Up" to end the call for both sides

## WebRTC Flow Explained

### 1. **Signaling Connection**
```javascript
// Client connects to WebSocket signaling server
ws = new WebSocket("ws://localhost:8080/ws?id=clientA");
```

### 2. **Call Request** 
```javascript
// Caller sends call request
ws.send({type: "call-request", to: "clientB"});
```

### 3. **Call Accept/Reject**
```javascript
// Receiver can accept or reject
ws.send({type: "call-accept", to: "clientA"});  // or "call-reject"
```

### 4. **WebRTC Negotiation** (Only after acceptance)
```javascript
// Caller creates offer and sends via WebSocket
const offer = await pc.createOffer();
ws.send({type: "offer", to: "clientB", data: offer});

// Callee receives offer, creates answer
await pc.setRemoteDescription(offer);
const answer = await pc.createAnswer();
ws.send({type: "answer", to: "clientA", data: answer});
```

### 5. **ICE Candidate Exchange**
```javascript
// Both clients exchange network connectivity info
pc.onicecandidate = (event) => {
  ws.send({type: "candidate", to: remoteId, data: event.candidate});
};
```

### 6. **Direct P2P Connection**
```
Client A ←──── Direct RTP Audio ────→ Client B
```

### 7. **Call Termination**
```javascript
// Either party can hang up, notifying the other
ws.send({type: "hangup", to: remoteId});
```

## Server Implementation

### WebSocket Signaling Hub
```go
type Hub struct {
    Clients    map[string]*Client  // Connected clients
    Register   chan *Client        // New client connections
    Unregister chan *Client        // Client disconnections
}
```

### Message Routing
```go
// Server simply routes messages between clients
type SignalingMessage struct {
    Type string      // "offer", "answer", "candidate"
    To   string      // Target client ID  
    From string      // Sender client ID
    Data interface{} // WebRTC data (SDP/ICE)
}
```

## Client Features

### UI Controls
- **Connect to Server**: Establish WebSocket connection
- **Call**: Send call request to remote client
- **Accept/Reject**: Handle incoming call requests with user choice
- **Hang Up**: End call and cleanup connections (notifies remote client)

### Incoming Call Experience
- **Visual notification**: Highlighted incoming call box with caller ID
- **User choice**: Accept or Reject buttons
- **No auto-answer**: User explicitly chooses to accept calls

### Real-time Status
- **Server connection status**: Connected/Disconnected from signaling server
- **Call state**: No Call → Requesting → Calling → Connected → Call Ended
- **Proper cleanup**: Both clients notified when call ends

## Benefits of P2P Approach

### **Performance**
- **Lower Latency**: Direct connection, no server relay
- **Better Quality**: No server-side audio processing/compression
- **Bandwidth Efficient**: Server only handles small signaling messages

### **Scalability** 
- **Server Load**: Minimal (only WebSocket messages)
- **Cost Effective**: No media bandwidth costs
- **Unlimited Concurrent Calls**: Server resources don't scale with calls

### **Privacy**
- **Audio Privacy**: Audio never passes through server
- **End-to-End**: Direct encrypted connection between clients

## Production Considerations

### **TURN Server Fallback**
For production, add TURN servers for clients behind restrictive NATs:

```javascript
const iceServers = [
  { urls: "stun:stun.l.google.com:19302" },
  { 
    urls: "turn:your-turn-server.com:3478",
    username: "user", 
    credential: "password" 
  }
];
```

### **Authentication & Security**
- Add user authentication to WebSocket connections
- Implement call authorization logic
- Use WSS (secure WebSocket) in production

### **Error Handling**
- Handle connection failures gracefully
- Implement automatic reconnection
- Add call timeout mechanisms

## Microservices Extension

This signaling server can be extended into a microservices architecture:

- **User Service**: Authentication, contacts, presence
- **Call Service**: Call routing, history, recording triggers  
- **Media Service**: TURN servers, media quality monitoring
- **Notification Service**: Call notifications, missed calls

The WebSocket signaling remains the same, with additional services handling business logic around the calls.
