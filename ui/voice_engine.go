package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/opus"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/webrtc/v4"
)

type VoiceCallState struct {
	IsActive   bool
	IsMuted    bool
	PeerCount  int
	Error      error
	StatusText string
}

type VoiceEngine struct {
	boardID   string
	apiClient *VoiceClient
	peerID    string

	audioStream mediadevices.MediaStream
	audioTrack  webrtc.TrackLocal
	audioPlayer *AudioPlayer

	peers      map[string]*webrtc.PeerConnection
	peersMutex sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	StateChan  chan VoiceCallState
	state      VoiceCallState
	stateMutex sync.RWMutex
}

func NewVoiceEngine(boardID string, apiClient *VoiceClient) *VoiceEngine {
	player, _ := NewAudioPlayer()
	ctx, cancel := context.WithCancel(context.Background())
	return &VoiceEngine{
		boardID:     boardID,
		apiClient:   apiClient,
		peerID:      uuid.New().String(),
		audioPlayer: player,
		peers:       make(map[string]*webrtc.PeerConnection),
		ctx:         ctx,
		cancel:      cancel,
		StateChan:   make(chan VoiceCallState, 10),
		state: VoiceCallState{
			IsActive:   false,
			IsMuted:    false,
			PeerCount:  1,
			StatusText: "Initializing...",
		},
	}
}

func (ve *VoiceEngine) emitState() {
	ve.stateMutex.RLock()
	s := ve.state
	ve.stateMutex.RUnlock()

	select {
	case ve.StateChan <- s:
	default:
	}
}

func (ve *VoiceEngine) updateState(updateFn func(*VoiceCallState)) {
	ve.stateMutex.Lock()
	updateFn(&ve.state)
	ve.stateMutex.Unlock()
	ve.emitState()
}

func (ve *VoiceEngine) Start() error {
	ve.updateState(func(s *VoiceCallState) {
		s.IsActive = true
	})

	// 1. Initialize Audio Device
	opusParams, err := opus.NewParams()
	if err != nil {
		errRet := fmt.Errorf("failed to create opus params: %w", err)
		ve.updateState(func(s *VoiceCallState) {
			s.Error = errRet
		})
		return errRet
	}

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithAudioEncoders(&opusParams),
	)

	ve.audioStream, err = mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Audio: func(c *mediadevices.MediaTrackConstraints) {},
		Codec: codecSelector,
	})

	if err != nil {
		errRet := fmt.Errorf("failed to get user media: %w", err)
		ve.updateState(func(s *VoiceCallState) {
			s.Error = errRet
		})
		return errRet
	}

	ve.updateState(func(s *VoiceCallState) {
		s.StatusText = "Connecting..."
	})

	audioTracks := ve.audioStream.GetAudioTracks()
	if len(audioTracks) == 0 {
		errRet := fmt.Errorf("no audio tracks found")
		ve.updateState(func(s *VoiceCallState) {
			s.Error = errRet
		})
		return errRet
	}

	ve.audioTrack = audioTracks[0].(webrtc.TrackLocal)

	// 2. Join the voice call via API
	err = ve.apiClient.JoinVoiceCall(ve.boardID, ve.peerID)
	if err != nil {
		errRet := fmt.Errorf("failed to join call via API: %w", err)
		ve.updateState(func(s *VoiceCallState) {
			s.Error = errRet
		})
		return errRet
	}

	ve.updateState(func(s *VoiceCallState) {
		s.StatusText = "Connected"
	})

	// 3. Start signaling loops
	go ve.pollPeers()
	go ve.pollSignals()

	return nil
}

func (ve *VoiceEngine) ToggleMute() {
	ve.updateState(func(s *VoiceCallState) {
		s.IsMuted = !s.IsMuted
	})
	// We'd typically disable the track here, but pion/mediadevices track.On/Off isn't straight forward
	// Just updating state for now.
}

func (ve *VoiceEngine) Stop() {
	ve.cancel()

	if ve.audioPlayer != nil {
		ve.audioPlayer.Close()
	}

	if ve.audioStream != nil {
		for _, t := range ve.audioStream.GetTracks() {
			t.Close()
		}
	}

	ve.peersMutex.Lock()
	for _, pc := range ve.peers {
		pc.Close()
	}
	ve.peers = make(map[string]*webrtc.PeerConnection)
	ve.peersMutex.Unlock()

	_ = ve.apiClient.LeaveVoiceCall(ve.boardID)

	ve.updateState(func(s *VoiceCallState) {
		s.IsActive = false
		s.StatusText = "Disconnected"
	})
}

