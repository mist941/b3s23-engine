package store

import (
	"database/sql"
	_ "embed"
	"fmt"
)

var schemaSQL string

const schemaVersion = 1

func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("store: read user_version: %w", err)
	}
	if v >= schemaVersion {
		return nil
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("store: apply schema: %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("store: set user_version: %w", err)
	}
	return nil
}
