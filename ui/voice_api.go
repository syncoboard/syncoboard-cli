package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/syncoboard/syncoboard/sdks/go/api"
)

type SignalPayload struct {
	Type string `json:"type"` // "offer", "answer", "candidate"
	Data string `json:"data"` // JSON stringified SDP or ICE candidate
}

type PeerSignal struct {
	PeerID string        `json:"peerId"`
	Signal SignalPayload `json:"signal"`
}

type VoiceClient struct {
	client *api.Client
}

func NewVoiceClient(client *api.Client) *VoiceClient {
	return &VoiceClient{client: client}
}

// JoinVoiceCall registers the user to the board's voice call
func (v *VoiceClient) JoinVoiceCall(boardID string, peerID string) error {
	endpoint := fmt.Sprintf("/boards/%s/voice/join", boardID)
	body := map[string]string{"peerId": peerID}
	return v.doRequest("POST", endpoint, body, nil)
}

// LeaveVoiceCall leaves the board's voice call
func (v *VoiceClient) LeaveVoiceCall(boardID string) error {
	endpoint := fmt.Sprintf("/boards/%s/voice/leave", boardID)
	return v.doRequest("POST", endpoint, nil, nil)
}

// GetActivePeers gets other active peers in the board's voice call
func (v *VoiceClient) GetActivePeers(boardID string) ([]string, error) {
	endpoint := fmt.Sprintf("/boards/%s/voice/peers", boardID)
	var resp struct {
		Peers []string `json:"peers"`
	}
	err := v.doRequest("GET", endpoint, nil, &resp)
	return resp.Peers, err
}

// SendSignal sends a WebRTC signal to a specific peer
func (v *VoiceClient) SendSignal(boardID string, toPeerID string, payload SignalPayload) error {
	endpoint := fmt.Sprintf("/boards/%s/voice/signal/%s", boardID, toPeerID)
	return v.doRequest("POST", endpoint, payload, nil)
}

// GetSignals gets pending WebRTC signals for the current user
func (v *VoiceClient) GetSignals(boardID string) ([]PeerSignal, error) {
	endpoint := fmt.Sprintf("/boards/%s/voice/signals", boardID)
	var resp struct {
		Signals []PeerSignal `json:"signals"`
	}
	err := v.doRequest("GET", endpoint, nil, &resp)
	return resp.Signals, err
}

func (v *VoiceClient) doRequest(method, endpoint string, body interface{}, out interface{}) error {
	url := fmt.Sprintf("%s%s", v.client.BaseURL, endpoint)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if v.client.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", v.client.Token))
	}

	resp, err := v.client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status code %d, body: %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
