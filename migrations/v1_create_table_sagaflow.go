package main

import (
	"database/sql"
	"fmt"

	"github.com/IsaacDSC/migrations"
	"github.com/IsaacDSC/migrations/migration"
)

func init() {
	migrations.State = append(migrations.State, migration.Migrate{
		Version: 1,
		Up: func(db *sql.Tx) error {
			const query = `CREATE TABLE IF NOT EXISTS rules (
					id UUID PRIMARY KEY,
					name TEXT NOT NULL UNIQUE,
					transactions JSONB NOT NULL,
					rollback JSONB NOT NULL,
					configs JSONB NOT NULL
				)`
			_, err := db.Exec(query)
			if err != nil {
				return fmt.Errorf("failed to create table: %w", err)
			}

			return nil
		},
		Down: func(db *sql.Tx) error {
			const query = `DROP TABLE IF EXISTS rules`
			_, err := db.Exec(query)
			if err != nil {
				return fmt.Errorf("failed to drop table: %w", err)
			}

			return nil
		},
	})
}
