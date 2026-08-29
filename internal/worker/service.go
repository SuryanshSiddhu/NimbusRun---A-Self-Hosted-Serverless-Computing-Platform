package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Service is the long-running worker process that polls for jobs and executes them.
type Service struct {
	runner       *Runner
	redisClient  *redis.Client
	workerID     string
	queueName    string
	consumerName string
	pollInterval time.Duration
}

// NewService creates a new worker Service.
func NewService(runner *Runner, redisClient *redis.Client, workerID, queueName, consumerName string, pollInterval time.Duration) *Service {
	return &Service{
		runner:       runner,
		redisClient:  redisClient,
		workerID:     workerID,
		queueName:    queueName,
		consumerName: consumerName,
		pollInterval: pollInterval,
	}
}

// Job represents a work item from the queue.
type Job struct {
	InvocationID       string                 `json:"invocation_id"`
	FunctionID         string                 `json:"function_id"`
	VersionID          string                 `json:"version_id"`
	ImageURI           string                 `json:"image_uri"`
	Entrypoint         string                 `json:"entrypoint"`
	Payload            map[string]interface{} `json:"payload"`
	MemoryLimitMB      int                    `json:"memory_limit_mb"`
	CPULimitMillicores int                    `json:"cpu_limit_millicores"`
	TimeoutSeconds     int                    `json:"timeout_seconds"`
	RetryCount         int                    `json:"retry_count"`
	IdempotencyKey     string                 `json:"idempotency_key,omitempty"`
}

// Start begins the worker's job processing loop.
func (s *Service) Start(ctx context.Context) error {
	log.Printf("Worker %s started, listening on %s", s.workerID, s.queueName)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.processNextJob(ctx); err != nil {
				log.Printf("Worker %s: error processing job: %v", s.workerID, err)
			}
		}
	}
}

func (s *Service) processNextJob(ctx context.Context) error {
	streams, err := s.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    s.workerID,
		Consumer: s.consumerName,
		Streams:  []string{s.queueName, ">"},
		Count:    1,
		Block:    5 * time.Second,
	}).Result()

	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading from stream: %w", err)
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			var job Job
			if err := json.Unmarshal([]byte(msg.Values["data"].(string)), &job); err != nil {
				log.Printf("Failed to parse job: %v", err)
				s.ack(ctx, msg.ID)
				continue
			}

			result := s.executeJob(ctx, &job)

			if err := s.publishResult(ctx, &job, result); err != nil {
				log.Printf("Failed to publish result: %v", err)
			}

			s.ack(ctx, msg.ID)
		}
	}
	return nil
}

type JobResult struct {
	ExitCode int
	Duration time.Duration
	Stdout   string
	Stderr   string
}

func (s *Service) executeJob(ctx context.Context, job *Job) JobResult {
	runReq := &RunRequest{
		InvocationID:       uuid.New(),
		ImageURI:           job.ImageURI,
		Entrypoint:         job.Entrypoint,
		Payload:            job.Payload,
		MemoryLimitMB:      job.MemoryLimitMB,
		CPULimitMillicores: job.CPULimitMillicores,
		TimeoutSeconds:     job.TimeoutSeconds,
		ContainerName:      fmt.Sprintf("nimbus-%s", uuid.New().String()[:8]),
	}

	result := s.runner.Run(ctx, runReq)
	return JobResult{
		ExitCode: result.ExitCode,
		Duration: result.Duration,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}
}

func (s *Service) publishResult(ctx context.Context, job *Job, result JobResult) error {
	res := map[string]interface{}{
		"invocation_id": job.InvocationID,
		"function_id":   job.FunctionID,
		"version_id":    job.VersionID,
		"status":        "SUCCESS",
		"exit_code":     result.ExitCode,
		"duration_ms":   result.Duration.Milliseconds(),
		"stdout":        result.Stdout,
		"stderr":        result.Stderr,
		"worker_id":     s.workerID,
		"timestamp":     time.Now().UnixMilli(),
	}

	data, _ := json.Marshal(res)
	return s.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "nimbusrun:results",
		Values: map[string]interface{}{"data": string(data)},
	}).Err()
}

func (s *Service) ack(ctx context.Context, msgID string) {
	s.redisClient.XAck(ctx, s.queueName, s.workerID, msgID).Err()
}

// HeartbeatInfo is the worker status information sent to the control plane.
type HeartbeatInfo struct {
	WorkerID       string  `json:"worker_id"`
	Hostname       string  `json:"hostname"`
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    float64 `json:"memory_usage"`
	RunningTasks   int     `json:"running_tasks"`
	AvailableSlots int     `json:"available_slots"`
	Timestamp      int64   `json:"timestamp"`
}

// SendHeartbeat publishes worker health to the control plane.
func (s *Service) SendHeartbeat(ctx context.Context, info HeartbeatInfo) error {
	data, _ := json.Marshal(info)
	return s.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "nimbusrun:heartbeats",
		Values: map[string]interface{}{"data": string(data)},
	}).Err()
}
