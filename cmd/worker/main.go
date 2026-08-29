package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nimbusrun/nimbusrun/internal/config"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database (optional for worker)
	databaseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName, cfg.Database.SSLMode)

	_, err = db.New(databaseURL)
	if err != nil {
		log.Printf("Warning: Could not connect to database: %v", err)
		// Worker can still function without DB for pure execution
	}

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create runner
	runner, err := worker.NewRunner(30 * time.Second)
	if err != nil {
		log.Fatalf("Failed to create container runner: %v", err)
	}

	// Create worker service
	service := worker.NewService(
		runner,
		rdb,
		fmt.Sprintf("worker-%d", os.Getpid()),
		"nimbusrun:jobs",
		fmt.Sprintf("worker-%d", os.Getpid()),
		5*time.Second,
	)

	// Start heartbeat goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Simple heartbeat - real implementation would collect metrics
				hb := worker.HeartbeatInfo{
					WorkerID:        fmt.Sprintf("worker-%d", os.Getpid()),
					Hostname:        "localhost",
					CPUUsage:        0.0,
					MemoryUsage:     0.0,
					RunningTasks:    0,
					AvailableSlots:  10,
					Timestamp:       time.Now().Unix(),
				}
				service.SendHeartbeat(context.Background(), hb)
			}
		}
	}()

	// Start service in background
	go func() {
		if err := service.Start(context.Background()); err != nil {
			log.Printf("Worker service error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\nShutting down worker...")
}