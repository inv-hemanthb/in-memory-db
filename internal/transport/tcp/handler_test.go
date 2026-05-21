package tcp

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/inv-hemanthb/in-memory-db/internal/kv/engine"
	"github.com/inv-hemanthb/in-memory-db/internal/kv/store"
)

type mockTimeProvider struct {
	mutext sync.Mutex
	now    time.Time
}

func newMockTimeProvider(start time.Time) *mockTimeProvider {
	return &mockTimeProvider{now: start}
}

func (m *mockTimeProvider) Now() time.Time {
	m.mutext.Lock()
	defer m.mutext.Unlock()
	return m.now
}

func (m *mockTimeProvider) Add(d time.Duration) time.Time {
	m.mutext.Lock()
	defer m.mutext.Unlock()
	m.now = m.now.Add(d)
	return m.now
}

type handlerOperation struct {
	Action    string
	Input     string
	Want      string
	ExpectErr bool
	AdvanceBy time.Duration
}

type handlerTestCase struct {
	Name       string
	Operations []handlerOperation
}

func compareHandlerResult(got, want string) error {
	if got != want {
		return fmt.Errorf("result mismatch: got %q, want %q", got, want)
	}
	return nil
}

func createHandlerTestCases() []handlerTestCase {
	return []handlerTestCase{
		{
			Name: "SET and GET",
			Operations: []handlerOperation{
				{Action: "exec", Input: `SET "k" VALUE "v"`},
				{Action: "exec", Input: `GET "k"`, Want: `"v"`},
			},
		},
		{
			Name: "GET missing key",
			Operations: []handlerOperation{
				{Action: "exec", Input: `GET "nope"`, ExpectErr: true},
			},
		},
		{
			Name: "parse error",
			Operations: []handlerOperation{
				{Action: "exec", Input: `SET bad`, ExpectErr: true},
			},
		},
		{
			Name: "DELETE key",
			Operations: []handlerOperation{
				{Action: "exec", Input: `SET "k" VALUE "v"`},
				{Action: "exec", Input: `DELETE "k"`},
				{Action: "exec", Input: `GET "k"`, ExpectErr: true},
			},
		},
		{
			Name: "CLEAR store",
			Operations: []handlerOperation{
				{Action: "exec", Input: `SET "k1" VALUE "v1"`},
				{Action: "exec", Input: `SET "k2" VALUE "v2"`},
				{Action: "exec", Input: `CLEAR`},
				{Action: "exec", Input: `GET "k1"`, ExpectErr: true},
				{Action: "exec", Input: `GET "k2"`, ExpectErr: true},
			},
		},
		{
			Name: "SET with TTL expiry",
			Operations: []handlerOperation{
				{Action: "exec", Input: `SET "k" VALUE "v" TTL "60"`},
				{Action: "exec", Input: `GET "k"`, Want: `"v"`},
				{Action: "advance", AdvanceBy: 2 * time.Minute},
				{Action: "exec", Input: `GET "k"`, ExpectErr: true},
			},
		},
		{
			Name: "escaped GET value",
			Operations: []handlerOperation{
				{Action: "exec", Input: `SET "k" VALUE "say \"hi\""`},
				{Action: "exec", Input: `GET "k"`, Want: `"say \"hi\""`},
			},
		},
		{
			Name: "unknown command via engine",
			Operations: []handlerOperation{
				{Action: "exec", Input: `FOO "bar"`, ExpectErr: true},
			},
		},
	}
}

func TestCommandHandler_Handle(t *testing.T) {
	tests := createHandlerTestCases()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			tp := newMockTimeProvider(time.Now())
			kvStore := store.New(tp)
			eng := engine.New(kvStore)
			handler := NewCommandHandler(eng)

			for i, op := range tt.Operations {
				switch op.Action {
				case "exec":
					got, err := handler.Handle(op.Input)

					if (err != nil) != op.ExpectErr {
						t.Fatalf("step %d: Input %q unexpected error state: %v", i, op.Input, err)
					}

					if op.ExpectErr {
						if tt.Name == "GET missing key" && !strings.Contains(err.Error(), "Key not found") {
							t.Fatalf("step %d: got %q, want Key not found", i, err.Error())
						}
						continue
					}

					if diff := compareHandlerResult(got, op.Want); diff != nil {
						t.Fatalf("step %d: %v\nInput: %q\nGot:  %q\nWant: %q", i, diff, op.Input, got, op.Want)
					}

				case "advance":
					tp.Add(op.AdvanceBy)

				default:
					t.Fatalf("step %d: unknown action: %s", i, op.Action)
				}
			}
		})
	}
}

func TestFormatQuotedValue(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte("hello"), `"hello"`},
		{[]byte(`say "hi"`), `"say \"hi\""`},
		{[]byte(`path\to`), `"path\\to"`},
		{[]byte(""), `""`},
	}

	for _, tt := range tests {
		if got := formatQuotedValue(tt.input); got != tt.want {
			t.Fatalf("formatQuotedValue(%q) got %q, want %q", tt.input, got, tt.want)
		}
	}
}
