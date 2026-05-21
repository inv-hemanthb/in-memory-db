package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockLogger struct{}

func (mockLogger) Trace(string, ...any) {}
func (mockLogger) Info(string, ...any)  {}
func (mockLogger) Warn(string, ...any)  {}
func (mockLogger) Error(string, ...any) {}

type readLineTestCase struct {
	Name         string
	Input        string
	Want         string
	ExpectErr    bool
	ExpectErrIs  error
	MaxLineBytes int
}

func createReadLineTestCases() []readLineTestCase {
	return []readLineTestCase{
		{
			Name:  "normal line",
			Input: "hello\n",
			Want:  "hello",
		},
		{
			Name:  "crlf trimmed",
			Input: "hello\r\n",
			Want:  "hello",
		},
		{
			Name:  "empty line",
			Input: "\n",
			Want:  "",
		},
		{
			Name:      "exceeds max bytes",
			Input:     strings.Repeat("a", 5) + "\n",
			ExpectErr: true,
			ExpectErrIs: ErrLineTooLong,
			MaxLineBytes: 4,
		},
		{
			Name:      "no newline hits limit",
			Input:     "aaaaa",
			ExpectErr: true,
			ExpectErrIs: ErrLineTooLong,
			MaxLineBytes: 4,
		},
		{
			Name:      "eof without newline",
			Input:     "partial",
			Want:      "partial",
		},
		{
			Name:      "eof empty",
			Input:     "",
			ExpectErr: true,
		},
	}
}

func TestReadLine(t *testing.T) {
	tests := createReadLineTestCases()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			max := defaultMaxLineBytes
			if tt.MaxLineBytes > 0 {
				max = tt.MaxLineBytes
			}

			got, err := readLine(bufio.NewReader(strings.NewReader(tt.Input)), max)

			if (err != nil) != tt.ExpectErr {
				t.Fatalf("readLine(%q) unexpected error state: %v", tt.Input, err)
			}

			if tt.ExpectErr && tt.ExpectErrIs != nil && !errors.Is(err, tt.ExpectErrIs) {
				t.Fatalf("readLine(%q) got err %v, want %v", tt.Input, err, tt.ExpectErrIs)
			}

			if !tt.ExpectErr {
				if diff := compareString(got, tt.Want); diff != nil {
					t.Errorf("\nInput: %q\nGot:  %q\nWant: %q\n%v", tt.Input, got, tt.Want, diff)
				}
			}
		})
	}
}

func compareString(got, want string) error {
	if got != want {
		return fmt.Errorf("string mismatch: got %q, want %q", got, want)
	}
	return nil
}

func compareResponse(got, want string) error {
	if got != want {
		return fmt.Errorf("response mismatch: got %q, want %q", got, want)
	}
	return nil
}

type Operation struct {
	Action       string
	Line         string
	Want         string
	ExpectEOF    bool
	Sleep        time.Duration
	Hold         bool
	CancelServer bool
}

type serverTestCase struct {
	Name       string
	Config     Config
	Handler    Handler
	Operations []Operation
}

func testConfig() Config {
	return Config{
		MaxLineBytes:    1024,
		MaxConnections:  2,
		ReadTimeout:     200 * time.Millisecond,
		WriteTimeout:    200 * time.Millisecond,
		IdleTimeout:     time.Second,
		ShutdownTimeout: 2 * time.Second,
		AcceptBackoff:   10 * time.Millisecond,
	}
}

