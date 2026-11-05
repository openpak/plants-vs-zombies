package main

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// memBackend stands in for Postgres in handler tests; production only uses pgBackend.
type memBackend struct {
	mu   sync.Mutex
	next int64
	by   map[string]*account
}

func newMemBackend() *memBackend { return &memBackend{next: 1000000001, by: map[string]*account{}} }

func (m *memBackend) login(_ context.Context, id string, pid uint64, name string) (account, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.by[id]
	if !ok {
		a = &account{BaasUserID: id, BlazeID: m.next, Created: time.Now().UTC()}
		m.next++
		m.by[id] = a
		a.Name = "Player" + strconv.FormatInt(a.BlazeID%100000, 10)
	}
	a.PID = pid
	if name != "" {
		a.Name = name
	}
	a.LastLogin = time.Now().UTC()
	return *a, !ok, nil
}

func (m *memBackend) saveData(_ context.Context, id string, data map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a := m.by[id]; a != nil {
		a.Data = data
	}
	return nil
}

func (m *memBackend) byBlaze(_ context.Context, id int64) (*account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.by {
		if a.BlazeID == id {
			c := *a
			return &c, nil
		}
	}
	return nil, nil
}

func (m *memBackend) erase(_ context.Context, baas []string, pids []int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, a := range m.by {
		hit := false
		for _, b := range baas {
			hit = hit || b == id
		}
		for _, p := range pids {
			hit = hit || int64(a.PID) == p
		}
		if hit {
			delete(m.by, id)
			n++
		}
	}
	return n, nil
}
