package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/IsaacDSC/migrations"
	_ "github.com/lib/pq"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		panic("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", url)
	if err != nil {
		panic(err)
	}

	defer db.Close()

	if err := db.Ping(); err != nil {
		panic(fmt.Errorf("database ping: %w", err))
	}

	migrations.Start(db)
}
