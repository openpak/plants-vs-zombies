package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// account is one OpenPak player as this game knows them. BlazeID doubles as the persona id and
// the Nucleus account id: the game only needs them stable and unique.
type account struct {
	BaasUserID string            `json:"baas_user_id"`
	PID        uint64            `json:"pid"`
	BlazeID    int64             `json:"blaze_id"`
	Name       string            `json:"name"`
	Created    time.Time         `json:"created"`
	LastLogin  time.Time         `json:"last_login"`
	Data       map[string]string `json:"data,omitempty"` // per-game blobs (ByteVault, settings), by key
}

// store keeps accounts in one JSON file. A Blaze login is rare and the data is small.
type store struct {
	mu   sync.Mutex
	path string
	Next int64               `json:"next"`
	By   map[string]*account `json:"accounts"` // by baas_user_id
}

func openStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &store{path: filepath.Join(dir, "accounts.json"), Next: 1000000001, By: map[string]*account{}}
	b, err := os.ReadFile(s.path)
	if err == nil {
		if err := json.Unmarshal(b, s); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *store) saveLocked() error {
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	tmp := s.path + ".new"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// get returns (creating if needed) the account for an OpenPak player.
func (s *store) get(baasID string, pid uint64, name string) (*account, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.By[baasID]
	first := !ok
	if !ok {
		a = &account{BaasUserID: baasID, BlazeID: s.Next, Created: time.Now().UTC()}
		s.Next++
		s.By[baasID] = a
	}
	a.PID = pid
	if name != "" {
		a.Name = name
	}
	if a.Name == "" {
		a.Name = "Player" + itoa(a.BlazeID%100000)
	}
	a.LastLogin = time.Now().UTC()
	return a, first, s.saveLocked()
}

// update runs fn on an account under the lock and saves.
func (s *store) update(a *account, fn func(a *account)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(a)
	return s.saveLocked()
}

func (s *store) byBlazeID(id int64) *account {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.By {
		if a.BlazeID == id {
			return a
		}
	}
	return nil
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
