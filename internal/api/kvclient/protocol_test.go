package kvclient

import (
	"errors"
	"testing"
)

func TestParseResponseLine(t *testing.T) {
	payload, err := parseResponseLine("SUCCESS: \"hello\"")
	if err != nil {
		t.Fatalf("parse success: %v", err)
	}
	if payload != `"hello"` {
		t.Fatalf("payload = %q", payload)
	}

	_, err = parseResponseLine("ERROR: Key not found")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("parse not found: got %v", err)
	}

	_, err = parseResponseLine("ERROR: server busy")
	if !errors.Is(err, ErrServerBusy) {
		t.Fatalf("parse busy: got %v", err)
	}
}

func TestParseGetPayload(t *testing.T) {
	got, err := parseGetPayload(`"say \"hi\""`)
	if err != nil {
		t.Fatalf("parse get payload: %v", err)
	}
	if string(got) != `say "hi"` {
		t.Fatalf("got = %q", got)
	}
}

func TestFormatSetCommand(t *testing.T) {
	cmd := formatSetCommand("k", []byte(`a"b\c`))
	want := `SET "k" VALUE "a\"b\\c"`
	if cmd != want {
		t.Fatalf("cmd = %q, want %q", cmd, want)
	}
}
