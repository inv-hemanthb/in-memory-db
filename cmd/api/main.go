package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/inv-hemanthb/in-memory-db/internal/api"
	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/api/kvclient"
	"github.com/inv-hemanthb/in-memory-db/internal/db"
	"github.com/inv-hemanthb/in-memory-db/internal/logger"
)

func main() {
	if err := db.LoadEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "load env: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(os.Stdout, logger.LevelInfo, true)

	sqlDB, err := db.Open()
	if err != nil {
		log.Error("open db: %v", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	store := apidb.NewStore(sqlDB)

	kv, err := kvclient.NewFromEnv()
	if err != nil {
		log.Error("kv client: %v", err)
		os.Exit(1)
	}

	server := api.NewServer(store, kv, log)

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf(":%s", port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, addr); err != nil {
		log.Error("HTTP server stopped with error: %v", err)
		os.Exit(1)
	}
}
