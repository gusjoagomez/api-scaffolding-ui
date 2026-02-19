package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-scaffolding/internal/config"
	"api-scaffolding/internal/database"
	"api-scaffolding/internal/server"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config/.env", "Path to .env configuration file")
	flag.Parse()

	// Load Configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// Fallback to old default if .env not found
		if configPath == "config/.env" {
			cfg, err = config.LoadConfig("config/.envapi")
		}
		if err != nil {
			log.Fatalf("Error loading configuration: %v", err)
		}
	}

	// Connect to Meta Database (Postgres)
	// The apigen schema lives here.
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUsername, cfg.DBPassword,
		cfg.DBName, cfg.DBSSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()

	log.Println("Connected to meta database successfully")

	// Init Schema
	if err := database.InitSchema(db, cfg.DBSchema); err != nil {
		log.Printf("Warning during schema initialization: %v", err)
	}

	// Start Server
	srv := server.NewServer(cfg, db)

	// Create a channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	// Context for graceful shutdown could be added here
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// graceful shutdown logic would go here if server had Shutdown method,
	// but http.ListenAndServe blocks.
	// To support graceful shutdown properly, srv.Start() should return the *http.Server
	// and we call Shutdown on it.
	// For now, simple exit is fine for dev tool.
}
