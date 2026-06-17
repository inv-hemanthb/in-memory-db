package kvclient

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/inv-hemanthb/in-memory-db/internal/db"
)

const defaultAddr = "localhost:55555"

type Client struct {
	addr string
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
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return "", fmt.Errorf("dial kv: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadlineFromContext(ctx)); err != nil {
		return "", err
	}

	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil {
		return "", fmt.Errorf("write command: %w", err)
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return parseResponseLine(strings.TrimSuffix(line, "\n"))
}

func deadlineFromContext(ctx context.Context) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		return deadline
	}
	return time.Time{}
}
