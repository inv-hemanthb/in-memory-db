package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/inv-hemanthb/in-memory-db/internal/db"
)

func main() {
	countFlag := flag.Int("count", 0, "number of rows to seed (overrides SEED_COUNT)")
	flag.Parse()

	if err := db.LoadEnv(); err != nil {
		log.Fatalf("load env: %v", err)
	}

	count := *countFlag
	if count == 0 {
		count = 500
		if v := os.Getenv("SEED_COUNT"); v != "" {
			parsed, err := strconv.Atoi(v)
			if err != nil {
				log.Fatalf("invalid SEED_COUNT: %v", err)
			}
			count = parsed
		}
	}

	if count <= 0 {
		log.Fatalf("seed count must be positive")
	}

	conn, err := db.Open()
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	var existing int
	if err := conn.QueryRow(
		`SELECT count(*) FROM items WHERE deleted_at IS NULL`,
	).Scan(&existing); err != nil {
		log.Fatalf("count items: %v", err)
	}

	if existing >= count {
		log.Printf("skip seed: %d live rows already exist (target %d)", existing, count)
		return
	}

	toInsert := count - existing
	tx, err := conn.Begin()
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	for i := 1; i <= toInsert; i++ {
		key := fmt.Sprintf("KEY-%06d", existing+i)
		value := fmt.Sprintf("seed-value-%06d", existing+i)
		if _, err := tx.Exec(
			`INSERT INTO items (key, value) VALUES ($1, $2)`,
			key,
			value,
		); err != nil {
			log.Fatalf("insert %s: %v", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}

	log.Printf("seeded %d rows (%d total live rows)", toInsert, count)
}
