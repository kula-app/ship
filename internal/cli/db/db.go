// Package db provides local SQLite database access for the CLI.
//
// The database is stored at ~/.ship/cli.db and is created automatically
// on first access. It uses the ent ORM with SQLite for type-safe queries.
package db

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"entgo.io/ent/dialect"

	"github.com/kula-app/ship/ent"

	_ "github.com/mattn/go-sqlite3"
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

		client, err := ent.Open(dialect.SQLite, "file:"+dbPath+"?cache=shared&_fk=1")
		if err != nil {
			initErr = err
			return
		}

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
	if instance != nil {
		return instance.Close()
	}
	return nil
}
