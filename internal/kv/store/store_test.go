package store

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"
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
	Action    string
	Key       string
	Value     []byte
	ExpiresAt int64
	AdvanceBy time.Duration

	WantValue []byte
	WantOK    bool
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

func createTestCases() []TestCase {
	now := time.Now().UnixNano()

	return []TestCase{
		{
			Name: "SET and GET",
			Operations: []Operation{
				{Action: "set", Key: "foo", Value: []byte("bar")},
				{Action: "get", Key: "foo", WantValue: []byte("bar"), WantOK: true},
			},
		},
		{
			Name: "GET missing key",
			Operations: []Operation{
				{Action: "get", Key: "missing", WantOK: false},
			},
		},
		{
			Name: "DELETE key",
			Operations: []Operation{
				{Action: "set", Key: "k", Value: []byte("v")},
				{Action: "delete", Key: "k"},
				{Action: "get", Key: "k", WantOK: false},
			},
		},
		{
			Name: "CLEAR store",
			Operations: []Operation{
				{Action: "set", Key: "k1", Value: []byte("v1")},
				{Action: "set", Key: "k2", Value: []byte("v2")},
				{Action: "clear"},
				{Action: "get", Key: "k1", WantOK: false},
				{Action: "get", Key: "k2", WantOK: false},
			},
		},
		{
			Name: "SET with expiration",
			Operations: []Operation{
				{
					Action:    "set",
					Key:       "foo",
					Value:     []byte("bar"),
					ExpiresAt: now + int64(time.Second),
				},
				{Action: "get", Key: "foo", WantValue: []byte("bar"), WantOK: true},
				{Action: "advance", AdvanceBy: 2 * time.Second},
				{Action: "get", Key: "foo", WantOK: false},
			},
		},
		{
			Name: "SET without expiration",
			Operations: []Operation{
				{
					Action: "set",
					Key:    "foo",
					Value:  []byte("bar"),
				},
				{Action: "advance", AdvanceBy: 10 * time.Second},
				{Action: "get", Key: "foo", WantValue: []byte("bar"), WantOK: true},
			},
		},
		{
			Name: "Overwrite key",
			Operations: []Operation{
				{Action: "set", Key: "k", Value: []byte("v1")},
				{Action: "set", Key: "k", Value: []byte("v2")},
				{Action: "get", Key: "k", WantValue: []byte("v2"), WantOK: true},
			},
		},
		{
			Name: "Value immutability",
			Operations: []Operation{
				{Action: "set", Key: "k", Value: []byte("hello")},
				{
					Action:    "get",
					Key:       "k",
					WantValue: []byte("hello"),
					WantOK:    true,
				},
			},
		},
	}
}

func TestKVStore(t *testing.T) {
	tests := createTestCases()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			tp := newMockTimeProvider(time.Now())
			store := New(tp)

			for i, op := range tt.Operations {
				switch op.Action {

				case "set":
					store.Set(op.Key, op.Value, op.ExpiresAt)

				case "get":
					got, ok := store.Get(op.Key)

					if ok != op.WantOK {
						t.Fatalf("step %d: expected ok=%v, got %v", i, op.WantOK, ok)
					}

					if !ok {
						return
					}

					if op.WantValue == nil {
						t.Fatalf("step %d: WantValue must be set when WantOK is true", i)
					}

					if err := compareValues(got, op.WantValue); err != nil {
						t.Fatalf("step %d: %v", i, err)
					}

				case "delete":
					store.Delete(op.Key)

				case "clear":
					store.Clear()

				case "advance":
					tp.Add(op.AdvanceBy)

				default:
					t.Fatalf("unknown action: %s", op.Action)
				}
			}
		})
	}

}
