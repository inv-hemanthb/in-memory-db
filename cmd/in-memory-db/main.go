package main

import (
	"os"

	"github.com/inv-hemanthb/in-memory-db/internal/logger"
	"github.com/inv-hemanthb/in-memory-db/internal/transport/tcp"
)

func main() {
	log := logger.New(os.Stdout, logger.LevelTrace, true)

	tests := []string{
		`SET "user" VALUE "john"`,
		`SET "user" VALUE "john" TTL "60"`,
		`SET "user" VALUE "{\"name\":\"john\"}"`,
		`SET "user" VALUE "john" TTL "abc"`,
		`SET user VALUE "john"`, // invalid (missing quotes)
		`GET "user"`,
		`DELETE "user"`,
		`CLEAR`,
		`CLEAR "extra"`, // invalid
		`SET "key" VALUE "value with \"quotes\""`,
		`SET "k" VALUE "v" TTL "0"`,
		`SET "k" VALUE "v" TTL "-10"`,
		`SET "k" VALUE "v" TTL "999999999999999999999"`,
		`SET "k" VALUE "v" TTL "\""`,
		`SET "k" VALUE ""`,
		`SET "k" "v" VALUE`,
		`SET "k" TTL "10" VALUE "v"`,
		`SET "k" VALUE "v" EXTRA "x"`,
	}

	for _, input := range tests {
		log.Info("INPUT: %s", input)

		cmd, err := tcp.Parse(input)
		if err != nil {
			log.Error("Parse error: %v", err)
			continue
		}

		log.Info("Parsed Command:")
		log.Info("  Type: %v", cmd.Type)
		log.Info("  Key: %s", cmd.Key)

		if cmd.Value != nil {
			log.Info("  Value: %s", string(cmd.Value))
		}

		if cmd.TTL != nil {
			log.Info("  TTL: %d", *cmd.TTL)
		} else {
			log.Info("  TTL: <nil>")
		}
	}
}
