package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const defaultDatabasePath = "database/database.db"

var DB *sql.DB

// Open opens the ElectriCredit SQLite database.
// Pass an empty path to use database/database.db relative to the server folder.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = defaultDatabasePath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// ElectriCredit is local-first and SQLite-backed.
	// A single writer avoids unnecessary lock contention.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA busy_timeout = 5000;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply sqlite pragma: %w", err)
		}
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	DB = db
	return db, nil
}

// Get returns the active database connection.
func Get() (*sql.DB, error) {
	if DB == nil {
		return nil, errors.New("database is not open")
	}
	return DB, nil
}

// Close safely closes the active database connection.
func Close() error {
	if DB == nil {
		return nil
	}

	err := DB.Close()
	DB = nil
	return err
}

// UnixTime returns a UTC Unix timestamp for durable records.
func UnixTime() int64 {
	return time.Now().UTC().Unix()
}
