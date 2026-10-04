package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

const (
	migrationsDir      = "migrations"
	schemaVersionTable = "schema_version"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Connect opens a pool, verifies the connection and applies pending migrations.
// Call it once at startup; the caller owns the pool and closes it.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	err = runMigrations(ctx, pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return pool, nil
}

// runMigrations applies the embedded tern migrations on a single pooled connection.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	migrator, err := migrate.NewMigrator(ctx, conn.Conn(), schemaVersionTable)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	migrationsFS, err := fs.Sub(migrations, migrationsDir)
	if err != nil {
		return fmt.Errorf("open migrations dir: %w", err)
	}

	err = migrator.LoadMigrations(migrationsFS)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	return migrator.Migrate(ctx)
}
