# WebRTC P2P Calling System - Mermaid Diagrams

## 1. System Architecture Overview

```mermaid
graph TB
    subgraph "Client Side"
        A[Client A Browser<br/>callerA.html]
        B[Client B Browser<br/>callerB.html]
    end
    
    subgraph "GoFiber Server"
        S[GoFiber HTTP Server<br/>:8080]
        WS[WebSocket Handler<br/>/ws?id=clientId]
        H[Hub - Client Manager<br/>Register/Unregister]
        R[Message Router<br/>Forward signaling]
        ICE[ICE Servers Endpoint<br/>/ice-servers]
        SF[Static Files<br/>HTML serving]
    end
    
    subgraph "External Services"
        STUN[Google STUN Server<br/>stun.l.google.com:19302]
    end
    
    A -.->|WebSocket Signaling| WS
    B -.->|WebSocket Signaling| WS
    WS --> H
    H --> R
    
    A -->|HTTP Request| ICE
    B -->|HTTP Request| ICE
    
    A -->|HTTP Request| SF
    B -->|HTTP Request| SF
    
    A -.->|STUN Requests| STUN
    B -.->|STUN Requests| STUN
    
    A ==>|Direct P2P RTP Audio| B
```

## **2. Complete Call Flow Sequence**

```mermaid
sequenceDiagram
    participant CA as Client A
    participant S as GoFiber Server
    participant CB as Client B
    participant STUN as STUN Server

    Note over CA, CB: Phase 1: Connection Setup
    CA->>S: WebSocket Connect (/ws?id=a)
    S->>S: Register Client A
    CB->>S: WebSocket Connect (/ws?id=b)
    S->>S: Register Client B

    Note over CA, CB: Phase 2: Call Initiation
    CA->>S: call-request → Client B
    S->>CB: Forward call-request from A
    CB->>CB: Show incoming call UI

    alt Call Accepted
        CB->>S: call-accept → Client A
        S->>CA: Forward call-accept from B
        CB->>CB: Get microphone access
    else Call Rejected
        CB->>S: call-reject → Client A
        S->>CA: Forward call-reject from B
        CA->>CA: Reset call UI
        Note over CA, CB: Call ends here if rejected
    end

    Note over CA, CB: Phase 3: WebRTC Setup
    CA->>S: GET /ice-servers
    S-->>CA: [STUN server config]
    CB->>S: GET /ice-servers
    S-->>CB: [STUN server config]

    par ICE Discovery
        CA->>STUN: Discover public IP
        STUN-->>CA: Public IP: 203.0.113.45
    and
        CB->>STUN: Discover public IP
        STUN-->>CB: Public IP: 198.51.100.33
    end

    Note over CA, CB: Phase 4: SDP Exchange
    CA->>CA: Create RTCPeerConnection
    CA->>CA: Create SDP Offer
    CA->>S: offer → Client B
    S->>CB: Forward SDP offer from A

    CB->>CB: Create RTCPeerConnection
    CB->>CB: Set remote description (offer)
    CB->>CB: Create SDP Answer
    CB->>S: answer → Client A
    S->>CA: Forward SDP answer from B
    CA->>CA: Set remote description (answer)

    Note over CA, CB: Phase 5: ICE Candidate Exchange
    par ICE Candidates Exchange
        CA->>S: candidate → Client B
        S->>CB: Forward ICE candidate from A
        CB->>CB: Add ICE candidate
    and
        CB->>S: candidate → Client A
        S->>CA: Forward ICE candidate from B
        CA->>CA: Add ICE candidate
    end

    Note over CA, CB: Phase 6: Direct P2P Connection
    CA->>CB: ICE connectivity checks
    CB->>CA: ICE connectivity checks
    CA->>CB: DTLS handshake (encryption)
    CB->>CA: DTLS handshake (encryption)
    CA->>CB: Direct RTP Audio Stream
    CB->>CA: Direct RTP Audio Stream

    Note over CA, CB: Phase 7: Active Call
    CA->>CB: Audio Stream
    CB->>CA: Audio Stream
    Note over S: Server no longer involved in media

    Note over CA, CB: Phase 8: Call Termination
    alt Hang up by Client A
        CA->>S: hangup → Client B
        S->>CB: Forward hangup from A
        CB->>CB: Close connection & cleanup
    else Hang up by Client B
        CB->>S: hangup → Client A
        S->>CA: Forward hangup from B
        CA->>CA: Close connection & cleanup
    end

    CA->>CA: Reset UI
    CB->>CB: Reset UI
```

