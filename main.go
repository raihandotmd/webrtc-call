package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pion/webrtc/v4"
)

var (
	pcA *webrtc.PeerConnection
	pcB *webrtc.PeerConnection

	// Pending candidates if peer not created yet
	pendingCandidatesA []webrtc.ICECandidateInit
	pendingCandidatesB []webrtc.ICECandidateInit

	localTrackToA *webrtc.TrackLocalStaticRTP
	localTrackToB *webrtc.TrackLocalStaticRTP

	// Store candidates to send to the other peer
	candidatesForA []map[string]interface{}
	candidatesForB []map[string]interface{}
)

// CORS middleware
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*") // allow all for testing
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func main() {
	// Single MediaEngine for registering codecs (use default codecs including opus)
	m := webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(&m))

	mux := http.NewServeMux()

	// OFFER A (client A -> server)
	mux.HandleFunc("/offer/a", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			return
		}

		var offerData struct {
			Type string `json:"type"`
			SDP  string `json:"sdp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&offerData); err != nil {
			http.Error(w, "Failed to parse offer JSON", 400)
			return
		}

		offer := offerData.SDP
		fmt.Println("[A] Received offer SDP from Client A")

		config := webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
		}

		pc, err := api.NewPeerConnection(config)
		if err != nil {
			http.Error(w, "Failed to create PeerConnection", 500)
			return
		}
		pcA = pc

		// Create a placeholder local track that server will write into to send audio -> A.
		// Use direct Opus capability
		opusCap := webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		}
		localTrackToA, _ = webrtc.NewTrackLocalStaticRTP(opusCap, "audio", "server-to-a")
		if _, err := pcA.AddTrack(localTrackToA); err != nil {
			fmt.Println("[A] Error adding pre-created localTrackToA:", err)
		} else {
			fmt.Println("[A] Added placeholder localTrackToA before answer")
		}

		pc.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c != nil {
				fmt.Println("[A] Generated ICE candidate:", c.String())
			}
		})
		pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			fmt.Println("[A] Connection state:", state.String())
		})

		pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			fmt.Println("[A] Got remote track from Client A; codec:", track.Codec().MimeType)
			// When A sends audio, forward to B by writing into localTrackToB (pre-created on pcB)
			if localTrackToB != nil {
				fmt.Println("[A] Forwarding RTP from A -> localTrackToB")
				rtpBuf := make([]byte, 1500)
				for {
					n, _, err := track.Read(rtpBuf)
					if err != nil {
						fmt.Println("[A] track.Read error:", err)
						return
					}
					if _, err := localTrackToB.Write(rtpBuf[:n]); err != nil {
						fmt.Println("[A] write to localTrackToB error:", err)
						return
					}
				}
			} else {
				fmt.Println("[A] localTrackToB is nil (pcB may not be created yet).")
			}
		})

		if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
			fmt.Println("[A] Error set remote desc:", err)
			http.Error(w, "Failed to set remote description", 400)
			return
		}

		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			fmt.Println("[A] CreateAnswer error:", err)
			http.Error(w, "Failed to create answer", 500)
			return
		}
		if err := pc.SetLocalDescription(answer); err != nil {
			fmt.Println("[A] SetLocalDescription error:", err)
			http.Error(w, "Failed to set local description", 500)
			return
		}

		// Add pending candidates if any
		for _, c := range pendingCandidatesA {
			if err := pcA.AddICECandidate(c); err != nil {
				fmt.Println("[A] AddICECandidate pending error:", err)
			}
		}
		pendingCandidatesA = nil

		fmt.Println("[A] Sending answer back to A")

		w.Header().Set("Content-Type", "application/json")
		answerResponse := map[string]string{
			"type": "answer",
			"sdp":  answer.SDP,
		}
		json.NewEncoder(w).Encode(answerResponse)
	})

	// OFFER B (client B -> server)
	mux.HandleFunc("/offer/b", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			return
		}

		var offerData struct {
			Type string `json:"type"`
			SDP  string `json:"sdp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&offerData); err != nil {
			http.Error(w, "Failed to parse offer JSON", 400)
			return
		}

		offer := offerData.SDP
		fmt.Println("[B] Received offer SDP from Client B")

		config := webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
		}

		pc, err := api.NewPeerConnection(config)
		if err != nil {
			http.Error(w, "Failed to create PeerConnection", 500)
			return
		}
		pcB = pc

		// Create placeholder local track server -> B (so B will have an incoming track in SDP)
		opusCap := webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		}
		localTrackToB, _ = webrtc.NewTrackLocalStaticRTP(opusCap, "audio", "server-to-b")
		if _, err := pcB.AddTrack(localTrackToB); err != nil {
			fmt.Println("[B] Error adding pre-created localTrackToB:", err)
		} else {
			fmt.Println("[B] Added placeholder localTrackToB before answer")
		}

		pc.OnICECandidate(func(c *webrtc.ICECandidate) {
			if c != nil {
				fmt.Println("[B] Generated ICE candidate:", c.String())
			}
		})
		pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			fmt.Println("[B] Connection state:", state.String())
		})

		pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			fmt.Println("[B] Got remote track from Client B; codec:", track.Codec().MimeType)
			// When B sends audio, forward to A by writing into localTrackToA (pre-created on pcA)
			if localTrackToA != nil {
				fmt.Println("[B] Forwarding RTP from B -> localTrackToA")
				rtpBuf := make([]byte, 1500)
				for {
					n, _, err := track.Read(rtpBuf)
					if err != nil {
						fmt.Println("[B] track.Read error:", err)
						return
					}
					if _, err := localTrackToA.Write(rtpBuf[:n]); err != nil {
						fmt.Println("[B] write to localTrackToA error:", err)
						return
					}
				}
			} else {
				fmt.Println("[B] localTrackToA is nil (pcA may not be created yet).")
			}
		})

		if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
			fmt.Println("[B] Error set remote desc:", err)
			http.Error(w, "Failed to set remote description", 400)
			return
		}

		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			fmt.Println("[B] CreateAnswer error:", err)
			http.Error(w, "Failed to create answer", 500)
			return
		}
		if err := pc.SetLocalDescription(answer); err != nil {
			fmt.Println("[B] SetLocalDescription error:", err)
			http.Error(w, "Failed to set local description", 500)
			return
		}

		// Add pending candidates if any
		for _, c := range pendingCandidatesB {
			if err := pcB.AddICECandidate(c); err != nil {
				fmt.Println("[B] AddICECandidate pending error:", err)
			}
		}
		pendingCandidatesB = nil

		fmt.Println("[B] Sending answer back to B")

		w.Header().Set("Content-Type", "application/json")
		answerResponse := map[string]string{
			"type": "answer",
			"sdp":  answer.SDP,
		}
		json.NewEncoder(w).Encode(answerResponse)
	})

	mux.HandleFunc("/candidate/a", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			return
		}

		if r.Method == http.MethodGet {
			// Client A is asking for candidates from B
			w.Header().Set("Content-Type", "application/json")
			if candidatesForA == nil {
				candidatesForA = []map[string]interface{}{}
			}
			json.NewEncoder(w).Encode(candidatesForA)
			candidatesForA = nil // Clear after sending
			return
		}

		if r.Method == http.MethodPost {
			// Client A is sending its candidates (to be forwarded to B)
			var candidateData map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&candidateData); err != nil {
				http.Error(w, "invalid candidate", http.StatusBadRequest)
				return
			}

			// Convert to webrtc.ICECandidateInit for local processing
			ice := webrtc.ICECandidateInit{}
			if candidate, ok := candidateData["candidate"].(string); ok {
				ice.Candidate = candidate
			}
			if sdpMid, ok := candidateData["sdpMid"].(string); ok {
				ice.SDPMid = &sdpMid
			}
			if sdpMLineIndex, ok := candidateData["sdpMLineIndex"].(float64); ok {
				index := uint16(sdpMLineIndex)
				ice.SDPMLineIndex = &index
			}

			if pcA != nil {
				if err := pcA.AddICECandidate(ice); err != nil {
					fmt.Println("[A] AddICECandidate error:", err)
				}
			} else {
				pendingCandidatesA = append(pendingCandidatesA, ice)
			}

			// Store the original candidate data to send to B
			candidatesForB = append(candidatesForB, candidateData)
			w.WriteHeader(http.StatusOK)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/candidate/b", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			return
		}

		if r.Method == http.MethodGet {
			// Client B is asking for candidates from A
			w.Header().Set("Content-Type", "application/json")
			if candidatesForB == nil {
				candidatesForB = []map[string]interface{}{}
			}
			json.NewEncoder(w).Encode(candidatesForB)
			candidatesForB = nil // Clear after sending
			return
		}

		if r.Method == http.MethodPost {
			// Client B is sending its candidates (to be forwarded to A)
			var candidateData map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&candidateData); err != nil {
				http.Error(w, "invalid candidate", http.StatusBadRequest)
				return
			}

			// Convert to webrtc.ICECandidateInit for local processing
			ice := webrtc.ICECandidateInit{}
			if candidate, ok := candidateData["candidate"].(string); ok {
				ice.Candidate = candidate
			}
			if sdpMid, ok := candidateData["sdpMid"].(string); ok {
				ice.SDPMid = &sdpMid
			}
			if sdpMLineIndex, ok := candidateData["sdpMLineIndex"].(float64); ok {
				index := uint16(sdpMLineIndex)
				ice.SDPMLineIndex = &index
			}

			if pcB != nil {
				if err := pcB.AddICECandidate(ice); err != nil {
					fmt.Println("[B] AddICECandidate error:", err)
				}
			} else {
				pendingCandidatesB = append(pendingCandidatesB, ice)
			}

			// Store the original candidate data to send to A
			candidatesForA = append(candidatesForA, candidateData)
			w.WriteHeader(http.StatusOK)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	fmt.Println("Server started at :8080")
	http.ListenAndServe(":8080", mux)
}
