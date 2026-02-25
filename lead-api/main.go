package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/lucasfdcampos/lead-api/internal/api"
	"github.com/lucasfdcampos/lead-api/internal/cache"
	"github.com/lucasfdcampos/lead-api/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// ─── Redis ────────────────────────────────────────────────────────────────
	var redisClient *cache.Client
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPass := os.Getenv("REDIS_PASSWORD")
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	rc := cache.New(redisAddr, redisPass, redisDB)
	ctx5s, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rc.Ping(ctx5s); err != nil {
		log.Printf("WARN: Redis not available (%v) — searches will not be cached in Redis", err)
	} else {
		redisClient = rc
		log.Printf("Redis connected: %s", redisAddr)
	}
	cancel()

	// ─── MongoDB ──────────────────────────────────────────────────────────────
	var mongoClient *store.Client
	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")

	ctx10s, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	mc, err := store.New(ctx10s, mongoURI)
	cancel2()
	if err != nil {
		log.Printf("WARN: MongoDB not available (%v) — searches will not be persisted", err)
	} else {
		mongoClient = mc
		log.Printf("MongoDB connected: %s", mongoURI)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = mc.Disconnect(ctx)
			cancel()
		}()
	}

	// ─── HTTP server ──────────────────────────────────────────────────────────
	addr := getEnv("ADDR", ":8080")
	handler := api.NewHandler(redisClient, mongoClient)
	srv := api.NewServer(addr, handler)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down...")
	ctx, cancel3 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel3()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("bye")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
