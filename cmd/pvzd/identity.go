package main

// Who the console is, asked of nx-baas (POST /internal/switch/identity with the BAAS id_token the
// console presents), the same contract the other OpenPak game services use.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var httpc = &http.Client{Timeout: 5 * time.Second}

type player struct {
	PID        uint64 `json:"pid"`
	BaasUserID string `json:"baas_user_id"`
	Nickname   string `json:"nickname"`
}

var (
	errTokenRejected = errors.New("token rejected")
	errAccountBanned = errors.New("account banned")
)

type identityClient struct{ url, key string }

func newIdentityClient(url, key string) identityClient { return identityClient{url: url, key: key} }

func (c identityClient) byIDToken(tok string) (*player, error) {
	body, _ := json.Marshal(map[string]any{"id_token": tok})
	req, err := http.NewRequest(http.MethodPost, c.url+"/internal/switch/identity", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", c.key)
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("nx-baas: %s: %w", resp.Status, errTokenRejected)
	case http.StatusForbidden:
		return nil, fmt.Errorf("nx-baas: %s: %w", resp.Status, errAccountBanned)
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("nx-baas identity: %s", resp.Status)
	}
	var p player
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	if p.BaasUserID == "" {
		return nil, fmt.Errorf("nx-baas identity: incomplete reply")
	}
	return &p, nil
}
