package kvclient

import (
	"fmt"
	"strconv"
	"strings"
)

func formatQuoted(value []byte) string {
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

func formatSetCommand(key string, value []byte) string {
	return fmt.Sprintf("SET %s VALUE %s", formatQuoted([]byte(key)), formatQuoted(value))
}

func formatGetCommand(key string) string {
	return fmt.Sprintf("GET %s", formatQuoted([]byte(key)))
}

func formatDeleteCommand(key string) string {
	return fmt.Sprintf("DELETE %s", formatQuoted([]byte(key)))
}

func parseResponseLine(line string) (payload string, err error) {
	line = strings.TrimSpace(line)

	if strings.HasPrefix(line, "ERROR: ") {
		return "", mapResponseError(strings.TrimPrefix(line, "ERROR: "))
	}

	if !strings.HasPrefix(line, "SUCCESS:") {
		return "", fmt.Errorf("unexpected response: %q", line)
	}

	rest := strings.TrimSpace(strings.TrimPrefix(line, "SUCCESS:"))
	return rest, nil
}

func parseGetPayload(payload string) ([]byte, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, fmt.Errorf("empty GET payload")
	}

	value, err := strconv.Unquote(payload)
	if err != nil {
		return nil, fmt.Errorf("unquote GET payload: %w", err)
	}

	return []byte(value), nil
}

func mapResponseError(message string) error {
	switch message {
	case "Key not found":
		return ErrNotFound
	case "server busy":
		return ErrServerBusy
	default:
		return fmt.Errorf("kv error: %s", message)
	}
}