func (ve *VoiceEngine) pollPeers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ve.ctx.Done():
			return
		case <-ticker.C:
			peers, err := ve.apiClient.GetActivePeers(ve.boardID)
			if err != nil {
				continue // Ignore errors during polling
			}

			ve.updateState(func(s *VoiceCallState) {
				s.PeerCount = len(peers) + 1 // +1 for self
			})

			ve.peersMutex.Lock()
			for _, p := range peers {
				if p == ve.peerID {
					continue
				}
				if _, exists := ve.peers[p]; !exists {
					// Need to create an offer for this new peer
					go ve.createPeerConnection(p, true)
				}
			}
			ve.peersMutex.Unlock()
		}
	}
}

func (ve *VoiceEngine) pollSignals() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ve.ctx.Done():
			return
		case <-ticker.C:
			signals, err := ve.apiClient.GetSignals(ve.boardID)
			if err != nil {
				continue
			}

			for _, sig := range signals {
				ve.handleSignal(sig)
			}
		}
	}
}

func (ve *VoiceEngine) createPeerConnection(peerID string, isInitiator bool) *webrtc.PeerConnection {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil
	}

	// Add local audio track
	if ve.audioTrack != nil {
		_, err = pc.AddTrack(ve.audioTrack)
		if err != nil {
			fmt.Printf("Error adding track: %v\n", err)
		}
	}

	// Handle ICE candidates
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		cJSON, _ := json.Marshal(c.ToJSON())
		_ = ve.apiClient.SendSignal(ve.boardID, peerID, SignalPayload{
			Type: "candidate",
			Data: string(cJSON),
		})
	})

	// Handle incoming tracks (playback)
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if ve.audioPlayer != nil {
			err := ve.audioPlayer.PlayTrack(track)
			if err != nil {
				fmt.Printf("Error playing track: %v\n", err)
			}
		} else {
			go func() {
				buf := make([]byte, 1500)
				for {
					if _, _, err := track.Read(buf); err != nil {
						return
					}
				}
			}()
		}
	})

	ve.peersMutex.Lock()
	ve.peers[peerID] = pc
	ve.peersMutex.Unlock()

	if isInitiator {
		offer, err := pc.CreateOffer(nil)
		if err == nil {
			err = pc.SetLocalDescription(offer)
			if err == nil {
				oJSON, _ := json.Marshal(offer)
				_ = ve.apiClient.SendSignal(ve.boardID, peerID, SignalPayload{
					Type: "offer",
					Data: string(oJSON),
				})
			}
		}
	}

	return pc
}

func (ve *VoiceEngine) handleSignal(sig PeerSignal) {
	ve.peersMutex.Lock()
	pc, exists := ve.peers[sig.PeerID]
	ve.peersMutex.Unlock()

	if !exists && sig.Signal.Type == "offer" {
		pc = ve.createPeerConnection(sig.PeerID, false)
	}

	if pc == nil {
		return
	}

	switch sig.Signal.Type {
	case "offer":
		var sd webrtc.SessionDescription
		_ = json.Unmarshal([]byte(sig.Signal.Data), &sd)
		_ = pc.SetRemoteDescription(sd)
		answer, err := pc.CreateAnswer(nil)
		if err == nil {
			_ = pc.SetLocalDescription(answer)
			aJSON, _ := json.Marshal(answer)
			_ = ve.apiClient.SendSignal(ve.boardID, sig.PeerID, SignalPayload{
				Type: "answer",
				Data: string(aJSON),
			})
		}
	case "answer":
		var sd webrtc.SessionDescription
		_ = json.Unmarshal([]byte(sig.Signal.Data), &sd)
		_ = pc.SetRemoteDescription(sd)
	case "candidate":
		var candidate webrtc.ICECandidateInit
		_ = json.Unmarshal([]byte(sig.Signal.Data), &candidate)
		_ = pc.AddICECandidate(candidate)
	}
}
