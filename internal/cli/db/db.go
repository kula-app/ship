// Package db provides local SQLite database access for the CLI.
//
// The database is stored at ~/.ship/cli.db and is created automatically
// on first access. It uses the ent ORM with SQLite for type-safe queries.
package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	"github.com/kula-app/ship/ent"

	_ "modernc.org/sqlite"
)

var (
	instance *ent.Client
	once     sync.Once
	initErr  error
)

// GetDBPath returns the path to the CLI database file.
func GetDBPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".ship", "cli.db"), nil
}

// GetClient returns a singleton ent client backed by SQLite.
// The database file and schema are created on first call.
func GetClient() (*ent.Client, error) {
	once.Do(func() {
		dbPath, err := GetDBPath()
		if err != nil {
			initErr = err
			return
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			initErr = err
			return
		}

		// Use the pure-Go modernc.org/sqlite driver (registered as "sqlite")
		// so the CLI works in cgo-disabled, statically-linked release builds.
		// ent.Open cannot be used directly because it maps dialect.SQLite to the
		// cgo "sqlite3" driver name.
		conn, err := sql.Open("sqlite", "file:"+dbPath+"?cache=shared&_pragma=foreign_keys(1)")
		if err != nil {
			initErr = err
			return
		}

		client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, conn)))

		// Auto-migrate the schema (create tables if they don't exist)
		if err := client.Schema.Create(context.Background()); err != nil {
			client.Close()
			initErr = err
			return
		}

		instance = client
	})

	return instance, initErr
}

// CloseDB closes the database connection if open.
func CloseDB() error {
	if instance == nil {
		once = sync.Once{}
		initErr = nil
		return nil
	}

	err := instance.Close()
	instance = nil
	once = sync.Once{}
	initErr = nil
	return err
}
