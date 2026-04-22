package main

import (
	"os"

	"github.com/inv-hemanthb/in-memory-db/internal/kv/engine"
	"github.com/inv-hemanthb/in-memory-db/internal/kv/store"
	"github.com/inv-hemanthb/in-memory-db/internal/logger"
	"github.com/inv-hemanthb/in-memory-db/internal/transport/tcp"
)

func main() {
	log := logger.New(os.Stdout, logger.LevelTrace, true)
	kvStore := store.New()
	kvEngine := engine.New(kvStore)

	commandStrs := []string{
		`SET "user" VALUE "john"`,
		`SET "session" VALUE "abc123" TTL "120"`,
		`GET "user"`,
		`DELETE "session"`,
		`CLEAR`,
		`SET "ads" VALUE "ad`,
	}

	for _, commandStr := range commandStrs {
		command, err := tcp.Parse(commandStr)
		if err != nil {
			log.Error("Failed to parse command: %v", err)
			return
		}

		result := kvEngine.ExecuteCommand(command)
		if result.Status == engine.EngineError {
			log.Error("Command execution failed: %s", result.Error())
			return
		} else {
			log.Info("Command executed successfully: %s => %s", commandStr, result)
		}

	}
}
