// Migrations: run the whole package so state.go and v1_*.go are compiled.
//
//	go run ./migrations/ up
//	go run ./migrations/ down
//	go run ./migrations/ new <filename>
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"

	_ "github.com/lib/pq"
)

var state []Migration

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run ./migrations/ up|down|new [filename]")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// create table migrations if not exists
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS migrations (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), version INT NOT NULL, rollback BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)")
	if err != nil {
		log.Fatal(err)
	}

	// SELECT last version for insert
	var dbVersion int
	err = db.QueryRow("SELECT version FROM migrations WHERE rollback = FALSE ORDER BY version DESC LIMIT 1").Scan(&dbVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Fatal(err)
	}

	cmd := os.Args[1]

	switch cmd {
	case "new":
		filename := os.Args[2]
		if filename == "" {
			log.Fatal("filename is required, use: go run main.go new <filename>")
		}

		version := len(state) + 1
		path := fmt.Sprintf("migrations/v%d_%s.go", version, filename)
		content := `
		package main

		import "database/sql"

		func init() {
			state = append(state, Migration{
				Version: 1,
				Up: func(db *sql.Tx) error {
					return nil
				},
				Down: func(db *sql.Tx) error {
					return nil
				},
			})
		}
		`
		os.WriteFile(path, []byte(content), 0644)

		// execute go fmt ./migrations/...
		exec.Command("go", "fmt", "./migrations/...").Run()

		log.Printf("Migration %d created at %s", version, path)

	case "up":
		if len(state[dbVersion:]) == 0 {
			log.Println("No migrations to apply")
			return
		}

		// order by version 1,2,3...
		sort.Slice(state[dbVersion:], func(i, j int) bool {
			return state[dbVersion:][i].Version < state[dbVersion:][j].Version
		})

		for _, migration := range state[dbVersion:] {
			tx, err := db.Begin()
			if err != nil {
				log.Fatal(err)
			}

			defer tx.Rollback()

			if err := migration.Up(tx); err != nil {
				log.Fatal(err)
			}

			_, err = tx.Exec("INSERT INTO migrations (version) VALUES ($1)", migration.Version)
			if err != nil {
				log.Fatal(err)
			}

			if err := tx.Commit(); err != nil {
				log.Fatal(err)
			}

			log.Printf("Migration %d applied", migration.Version)
		}
	case "down":
		// down somente da ultima migration aplicada
		if dbVersion == 0 {
			log.Println("No migrations to revert")
			return
		}
		if dbVersion > len(state) {
			log.Fatalf("db version %d exceeds known migrations (%d)", dbVersion, len(state))
		}
		tx, err := db.Begin()
		if err != nil {
			log.Fatal(err)
		}
		defer tx.Rollback()

		// order by version 1,2,3...
		sort.Slice(state[dbVersion:], func(i, j int) bool {
			return state[dbVersion:][i].Version < state[dbVersion:][j].Version
		})

		migration := state[dbVersion-1]

		if migration.Version == 0 {
			log.Fatalf("migration %d not found", dbVersion)
		}

		if err := migration.Down(tx); err != nil {
			log.Fatal(err)
		}
		_, err = tx.Exec("UPDATE migrations SET rollback = true WHERE version = $1", migration.Version)
		if err != nil {
			log.Fatal(err)
		}

		if err := tx.Commit(); err != nil {
			log.Fatal(err)
		}

		log.Printf("Migration %d reverted", migration.Version)
	}
}
