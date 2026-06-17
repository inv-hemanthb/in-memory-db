package kvclient_test

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/inv-hemanthb/in-memory-db/internal/api/kvclient"
	"github.com/inv-hemanthb/in-memory-db/internal/db"
)

func openTestClient(t *testing.T) *kvclient.Client {
	t.Helper()

	if err := db.LoadEnv(); err != nil {
		t.Fatalf("load env: %v", err)
	}

	client, err := kvclient.NewFromEnv()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	conn, err := net.DialTimeout("tcp", client.Addr(), 2*time.Second)
	if err != nil {
		t.Skipf("kv server unreachable at %s: %v", client.Addr(), err)
	}
	conn.Close()

	return client
}

func testKey(t *testing.T, suffix string) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	return "TEST-" + name + "-" + suffix
}

func TestSetAndGet(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	key := testKey(t, "roundtrip")

	t.Cleanup(func() {
		_ = client.Delete(context.Background(), key)
	})

	if err := client.Set(ctx, key, []byte("alpha")); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "alpha" {
		t.Fatalf("get = %q, want alpha", got)
	}
}

func TestGetNotFound(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	key := testKey(t, "missing")

	_, err := client.Get(ctx, key)
	if !errors.Is(err, kvclient.ErrNotFound) {
		t.Fatalf("get missing: got %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	key := testKey(t, "delete")

	if err := client.Set(ctx, key, []byte("gone")); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := client.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := client.Get(ctx, key)
	if !errors.Is(err, kvclient.ErrNotFound) {
		t.Fatalf("get after delete: got %v, want ErrNotFound", err)
	}
}

func TestSetOverwrite(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	key := testKey(t, "overwrite")

	t.Cleanup(func() {
		_ = client.Delete(context.Background(), key)
	})

	if err := client.Set(ctx, key, []byte("first")); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := client.Set(ctx, key, []byte("second")); err != nil {
		t.Fatalf("set second: %v", err)
	}

	got, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("get = %q, want second", got)
	}
}

func TestQuotedValueEscapes(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	key := testKey(t, "escape")
	value := `say "hello" and \ bye`

	t.Cleanup(func() {
		_ = client.Delete(context.Background(), key)
	})

	if err := client.Set(ctx, key, []byte(value)); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := client.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got) != value {
		t.Fatalf("get = %q, want %q", got, value)
	}
}

func TestClear(t *testing.T) {
	client := openTestClient(t)
	ctx := context.Background()
	key1 := testKey(t, "clear-1")
	key2 := testKey(t, "clear-2")

	if err := client.Set(ctx, key1, []byte("one")); err != nil {
		t.Fatalf("set key1: %v", err)
	}
	if err := client.Set(ctx, key2, []byte("two")); err != nil {
		t.Fatalf("set key2: %v", err)
	}

	if err := client.Clear(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}

	_, err := client.Get(ctx, key1)
	if !errors.Is(err, kvclient.ErrNotFound) {
		t.Fatalf("get key1 after clear: got %v, want ErrNotFound", err)
	}

	_, err = client.Get(ctx, key2)
	if !errors.Is(err, kvclient.ErrNotFound) {
		t.Fatalf("get key2 after clear: got %v, want ErrNotFound", err)
	}
}

func TestNewFromEnvUsesKVAddr(t *testing.T) {
	if err := db.LoadEnv(); err != nil {
		t.Fatalf("load env: %v", err)
	}

	client, err := kvclient.NewFromEnv()
	if err != nil {
		t.Fatalf("new from env: %v", err)
	}

	want := os.Getenv("KV_ADDR")
	if want == "" {
		want = "localhost:55555"
	}
	if client.Addr() != want {
		t.Fatalf("addr = %q, want %q", client.Addr(), want)
	}
}