## **3. WebSocket Message Flow**

```mermaid
graph TD
    subgraph "Message Types"
        CR[call-request<br/>Initiate call]
        CA[call-accept<br/>Accept call]
        CRJ[call-reject<br/>Reject call]
        O[offer<br/>SDP offer]
        A[answer<br/>SDP answer]
        IC[candidate<br/>ICE candidate]
        H[hangup<br/>End call]
    end

    subgraph "Client A Flow"
        A1[Click Call] --> A2[Send call-request]
        A3[Receive call-accept] --> A4[Send offer]
        A5[Receive answer] --> A6[Send candidates]
        A7[Click Hangup] --> A8[Send hangup]
    end

    subgraph "Client B Flow"
        B1[Receive call-request] --> B2[Show Accept/Reject]
        B3[Click Accept] --> B4[Send call-accept]
        B5[Receive offer] --> B6[Send answer]
        B7[Send candidates] --> B8[P2P Connected]
        B9[Receive hangup] --> B10[Cleanup]
    end

    subgraph "Server Hub"
        S1[Receive Message] --> S2[Set msg.From = clientID]
        S2 --> S3[Route to msg.To client]
        S3 --> S4[Forward via WebSocket]
    end

    A2 --> S1
    A4 --> S1
    A8 --> S1
    S4 --> B1
    S4 --> B5
    S4 --> B9

    B4 --> S1
    B6 --> S1
    S4 --> A3
    S4 --> A5

```

## **4. WebRTC Peer Connection State Machine**

```mermaid
stateDiagram-v2
    [*] --> Disconnected

    Disconnected --> Connecting : createPeerConnection()
    Connecting --> Connected : ICE Success
    Connecting --> Failed : ICE Failed
    Connecting --> Disconnected : Connection timeout

    Connected --> HaveLocalOffer : createOffer()
    HaveLocalOffer --> HaveRemoteAnswer : setRemoteDescription(answer)
    HaveRemoteAnswer --> Stable : ICE complete

    Connected --> HaveRemoteOffer : setRemoteDescription(offer)
    HaveRemoteOffer --> HaveLocalAnswer : createAnswer()
    HaveLocalAnswer --> Stable : ICE complete

    Stable --> MediaFlow : ontrack event
    MediaFlow --> AudioPlaying : Audio streams active

    AudioPlaying --> Closing : hangup()
    Closing --> Closed : cleanup complete
    Closed --> [*]

    Failed --> [*]
    Disconnected --> [*]

```

## **5. Server Architecture Components**

```mermaid
graph TB
    subgraph GoFiber_Application
        App["fiber.New()"]
        CORS["CORS Middleware"]
        Routes["HTTP Routes"]
    end

    subgraph WebSocket_Layer
        WSMiddleware["WebSocket Upgrade Middleware"]
        WSHandler["WebSocket Connection Handler"]
    end

    subgraph Client_Management
        Hub["Hub Struct"]
        Clients["map[string]*Client"]
        Register["Register Channel"]
        Unregister["Unregister Channel"]
    end

    subgraph Message_Processing
        Reader["Message Reader Goroutine"]
        Router["Message Router"]
        Sender["Message Sender"]
    end

    subgraph HTTP_Endpoints
        Static["Static File Server\n/callerA.html, /callerB.html"]
        ICEEndpoint["ICE Servers\n/ice-servers"]
        WSEndpoint["WebSocket\n/ws"]
    end

    App --> CORS
    CORS --> Routes
    Routes --> Static
    Routes --> ICEEndpoint
    Routes --> WSMiddleware

    WSMiddleware --> WSHandler
    WSHandler --> Hub
    Hub --> Clients
    Hub --> Register
    Hub --> Unregister

    WSHandler --> Reader
    Reader --> Router
    Router --> Sender
    Sender --> Clients
```

