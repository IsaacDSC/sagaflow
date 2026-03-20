package main

import (
	"database/sql"

	"github.com/IsaacDSC/migrations"
	"github.com/IsaacDSC/migrations/migration"
)

func init() {
	migrations.State = append(migrations.State, migration.Migrate{
		Version: 2,
		Up: func(db *sql.Tx) error {
			const query = `
				CREATE TABLE IF NOT EXISTS transactions (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					rule_id UUID NOT NULL,
					transaction_id UUID NOT NULL UNIQUE,
					config_rules JSONB NOT NULL,
					data JSONB NOT NULL,
					headers JSONB NOT NULL,
					status TEXT NOT NULL,
					error TEXT,
					retries INT NOT NULL DEFAULT 0,
					created_at TIMESTAMP NOT NULL,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					CONSTRAINT fk_rule FOREIGN KEY (rule_id) REFERENCES rules (id)
				)
			`
			_, err := db.Exec(query)
			return err
		},
		Down: func(db *sql.Tx) error {
			const query = `
				DROP TABLE IF EXISTS transactions
			`
			_, err := db.Exec(query)
			return err
		},
	})
}
