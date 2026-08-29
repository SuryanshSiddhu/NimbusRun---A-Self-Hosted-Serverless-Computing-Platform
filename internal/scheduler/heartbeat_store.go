package scheduler

import (
	"sync"
	"time"

	"github.com/nimbusrun/nimbusrun/internal/queue"
)

// WorkerHeartbeatStore manages the in-memory state of active workers.
type WorkerHeartbeatStore struct {
	workers map[string]*queue.WorkerHeartbeat
	mu      sync.RWMutex
	ttl     time.Duration
}

// NewWorkerHeartbeatStore creates a new WorkerHeartbeatStore.
func NewWorkerHeartbeatStore() *WorkerHeartbeatStore {
	return &WorkerHeartbeatStore{
		workers: make(map[string]*queue.WorkerHeartbeat),
		ttl:     30 * time.Second,
	}
}

// Update records a new heartbeat for a worker.
func (s *WorkerHeartbeatStore) Update(hb *queue.WorkerHeartbeat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[hb.WorkerID] = hb
}

// Get returns the heartbeat for a worker.
func (s *WorkerHeartbeatStore) Get(workerID string) *queue.WorkerHeartbeat {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workers[workerID]
}

// List returns all workers.
func (s *WorkerHeartbeatStore) List() []*queue.WorkerHeartbeat {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*queue.WorkerHeartbeat, 0, len(s.workers))
	for _, w := range s.workers {
		result = append(result, w)
	}
	return result
}

// ListHealthy returns workers with recent heartbeats.
func (s *WorkerHeartbeatStore) ListHealthy() []*queue.WorkerHeartbeat {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	result := make([]*queue.WorkerHeartbeat, 0)
	for _, w := range s.workers {
		if w.IsHealthy(now) {
			result = append(result, w)
		}
	}
	return result
}

// Remove marks a worker as unhealthy and removes it from the store.
func (s *WorkerHeartbeatStore) Remove(workerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workers, workerID)
}

// CleanStale removes workers with stale heartbeats.
func (s *WorkerHeartbeatStore) CleanStale() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	removed := 0
	for id, w := range s.workers {
		if !w.IsHealthy(now) {
			delete(s.workers, id)
			removed++
		}
	}
	return removed
}

// Count returns the number of tracked workers.
func (s *WorkerHeartbeatStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.workers)
}

// CountHealthy returns the number of workers with recent heartbeats.
func (s *WorkerHeartbeatStore) CountHealthy() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	count := 0
	for _, w := range s.workers {
		if w.IsHealthy(now) {
			count++
		}
	}
	return count
}
