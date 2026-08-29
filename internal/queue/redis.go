package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrNoJobs = errors.New("no jobs available")

// RedisStreamQueue manages job scheduling via Redis Streams.
type RedisStreamQueue struct {
	client       *redis.Client
	jobStream    string
	resultStream string
}

// NewRedisStreamQueue creates a new Redis-based job queue.
func NewRedisStreamQueue(client *redis.Client, jobStream, resultStream string) *RedisStreamQueue {
	return &RedisStreamQueue{
		client:       client,
		jobStream:    jobStream,
		resultStream: resultStream,
	}
}

// EnqueueJob adds a new job to the main job stream.
func (q *RedisStreamQueue) EnqueueJob(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshaling job: %w", err)
	}

	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.jobStream,
		ID:     "*",
		Values: map[string]interface{}{
			"job_id": job.ID.String(),
			"data":   string(data),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("xadd to stream: %w", err)
	}
	return nil
}

// EnqueueJobToStream adds a job to a specific stream (e.g., a worker's queue).
func (q *RedisStreamQueue) EnqueueJobToStream(ctx context.Context, stream string, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshaling job: %w", err)
	}

	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		ID:     "*",
		Values: map[string]interface{}{
			"job_id": uuid.New().String(),
			"data":   string(data),
		},
	}).Result()

	return err
}

// RegisterConsumerGroup creates a consumer group for the stream.
func (q *RedisStreamQueue) RegisterConsumerGroup(ctx context.Context, groupName string) error {
	err := q.client.XGroupCreateMkStream(ctx, q.jobStream, groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("creating consumer group: %w", err)
	}
	return nil
}

// DequeueJobs claims pending jobs for a consumer.
func (q *RedisStreamQueue) DequeueJobs(ctx context.Context, groupName, consumerName string, count int64) ([]*QueuedJob, error) {
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{q.jobStream, ">"},
		Count:    count,
		Block:    5 * time.Second,
	}).Result()

	if err == redis.Nil {
		return nil, ErrNoJobs
	}
	if err != nil {
		return nil, fmt.Errorf("xreadgroup: %w", err)
	}

	var jobs []*QueuedJob
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			dataStr, ok := msg.Values["data"].(string)
			if !ok {
				continue
			}
			var job Job
			if err := json.Unmarshal([]byte(dataStr), &job); err != nil {
				continue
			}
			jobs = append(jobs, &QueuedJob{
				ID:    msg.ID,
				Job:   &job,
				Claim: consumerName,
			})
		}
	}
	return jobs, nil
}

// AckJob acknowledges a completed job.
func (q *RedisStreamQueue) AckJob(ctx context.Context, groupName, jobID string) error {
	return q.client.XAck(ctx, q.jobStream, groupName, jobID).Err()
}

// GetPendingCount returns the number of pending (unacknowledged) jobs.
func (q *RedisStreamQueue) GetPendingCount(ctx context.Context) (int64, error) {
	info, err := q.client.XPending(ctx, q.jobStream, "nimbusrun:workers").Result()
	if err != nil {
		return 0, err
	}
	return info.Count, nil
}

// GetQueueDepth returns total messages in the stream.
func (q *RedisStreamQueue) GetQueueDepth(ctx context.Context) (int64, error) {
	return q.client.XLen(ctx, q.jobStream).Result()
}

// DeleteJob deletes a job from the stream.
func (q *RedisStreamQueue) DeleteJob(ctx context.Context, jobID string) error {
	return q.client.XDel(ctx, q.jobStream, jobID).Err()
}

// EnqueueDLQ moves a failed job to the dead-letter queue.
func (q *RedisStreamQueue) EnqueueDLQ(ctx context.Context, job *Job, errMsg string) error {
	data, _ := json.Marshal(job)
	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.jobStream + ":dlq",
		ID:     "*",
		Values: map[string]interface{}{
			"job_id":    uuid.New().String(),
			"data":      string(data),
			"error":     errMsg,
			"timestamp": time.Now().Unix(),
		},
	}).Err()
}

// ReadResults reads completed job results from the results stream.
func (q *RedisStreamQueue) ReadResults(ctx context.Context, lastID string, count int64) ([]JobResult, error) {
	streams, err := q.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{q.resultStream, lastID},
		Count:   count,
		Block:   2 * time.Second,
	}).Result()

	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var results []JobResult
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			dataStr, ok := msg.Values["data"].(string)
			if !ok {
				continue
			}
			var result JobResult
			if err := json.Unmarshal([]byte(dataStr), &result); err != nil {
				continue
			}
			result.MessageID = msg.ID
			results = append(results, result)
		}
	}
	return results, nil
}

// GetDLQEntries returns messages from the dead-letter queue.
func (q *RedisStreamQueue) GetDLQEntries(ctx context.Context, count int64) ([]redis.XMessage, error) {
	return q.client.XRange(ctx, q.jobStream+":dlq", "-", "+").Result()
}
