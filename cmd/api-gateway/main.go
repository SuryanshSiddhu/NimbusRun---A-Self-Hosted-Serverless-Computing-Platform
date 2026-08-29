package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nimbusrun/nimbusrun/internal/auth"
	"github.com/nimbusrun/nimbusrun/internal/config"
	"github.com/nimbusrun/nimbusrun/internal/db"
	"github.com/nimbusrun/nimbusrun/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize database
	databaseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.DBName, cfg.Database.SSLMode)

	database, err := db.New(databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	if err := db.RunMigrations(database, "migrations"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	// Initialize auth service
	authService := auth.NewService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)

	// Setup HTTP server
	server := http.NewServer(cfg, database, authService)
	server.SetupRoutes()

	// Start server in a goroutine
	go func() {
		fmt.Printf("NimbusRun API Gateway starting on %s:%s\n", cfg.Server.Host, cfg.Server.Port)
		if err := server.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down NimbusRun...")
}
