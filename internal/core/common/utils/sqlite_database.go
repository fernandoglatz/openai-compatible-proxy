package utils

import (
	"context"
	"database/sql"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "modernc.org/sqlite"
)

var Database databaseType

type databaseType struct {
	Client *sql.DB
}

// OpenDatabase opens the SQLite file at path and applies every migration under
// migrationsPath. WAL keeps reads from blocking the sync writer; a single connection
// makes SQLITE_BUSY impossible at this workload (a handful of model rows per sync),
// which is cheaper than reasoning about retry behavior.
func OpenDatabase(ctx context.Context, path string, migrationsPath string) (*sql.DB, error) {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return nil, err
	}

	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}

	err = runMigrations(db, migrationsPath)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func runMigrations(db *sql.DB, migrationsPath string) error {
	driver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return err
	}

	migrations, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "sqlite", driver)
	if err != nil {
		return err
	}

	err = migrations.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

func ConnectToDatabase(ctx context.Context) error {
	path := config.ApplicationConfig.Data.SQLite.Path

	log.Info(ctx).Msg("Opening SQLite database at " + path)

	db, err := OpenDatabase(ctx, path, "scripts/sqlite/migrations")
	if err != nil {
		return err
	}

	log.Info(ctx).Msg("SQLite database ready!")

	Database = databaseType{Client: db}

	return nil
}
