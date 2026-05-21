package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/inv-hemanthb/in-memory-db/internal/kv/engine"
	"github.com/inv-hemanthb/in-memory-db/internal/kv/store"
	"github.com/inv-hemanthb/in-memory-db/internal/logger"
	"github.com/inv-hemanthb/in-memory-db/internal/timeprovider"
	"github.com/inv-hemanthb/in-memory-db/internal/transport/tcp"
)

func main() {
	log := logger.New(os.Stdout, logger.LevelTrace, true)

	tp := timeprovider.New()
	kvStore := store.New(tp)
	eng := engine.New(kvStore)
	cmdHandler := tcp.NewCommandHandler(eng)

	tcpServer := tcp.New("localhost:55555", log, cmdHandler.Handle)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := tcpServer.Run(ctx); err != nil {
		log.Error("TCP server stopped with error: %v", err)
		return
	}
	log.Info("TCP server stopped")
}