## **6. Client-Side Component Flow**

```mermaid
graph TD
    subgraph HTML_Interface
        UI["User Interface"]
        ConnectBtn["Connect Button"]
        CallBtn["Call Button"]
        AcceptBtn["Accept Button"]
        RejectBtn["Reject Button"]
        HangupBtn["Hangup Button"]
        Status["Status Display"]
        Audio["Audio Element"]
    end

    subgraph JavaScript_Core
        WS["WebSocket Connection"]
        PC["RTCPeerConnection"]
        Stream["MediaStream"]
        Handlers["Event Handlers"]
    end

    subgraph WebRTC_APIs
        GetUserMedia["getUserMedia()"]
        CreateOffer["createOffer()"]
        CreateAnswer["createAnswer()"]
        AddTrack["addTrack()"]
        OnTrack["ontrack event"]
        OnICE["onicecandidate"]
    end

    subgraph Network_Layer
        ICEServers["ICE Servers Config"]
        STUNQuery["STUN Queries"]
        P2PMedia["P2P Media Stream"]
    end

    UI --> Handlers
    ConnectBtn --> WS
    CallBtn --> Handlers
    AcceptBtn --> GetUserMedia
    GetUserMedia --> Stream
    Stream --> AddTrack
    AddTrack --> PC
    PC --> CreateOffer
    PC --> CreateAnswer
    PC --> OnICE
    OnTrack --> Audio

    WS --> Handlers
    Handlers --> PC
    PC --> ICEServers
    ICEServers --> STUNQuery
    STUNQuery --> P2PMedia
    P2PMedia --> OnTrack

```

## **7. Data Flow Architecture**

```mermaid
flowchart LR
    subgraph "Client A"
        A1[HTML UI] --> A2[JavaScript]
        A2 --> A3[WebSocket]
        A2 --> A4[WebRTC PC]
        A5[Microphone] --> A4
        A4 --> A6[Speakers]
    end

    subgraph "Signaling Server"
        S1[GoFiber Router] --> S2[WebSocket Handler]
        S2 --> S3[Hub Manager]
        S3 --> S4[Message Router]
    end

    subgraph "Client B"
        B1[HTML UI] --> B2[JavaScript]
        B2 --> B3[WebSocket]
        B2 --> B4[WebRTC PC]
        B5[Microphone] --> B4
        B4 --> B6[Speakers]
    end

    subgraph "Internet"
        I1[STUN Server]
        I2[Direct P2P Path]
    end

    A3 -.->|Signaling| S2
    S4 -.->|Signaling| B3

    A4 -->|STUN Query| I1
    B4 -->|STUN Query| I1

    A4 ==>|RTP Audio| I2
    I2 ==>|RTP Audio| B4
```

## **8. Error Handling Flow**

```mermaid
graph TD
    subgraph "Connection Errors"
        CE1[WebSocket Connection Failed] --> CE2[Show Connection Error]
        CE3[WebSocket Disconnected] --> CE4[Disable Call Features]
        CE5[Server Unreachable] --> CE6[Retry Connection]
    end

    subgraph "Call Errors"
        CAE1[Call Rejected] --> CAE2[Reset UI State]
        CAE3[Media Access Denied] --> CAE4[Show Microphone Error]
        CAE5[Remote Hangup] --> CAE6[Cleanup Connection]
    end

    subgraph "WebRTC Errors"
        WE1[ICE Connection Failed] --> WE2[Show Connection Failed]
        WE3[Offer/Answer Failed] --> WE4[Reset Call State]
        WE5[STUN Server Unreachable] --> WE6[Try Fallback Methods]
    end

    subgraph "Recovery Actions"
        RA1[Reset UI] --> RA2[Close Connections]
        RA2 --> RA3[Clear Media Streams]
        RA3 --> RA4[Enable Retry]
    end

    CE2 --> RA1
    CAE2 --> RA1
    WE2 --> RA1
    CAE6 --> RA1
```