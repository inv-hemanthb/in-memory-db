package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/inv-hemanthb/in-memory-db/internal/db"
)

const bootstrapSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

func main() {
	if err := db.LoadEnv(); err != nil {
		log.Fatalf("load env: %v", err)
	}

	conn, err := db.Open()
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(bootstrapSQL); err != nil {
		log.Fatalf("bootstrap schema_migrations: %v", err)
	}

	root, err := db.RepoRoot()
	if err != nil {
		log.Fatalf("repo root: %v", err)
	}

	migrationsDir := filepath.Join(root, "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatalf("read migrations dir: %v", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	if len(files) == 0 {
		log.Println("no migrations found")
		return
	}

	for _, name := range files {
		if err := applyMigration(conn, migrationsDir, name); err != nil {
			log.Fatalf("migration %s: %v", name, err)
		}
	}
}

func applyMigration(conn *sql.DB, dir, name string) error {
	applied, err := isApplied(conn, name)
	if err != nil {
		return err
	}
	if applied {
		log.Printf("skip %s (already applied)", name)
		return nil
	}

	sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return err
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(string(sqlBytes)); err != nil {
		return err
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("applied %s", name)
	return nil
}

func isApplied(conn *sql.DB, version string) (bool, error) {
	var exists bool
	err := conn.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
		version,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}
