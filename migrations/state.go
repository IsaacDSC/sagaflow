package main

import "database/sql"

type Migration struct {
	Version int
	Up      func(db *sql.Tx) error
	Down    func(db *sql.Tx) error
}
