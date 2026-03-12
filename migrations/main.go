package main

import (
	"database/sql"

	"github.com/IsaacDSC/migrations"
	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "postgres://idsc:admin@localhost:5432/sagaflow?sslmode=disable")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	migrations.Start(db)
}
