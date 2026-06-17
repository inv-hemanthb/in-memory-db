package db_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/db"
)

func openTestStore(t *testing.T) (*apidb.Store, *sql.DB) {
	t.Helper()

	if os.Getenv("DATABASE_URL") == "" {
		if err := db.LoadEnv(); err != nil {
			t.Fatalf("load env: %v", err)
		}
	}
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is not set")
	}

	conn, err := db.Open()
	if err != nil {
		t.Skipf("open db: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return apidb.NewStore(conn), conn
}

func testKey(t *testing.T, suffix string) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	return "TEST-" + name + "-" + suffix
}

func TestCreateAndGet(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	key := testKey(t, "create")

	created, err := store.Create(ctx, key, "alpha")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_ = store.HardDelete(context.Background(), created.ID)
	})

	if created.Key != key || created.Value != "alpha" || created.DeletedAt != nil {
		t.Fatalf("unexpected created item: %+v", created)
	}

	byID, err := store.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.Value != "alpha" {
		t.Fatalf("get by id value = %q", byID.Value)
	}

	byKey, err := store.GetByKey(ctx, key)
	if err != nil {
		t.Fatalf("get by key: %v", err)
	}
	if byKey.ID != created.ID {
		t.Fatalf("get by key id = %d, want %d", byKey.ID, created.ID)
	}

	byBoth, err := store.GetByIDAndKey(ctx, created.ID, key)
	if err != nil {
		t.Fatalf("get by id and key: %v", err)
	}
	if byBoth.Value != "alpha" {
		t.Fatalf("get by id and key value = %q", byBoth.Value)
	}
}

func TestUpdate(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	key := testKey(t, "update")

	created, err := store.Create(ctx, key, "before")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_ = store.HardDelete(context.Background(), created.ID)
	})

	updated, err := store.Update(ctx, created.ID, "after")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Value != "after" {
		t.Fatalf("value = %q, want after", updated.Value)
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Fatalf("updated_at went backwards: before=%v after=%v", created.UpdatedAt, updated.UpdatedAt)
	}
}

func TestCreateDuplicateKey(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	key := testKey(t, "dup")

	first, err := store.Create(ctx, key, "one")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	t.Cleanup(func() {
		_ = store.HardDelete(context.Background(), first.ID)
	})

	_, err = store.Create(ctx, key, "two")
	if !errors.Is(err, apidb.ErrDuplicateKey) {
		t.Fatalf("create duplicate: got %v, want ErrDuplicateKey", err)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()

	_, err := store.GetByID(ctx, 999999999)
	if !errors.Is(err, apidb.ErrNotFound) {
		t.Fatalf("get missing: got %v, want ErrNotFound", err)
	}
}

func TestSoftDelete(t *testing.T) {
	store, conn := openTestStore(t)
	ctx := context.Background()
	key := testKey(t, "soft")

	created, err := store.Create(ctx, key, "gone")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_ = store.HardDelete(context.Background(), created.ID)
	})

	if err := store.SoftDelete(ctx, created.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	_, err = store.GetByID(ctx, created.ID)
	if !errors.Is(err, apidb.ErrNotFound) {
		t.Fatalf("get after soft delete: got %v, want ErrNotFound", err)
	}

	var deletedAt sql.NullTime
	err = conn.QueryRowContext(ctx, `
		SELECT deleted_at FROM items WHERE id = $1`,
		created.ID,
	).Scan(&deletedAt)
	if err != nil {
		t.Fatalf("query deleted_at: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatal("deleted_at is null after soft delete")
	}
}

func TestSoftDeleteThenReuseKey(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	key := testKey(t, "reuse")

	first, err := store.Create(ctx, key, "first")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	if err := store.SoftDelete(ctx, first.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	second, err := store.Create(ctx, key, "second")
	if err != nil {
		t.Fatalf("create second with same key: %v", err)
	}
	t.Cleanup(func() {
		_ = store.HardDelete(context.Background(), first.ID)
		_ = store.HardDelete(context.Background(), second.ID)
	})

	if second.ID == first.ID {
		t.Fatalf("reused row id %d", second.ID)
	}

	got, err := store.GetByKey(ctx, key)
	if err != nil {
		t.Fatalf("get by key: %v", err)
	}
	if got.ID != second.ID || got.Value != "second" {
		t.Fatalf("unexpected live row: %+v", got)
	}
}

func TestHardDelete(t *testing.T) {
	store, conn := openTestStore(t)
	ctx := context.Background()
	key := testKey(t, "hard")

	created, err := store.Create(ctx, key, "remove")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := store.HardDelete(ctx, created.ID); err != nil {
		t.Fatalf("hard delete: %v", err)
	}

	var count int
	err = conn.QueryRowContext(ctx, `SELECT count(*) FROM items WHERE id = $1`, created.ID).Scan(&count)
	if err != nil {
		t.Fatalf("count row: %v", err)
	}
	if count != 0 {
		t.Fatalf("row still exists after hard delete")
	}

	_, err = store.GetByID(ctx, created.ID)
	if !errors.Is(err, apidb.ErrNotFound) {
		t.Fatalf("get after hard delete: got %v, want ErrNotFound", err)
	}
}

func TestSoftDeleteNotFound(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()

	err := store.SoftDelete(ctx, 999999999)
	if !errors.Is(err, apidb.ErrNotFound) {
		t.Fatalf("soft delete missing: got %v, want ErrNotFound", err)
	}
}

func TestHardDeleteNotFound(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()

	err := store.HardDelete(ctx, 999999999)
	if !errors.Is(err, apidb.ErrNotFound) {
		t.Fatalf("hard delete missing: got %v, want ErrNotFound", err)
	}
}

func TestUpdateNotFound(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()

	_, err := store.Update(ctx, 999999999, "nope")
	if !errors.Is(err, apidb.ErrNotFound) {
		t.Fatalf("update missing: got %v, want ErrNotFound", err)
	}
}

func TestCreateSetsTimestamps(t *testing.T) {
	store, _ := openTestStore(t)
	ctx := context.Background()
	key := testKey(t, "ts")

	before := time.Now().Add(-time.Second)
	created, err := store.Create(ctx, key, "ts")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_ = store.HardDelete(context.Background(), created.ID)
	})
	after := time.Now().Add(time.Second)

	if created.CreatedAt.Before(before) || created.CreatedAt.After(after) {
		t.Fatalf("created_at out of range: %v", created.CreatedAt)
	}
	if created.UpdatedAt.Before(before) || created.UpdatedAt.After(after) {
		t.Fatalf("updated_at out of range: %v", created.UpdatedAt)
	}
}
