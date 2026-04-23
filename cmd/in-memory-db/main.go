package main

import (
	"os"

	"github.com/inv-hemanthb/in-memory-db/internal/logger"
	"github.com/inv-hemanthb/in-memory-db/internal/transport/tcp"
)

func main() {
	log := logger.New(os.Stdout, logger.LevelTrace, true)

	tcpServer := tcp.New("localhost:55555", log)
	err := tcpServer.Start()

	if err != nil {
		log.Error("Failed to start tcp server: %v", err)
	}
}
