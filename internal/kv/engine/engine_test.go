package engine

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/inv-hemanthb/in-memory-db/internal/kv/store"
	"github.com/inv-hemanthb/in-memory-db/internal/parser"
)

type mockTimeProvider struct {
	mutext sync.Mutex
	now    time.Time
}

func newMockTimeProvider(start time.Time) *mockTimeProvider {
	return &mockTimeProvider{
		now: start,
	}
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

type Operation struct {
	Action string

	Cmd parser.Command

	WantStatus EngineStatus
	WantValue  []byte
	WantErrMsg string

	AdvanceBy time.Duration
}

type TestCase struct {
	Name       string
	Operations []Operation
}

func compareValues(got []byte, want []byte) error {
	if !bytes.Equal(got, want) {
		return fmt.Errorf("value mismatch: got %q, want %q", string(got), string(want))
	}
	return nil
}

func compareEngineResult(got, want EngineResult) error {
	if got.Status != want.Status {
		return fmt.Errorf("status mismatch: got %v, want %v", got.Status, want.Status)
	}
	if got.ErrorMessage != want.ErrorMessage {
		return fmt.Errorf("error message mismatch: got %q, want %q", got.ErrorMessage, want.ErrorMessage)
	}
	if want.Status == EngineSuccess && want.Value != nil {
		if err := compareValues(got.Value, want.Value); err != nil {
			return err
		}
	}
	return nil
}

func wantResult(op Operation) EngineResult {
	return EngineResult{
		Status:       op.WantStatus,
		Value:        op.WantValue,
		ErrorMessage: op.WantErrMsg,
	}
}

func createTestCases() []TestCase {
	ptr := func(v int64) *int64 { return &v }

	return []TestCase{
		{
			Name: "SET and GET",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "foo", Value: []byte("bar")},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "foo"},
					WantStatus: EngineSuccess,
					WantValue:  []byte("bar"),
				},
			},
		},
		{
			Name: "GET missing key",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "missing"},
					WantStatus: EngineError,
					WantErrMsg: "Key not found",
				},
			},
		},
		{
			Name: "DELETE key",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "k", Value: []byte("v")},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdDelete, Key: "k"},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "k"},
					WantStatus: EngineError,
					WantErrMsg: "Key not found",
				},
			},
		},
		{
			Name: "CLEAR store",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "k1", Value: []byte("v1")},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "k2", Value: []byte("v2")},
					WantStatus: EngineSuccess,
				},
				{
					Action:     "exec",
					Cmd:        parser.Command{Type: parser.CmdClear},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "k1"},
					WantStatus: EngineError,
					WantErrMsg: "Key not found",
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "k2"},
					WantStatus: EngineError,
					WantErrMsg: "Key not found",
				},
			},
		},
		{
			Name: "SET with TTL",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "foo", Value: []byte("bar"), TTL: ptr(60)},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "foo"},
					WantStatus: EngineSuccess,
					WantValue:  []byte("bar"),
				},
				{Action: "advance", AdvanceBy: 2 * time.Minute},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "foo"},
					WantStatus: EngineError,
					WantErrMsg: "Key not found",
				},
			},
		},
		{
			Name: "SET without TTL",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "foo", Value: []byte("bar")},
					WantStatus: EngineSuccess,
				},
				{Action: "advance", AdvanceBy: 10 * time.Second},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "foo"},
					WantStatus: EngineSuccess,
					WantValue:  []byte("bar"),
				},
			},
		},
		{
			Name: "Overwrite key",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "k", Value: []byte("v1")},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "k", Value: []byte("v2")},
					WantStatus: EngineSuccess,
				},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "k"},
					WantStatus: EngineSuccess,
					WantValue:  []byte("v2"),
				},
			},
		},
		{
			Name: "SET with zero TTL",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdSet, Key: "k", Value: []byte("v"), TTL: ptr(0)},
					WantStatus: EngineSuccess,
				},
				{Action: "advance", AdvanceBy: time.Nanosecond},
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CmdGet, Key: "k"},
					WantStatus: EngineError,
					WantErrMsg: "Key not found",
				},
			},
		},
		{
			Name: "Unknown command",
			Operations: []Operation{
				{
					Action: "exec",
					Cmd:    parser.Command{Type: parser.CommandType(999)},
					WantStatus: EngineError,
					WantErrMsg: "Unknown command",
				},
			},
		},
	}
}

func TestEngineExecuteCommand(t *testing.T) {
	tests := createTestCases()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			tp := newMockTimeProvider(time.Now())
			kvStore := store.New(tp)
			eng := New(kvStore)

			for i, op := range tt.Operations {
				switch op.Action {
				case "exec":
					got := eng.ExecuteCommand(op.Cmd)
					if diff := compareEngineResult(got, wantResult(op)); diff != nil {
						t.Fatalf("step %d: %v\nCmd: %+v\nGot:  %+v\nWant: %+v", i, diff, op.Cmd, got, wantResult(op))
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

func TestEngineStatus_String(t *testing.T) {
	tests := []struct {
		status EngineStatus
		want   string
	}{
		{EngineSuccess, "SUCCESS"},
		{EngineError, "ERROR"},
		{EngineStatus(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Fatalf("got %s, want %s", got, tt.want)
		}
	}
}

func TestEngineResult_Error(t *testing.T) {
	success := EngineResult{Status: EngineSuccess}
	if success.Error() != "" {
		t.Fatalf("expected empty error on success")
	}

	errResult := EngineResult{Status: EngineError, ErrorMessage: "Key not found"}
	if errResult.Error() != "Key not found" {
		t.Fatalf("got %q, want %q", errResult.Error(), "Key not found")
	}
}
