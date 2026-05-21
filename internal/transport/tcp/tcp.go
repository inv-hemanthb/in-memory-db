package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultMaxLineBytes    = 64 << 10
	defaultMaxConnections  = 256
	defaultReadTimeout     = 30 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 5 * time.Minute
	defaultShutdownTimeout = 10 * time.Second
	defaultAcceptBackoff   = 50 * time.Millisecond
)

var ErrLineTooLong = errors.New("line too long")

type Logger interface {
	Trace(string, ...any)
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

type Handler func(line string) (result string, err error)

type Config struct {
	MaxLineBytes    int
	MaxConnections  int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	AcceptBackoff   time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxLineBytes:    defaultMaxLineBytes,
		MaxConnections:  defaultMaxConnections,
		ReadTimeout:     defaultReadTimeout,
		WriteTimeout:    defaultWriteTimeout,
		IdleTimeout:     defaultIdleTimeout,
		ShutdownTimeout: defaultShutdownTimeout,
		AcceptBackoff:   defaultAcceptBackoff,
	}
}

type TCPServer struct {
	addr         string
	logger       Logger
	config       Config
	handler      Handler
	sem          chan struct{}
	wg           sync.WaitGroup
	shuttingDown atomic.Bool
}

func New(addr string, logger Logger) *TCPServer {
	return NewWithConfig(addr, logger, DefaultConfig(), HandleCommandStr)
}

func NewWithConfig(addr string, logger Logger, config Config, handler Handler) *TCPServer {
	if handler == nil {
		handler = HandleCommandStr
	}
	return &TCPServer{
		addr:    addr,
		logger:  logger,
		config:  config,
		handler: handler,
		sem:     make(chan struct{}, config.MaxConnections),
	}
}

func (server *TCPServer) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", server.addr)
	if err != nil {
		return err
	}
	return server.serve(ctx, ln)
}

func (server *TCPServer) RunListener(ctx context.Context, ln net.Listener) error {
	return server.serve(ctx, ln)
}

func (server *TCPServer) serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		server.beginShutdown(ln)
	}()

	server.logger.Info("TCP server started on %s", ln.Addr().String())

	for {
		conn, err := ln.Accept()
		if err != nil {
			if server.shuttingDown.Load() {
				return server.waitForConnections()
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Temporary() {
				time.Sleep(server.config.AcceptBackoff)
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}

		select {
		case server.sem <- struct{}{}:
			server.wg.Add(1)
			go func(c net.Conn) {
				defer server.wg.Done()
				defer func() { <-server.sem }()
				server.handleConnection(c)
			}(conn)
		default:
			server.rejectBusy(conn)
		}
	}
}

func (server *TCPServer) beginShutdown(ln net.Listener) {
	if server.shuttingDown.Swap(true) {
		return
	}
	ln.Close()
}

func (server *TCPServer) waitForConnections() error {
	done := make(chan struct{})
	go func() {
		server.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(server.config.ShutdownTimeout):
		return context.DeadlineExceeded
	}
}

func (server *TCPServer) rejectBusy(conn net.Conn) {
	_ = conn.SetWriteDeadline(time.Now().Add(server.config.WriteTimeout))
	_, _ = io.WriteString(conn, "ERROR: server busy\n")
	conn.Close()
	server.logger.Warn("connection rejected: at capacity")
}

func (server *TCPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	connStart := time.Now()

	for {
		if err := conn.SetReadDeadline(server.readDeadline(connStart)); err != nil {
			return
		}

		line, err := readLine(reader, server.config.MaxLineBytes)
		if err != nil {
			if errors.Is(err, ErrLineTooLong) {
				server.writeResponse(conn, writer, "ERROR: line too long\n")
			} else if !isTimeoutOrEOF(err) {
				server.logger.Error("Failed to read from connection: %v", err)
			}
			return
		}

		server.logger.Trace("Got `%s` from client", line)
		result, err := server.handler(line)

		var response string
		if err != nil {
			response = "ERROR: " + err.Error() + "\n"
		} else {
			response = "SUCCESS: " + result + "\n"
		}

		if err := server.writeResponse(conn, writer, response); err != nil {
			server.logger.Error("Failed to write to connection: %v", err)
			return
		}
	}
}

func (server *TCPServer) readDeadline(connStart time.Time) time.Time {
	deadline := time.Now().Add(server.config.ReadTimeout)
	if server.config.IdleTimeout > 0 {
		idleEnd := connStart.Add(server.config.IdleTimeout)
		if idleEnd.Before(deadline) {
			deadline = idleEnd
		}
	}
	return deadline
}

func (server *TCPServer) writeResponse(conn net.Conn, writer *bufio.Writer, response string) error {
	if err := conn.SetWriteDeadline(time.Now().Add(server.config.WriteTimeout)); err != nil {
		return err
	}
	if _, err := writer.WriteString(response); err != nil {
		return err
	}
	return writer.Flush()
}

func readLine(reader *bufio.Reader, max int) (string, error) {
	var buf []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				if len(buf) == 0 {
					return "", io.EOF
				}
				return strings.TrimRight(string(buf), "\r"), nil
			}
			return "", err
		}
		if b == '\n' {
			return strings.TrimRight(string(buf), "\r"), nil
		}
		buf = append(buf, b)
		if len(buf) > max {
			return "", ErrLineTooLong
		}
	}
}

func isTimeoutOrEOF(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
