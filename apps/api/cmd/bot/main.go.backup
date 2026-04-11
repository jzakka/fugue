package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/chungsanghwa/fugue/apps/api/internal/bot"
	"github.com/chungsanghwa/fugue/apps/api/internal/bot/sources"
	db "github.com/chungsanghwa/fugue/apps/api/internal/db"
	"github.com/chungsanghwa/fugue/apps/api/internal/storage"
)

func main() {
	_ = godotenv.Load()

	log.Println("fuguebot: starting crawl run")

	// Database
	dbURL := envOrDefault("DATABASE_URL", "postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable")
	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("fuguebot: database open: %v", err)
	}
	defer func() { _ = database.Close() }()
	database.SetMaxOpenConns(5)
	database.SetMaxIdleConns(2)
	database.SetConnMaxLifetime(5 * time.Minute)

	if err := database.Ping(); err != nil {
		log.Fatalf("fuguebot: database ping: %v", err)
	}

	// Storage (S3/MinIO)
	store, err := storage.NewClient(storage.Config{
		Endpoint:  envOrDefault("S3_ENDPOINT", "http://localhost:9000"),
		Region:    envOrDefault("S3_REGION", "us-east-1"),
		Bucket:    envOrDefault("S3_BUCKET", "fugue-media"),
		AccessKey: envOrDefault("S3_ACCESS_KEY", "fugue"),
		SecretKey: envOrDefault("S3_SECRET_KEY", "fuguedev123"),
		PublicURL: envOrDefault("S3_PUBLIC_URL", "http://localhost:9000/fugue-media"),
	})
	if err != nil {
		log.Fatalf("fuguebot: storage: %v", err)
	}

	// Create engine
	queries := db.New(database)
	engine := bot.NewEngine(queries, store)

	// Register sources
	if key := os.Getenv("UNSPLASH_ACCESS_KEY"); key != "" {
		engine.RegisterSource(sources.NewUnsplash(key))
		log.Println("fuguebot: registered unsplash source")
	} else {
		log.Println("fuguebot: UNSPLASH_ACCESS_KEY not set, skipping unsplash")
	}

	// FMA doesn't need API keys, always register
	engine.RegisterSource(sources.NewFMA(nil))
	log.Println("fuguebot: registered fma source")

	// Run the crawl
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := engine.Run(ctx); err != nil {
		log.Fatalf("fuguebot: crawl failed: %v", err)
	}

	log.Println("fuguebot: crawl run completed")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
