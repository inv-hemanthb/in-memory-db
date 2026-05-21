package tcp

import (
	"errors"
	"strings"

	"github.com/inv-hemanthb/in-memory-db/internal/kv/engine"
	"github.com/inv-hemanthb/in-memory-db/internal/parser"
)

type CommandHandler struct {
	engine *engine.Engine
}

func NewCommandHandler(e *engine.Engine) *CommandHandler {
	return &CommandHandler{engine: e}
}

func (handler *CommandHandler) Handle(line string) (string, error) {
	cmd, err := parser.Parse(line)
	if err != nil {
		return "", err
	}

	result := handler.engine.ExecuteCommand(cmd)
	if result.Status == engine.EngineError {
		return "", errors.New(result.ErrorMessage)
	}

	if cmd.Type == parser.CmdGet {
		return formatQuotedValue(result.Value), nil
	}

	return "", nil
}

func formatQuotedValue(value []byte) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, b := range value {
		switch b {
		case '\\':
			builder.WriteString(`\\`)
		case '"':
			builder.WriteString(`\"`)
		default:
			builder.WriteByte(b)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}
