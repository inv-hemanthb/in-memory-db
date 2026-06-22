package kvclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/inv-hemanthb/in-memory-db/internal/db"
)

const defaultAddr = "localhost:55555"

type Client struct {
	addr   string
	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader
}

func New(addr string) *Client {
	return &Client{addr: addr}
}

func NewFromEnv() (*Client, error) {
	if err := db.LoadEnv(); err != nil {
		return nil, err
	}

	addr := os.Getenv("KV_ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	return New(addr), nil
}

func (c *Client) Addr() string {
	return c.addr
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeConnLocked()
}

func (c *Client) Set(ctx context.Context, key string, value []byte) error {
	_, err := c.roundTrip(ctx, formatSetCommand(key, value))
	return err
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	payload, err := c.roundTrip(ctx, formatGetCommand(key))
	if err != nil {
		return nil, err
	}

	return parseGetPayload(payload)
}

func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.roundTrip(ctx, formatDeleteCommand(key))
	return err
}

func (c *Client) Clear(ctx context.Context) error {
	_, err := c.roundTrip(ctx, "CLEAR")
	return err
}

func (c *Client) roundTrip(ctx context.Context, command string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for attempt := 0; attempt < 2; attempt++ {
		payload, err := c.roundTripLocked(ctx, command)
		if err == nil {
			return payload, nil
		}
		if errors.Is(err, ErrNotFound) {
			return "", err
		}
		if errors.Is(err, ErrServerBusy) {
			c.closeConnLocked()
			return "", err
		}
		c.closeConnLocked()
		if attempt == 1 {
			return "", err
		}
	}

	return "", fmt.Errorf("kv round trip failed")
}

func (c *Client) roundTripLocked(ctx context.Context, command string) (string, error) {
	if err := c.ensureConnLocked(ctx); err != nil {
		return "", err
	}

	if err := c.conn.SetDeadline(deadlineFromContext(ctx)); err != nil {
		return "", err
	}

	if _, err := fmt.Fprintf(c.conn, "%s\n", command); err != nil {
		return "", fmt.Errorf("write command: %w", err)
	}

	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return parseResponseLine(strings.TrimSuffix(line, "\n"))
}

func (c *Client) ensureConnLocked(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("dial kv: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

func (c *Client) closeConnLocked() error {
	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	c.reader = nil
	return err
}

func deadlineFromContext(ctx context.Context) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline
	}
	return time.Time{}
}