func createServerTestCases() []serverTestCase {
	return []serverTestCase{
		{
			Name:   "handler success",
			Config: testConfig(),
			Handler: func(line string) (string, error) {
				return "ok-" + line, nil
			},
			Operations: []Operation{
				{Action: "send", Line: "PING\n", Want: "SUCCESS: ok-PING\n"},
			},
		},
		{
			Name:   "handler error",
			Config: testConfig(),
			Handler: func(string) (string, error) {
				return "", errors.New("bad command")
			},
			Operations: []Operation{
				{Action: "send", Line: "BAD\n", Want: "ERROR: bad command\n"},
			},
		},
		{
			Name:   "line too long",
			Config: testConfig(),
			Handler: func(string) (string, error) {
				return "ok", nil
			},
			Operations: []Operation{
				{
					Action: "sendRaw",
					Line:   strings.Repeat("x", testConfig().MaxLineBytes+1) + "\n",
					Want:   "ERROR: line too long\n",
				},
			},
		},
		{
			Name: "max connections",
			Config: func() Config {
				c := testConfig()
				c.MaxConnections = 1
				return c
			}(),
			Handler: func(string) (string, error) {
				time.Sleep(500 * time.Millisecond)
				return "ok", nil
			},
			Operations: []Operation{
				{Action: "holdSend", Line: "WAIT\n"},
				{Action: "dialBusy"},
				{Action: "releaseHold"},
			},
		},
		{
			Name:   "read timeout",
			Config: testConfig(),
			Handler: func(string) (string, error) {
				return "ok", nil
			},
			Operations: []Operation{
				{Action: "openIdle", Line: "no newline yet"},
				{Action: "sleep", Sleep: 400 * time.Millisecond},
				{Action: "expectIdleClosed"},
			},
		},
		{
			Name:   "graceful shutdown",
			Config: testConfig(),
			Handler: func(string) (string, error) {
				time.Sleep(50 * time.Millisecond)
				return "done", nil
			},
			Operations: []Operation{
				{Action: "send", Line: "WORK\n", Want: "SUCCESS: done\n"},
				{Action: "cancelServer"},
			},
		},
	}
}

func TestTCPServer(t *testing.T) {
	tests := createServerTestCases()

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			server := NewWithConfig(ln.Addr().String(), mockLogger{}, tt.Config, tt.Handler)
			errCh := make(chan error, 1)
			go func() {
				errCh <- server.RunListener(ctx, ln)
			}()

			var holdConn net.Conn
			var holdRelease chan struct{}
			var holdOnce sync.Once
			var idleConn net.Conn

			for i, op := range tt.Operations {
				switch op.Action {
				case "send":
					got := dialAndReadLine(t, ln.Addr().String(), op.Line)
					if diff := compareResponse(got, op.Want); diff != nil {
						t.Fatalf("step %d: %v", i, diff)
					}

				case "sendRaw":
					got := dialAndReadLine(t, ln.Addr().String(), op.Line)
					if diff := compareResponse(got, op.Want); diff != nil {
						t.Fatalf("step %d: %v", i, diff)
					}

				case "holdSend":
					holdConn = dial(t, ln.Addr().String())
					holdRelease = make(chan struct{})
					go func() {
						_, _ = holdConn.Write([]byte(op.Line))
						<-holdRelease
					}()

				case "releaseHold":
					holdOnce.Do(func() { close(holdRelease) })
					holdConn.Close()

				case "dialBusy":
					time.Sleep(20 * time.Millisecond)
					conn := dial(t, ln.Addr().String())
					_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
					buf := make([]byte, 64)
					n, readErr := conn.Read(buf)
					conn.Close()
					got := string(buf[:n])
					if diff := compareResponse(got, "ERROR: server busy\n"); diff != nil {
						t.Fatalf("step %d: %v (readErr=%v)", i, diff, readErr)
					}

				case "openIdle":
					idleConn = dial(t, ln.Addr().String())
					if _, err := idleConn.Write([]byte(op.Line)); err != nil {
						t.Fatalf("step %d: write: %v", i, err)
					}

				case "sleep":
					time.Sleep(op.Sleep)

				case "expectIdleClosed":
					_ = idleConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
					buf := make([]byte, 16)
					_, err := idleConn.Read(buf)
					idleConn.Close()
					if err == nil {
						t.Fatalf("step %d: expected idle connection to close", i)
					}

				case "cancelServer":
					cancel()
					select {
					case err := <-errCh:
						if err != nil && !errors.Is(err, context.DeadlineExceeded) {
							t.Fatalf("step %d: server exit: %v", i, err)
						}
					case <-time.After(3 * time.Second):
						t.Fatalf("step %d: server did not stop", i)
					}

				default:
					t.Fatalf("step %d: unknown action: %s", i, op.Action)
				}
			}

			if tt.Name != "graceful shutdown" {
				cancel()
				<-errCh
			}
		})
	}
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	return conn
}

func dialAndReadLine(t *testing.T, addr, line string) string {
	t.Helper()
	conn := dial(t, addr)
	defer conn.Close()

	if _, err := conn.Write([]byte(line)); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	got, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return got
}
