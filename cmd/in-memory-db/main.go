package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/inv-hemanthb/in-memory-db/internal/logger"
	"github.com/inv-hemanthb/in-memory-db/internal/transport/tcp"
)

func main() {
	log := logger.New(os.Stdout, logger.LevelTrace, true)

	tcpServer := tcp.New("localhost:55555", log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := tcpServer.Run(ctx); err != nil {
		log.Error("TCP server stopped with error: %v", err)
		return
	}
	log.Info("TCP server stopped")
}
