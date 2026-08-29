package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/queue"
	"github.com/redis/go-redis/v9"
)

// Scheduler assigns jobs to workers based on resource availability.
type Scheduler struct {
	db          *db.DB
	redis       *redis.Client
	jobQueue    *queue.RedisStreamQueue
	workerStore *WorkerHeartbeatStore
	SelectFunc  func([]*queue.WorkerHeartbeat) *queue.WorkerHeartbeat
	mu          sync.RWMutex
	running     bool
	workers     map[string]*queue.WorkerHeartbeat
}

// NewScheduler creates a new job scheduler.
func NewScheduler(database *db.DB, redis *redis.Client, jobQueue *queue.RedisStreamQueue) *Scheduler {
	s := &Scheduler{
		db:          database,
		redis:       redis,
		jobQueue:    jobQueue,
		workerStore: NewWorkerHeartbeatStore(),
		SelectFunc:  selectLowestLoadWorker,
		workers:     make(map[string]*queue.WorkerHeartbeat),
	}
	return s
}

// Start begins the scheduler's main loop.
func (s *Scheduler) Start(ctx context.Context) error {
	log.Println("Starting NimbusRun scheduler...")
	s.running = true

	if err := s.jobQueue.RegisterConsumerGroup(ctx, "scheduler"); err != nil {
		return fmt.Errorf("registering scheduler consumer group: %w", err)
	}

	go s.processHeartbeats(ctx)
	go s.dispatchJobs(ctx)

	return nil
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	s.running = false
}

// processHeartbeats listens for worker heartbeats and updates worker store.
func (s *Scheduler) processHeartbeats(ctx context.Context) {
	if err := s.jobQueue.RegisterConsumerGroup(ctx, "scheduler-heartbeats"); err != nil {
		log.Printf("Failed to create heartbeat consumer group: %v", err)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			streams, err := s.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    "scheduler-heartbeats",
				Consumer: "scheduler",
				Streams:  []string{"nimbusrun:heartbeats", ">"},
				Count:    100,
				Block:    1 * time.Second,
			}).Result()

			if err == redis.Nil {
				continue
			}
			if err != nil {
				log.Printf("Error reading heartbeats: %v", err)
				continue
			}

			s.mu.Lock()
			for _, stream := range streams {
				for _, msg := range stream.Messages {
					dataStr, ok := msg.Values["data"].(string)
					if !ok {
						continue
					}
					var hb queue.WorkerHeartbeat
					if err := json.Unmarshal([]byte(dataStr), &hb); err != nil {
						log.Printf("Failed to parse heartbeat: %v", err)
						continue
					}
					if err := hb.Validate(); err != nil {
						log.Printf("Invalid heartbeat from %s: %v", hb.WorkerID, err)
						continue
					}

					s.workers[hb.WorkerID] = &hb
					s.redis.XAck(ctx, "nimbusrun:heartbeats", "scheduler-heartbeats", msg.ID)
				}
			}
			s.mu.Unlock()
		}
	}
}

// dispatchJobs continuously pulls jobs from queue and assigns them to workers.
func (s *Scheduler) dispatchJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := s.dispatchOneJob(ctx); err != nil {
				log.Printf("Error dispatching job: %v", err)
				time.Sleep(100 * time.Millisecond)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// dispatchOneJob processes a single job from the queue.
func (s *Scheduler) dispatchOneJob(ctx context.Context) error {
	jobs, err := s.jobQueue.DequeueJobs(ctx, "scheduler", "dispatcher-1", 1)
	if err == queue.ErrNoJobs {
		return nil
	}
	if err != nil {
		return fmt.Errorf("dequeuing job: %w", err)
	}
	if len(jobs) == 0 {
		return nil
	}

	queuedJob := jobs[0]
	worker := s.selectWorker()
	if worker == nil {
		s.jobQueue.EnqueueJob(ctx, queuedJob.Job)
		log.Println("No healthy workers available, requeuing job")
		time.Sleep(1 * time.Second)
		return nil
	}

	workerStream := fmt.Sprintf("nimbusrun:worker:%s", worker.WorkerID)
	if err := s.jobQueue.EnqueueJobToStream(ctx, workerStream, queuedJob.Job); err != nil {
		log.Printf("Failed to assign job to worker %s: %v", worker.WorkerID, err)
		s.jobQueue.EnqueueJob(ctx, queuedJob.Job)
		return err
	}

	log.Printf("Dispatched job %s to worker %s (load: %.2f)", queuedJob.Job.ID, worker.WorkerID, worker.CalculateLoad())
	s.jobQueue.AckJob(ctx, "scheduler", queuedJob.ID)

	return nil
}

// selectWorker chooses the best available worker using the selection policy.
func (s *Scheduler) selectWorker() *queue.WorkerHeartbeat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var candidates []*queue.WorkerHeartbeat
	for _, w := range s.workers {
		if w.IsHealthy(now) {
			candidates = append(candidates, w)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return s.SelectFunc(candidates)
}

// selectLowestLoadWorker returns the worker with the lowest normalized load score.
func selectLowestLoadWorker(workers []*queue.WorkerHeartbeat) *queue.WorkerHeartbeat {
	if len(workers) == 0 {
		return nil
	}
	best := workers[0]
	bestLoad := best.CalculateLoad()
	for _, w := range workers[1:] {
		load := w.CalculateLoad()
		if load < bestLoad {
			bestLoad = load
			best = w
		}
	}
	return best
}

// GetWorkerStats returns current worker statistics.
func (s *Scheduler) GetWorkerStats() []*queue.WorkerHeartbeat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var healthy []*queue.WorkerHeartbeat
	for _, w := range s.workers {
		if w.IsHealthy(now) {
			healthy = append(healthy, w)
		}
	}
	return healthy
}

// SelectByInvocation assigns a job to a worker for an invocation.
func (s *Scheduler) SelectByInvocation(ctx context.Context, invID uuid.UUID) (*queue.WorkerHeartbeat, error) {
	return s.selectWorker(), nil
}
