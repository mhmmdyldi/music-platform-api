package database

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Migrator applies the versioned SQL migrations in a filesystem (normally the
// embedded db/migrations) to one database.
type Migrator struct {
	provider *goose.Provider
	closeDB  func() error
}

// NewMigrator prepares migrations from fsys against the database behind pool.
func NewMigrator(pool *pgxpool.Pool, fsys fs.FS) (*Migrator, error) {
	db := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return &Migrator{provider: provider, closeDB: db.Close}, nil
}

// Close releases the migrator's connection. It does not close the pool.
func (m *Migrator) Close() error {
	return m.closeDB()
}

// Up applies every pending migration and returns the versions applied.
func (m *Migrator) Up(ctx context.Context) ([]int64, error) {
	results, err := m.provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate up: %w", err)
	}
	return versions(results), nil
}

// DownOne rolls back the most recently applied migration.
func (m *Migrator) DownOne(ctx context.Context) (int64, error) {
	result, err := m.provider.Down(ctx)
	if err != nil {
		return 0, fmt.Errorf("migrate down: %w", err)
	}
	return result.Source.Version, nil
}

// DownAll rolls back every applied migration.
func (m *Migrator) DownAll(ctx context.Context) ([]int64, error) {
	results, err := m.provider.DownTo(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("migrate down to 0: %w", err)
	}
	return versions(results), nil
}

// MigrationStatus is one migration and whether it is applied.
type MigrationStatus struct {
	Version int64
	File    string
	Applied bool
}

// Status lists every known migration in version order.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	statuses, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration status: %w", err)
	}
	out := make([]MigrationStatus, len(statuses))
	for i, s := range statuses {
		out[i] = MigrationStatus{
			Version: s.Source.Version,
			File:    s.Source.Path,
			Applied: s.State == goose.StateApplied,
		}
	}
	return out, nil
}

// Pending reports how many migrations are not yet applied.
func (m *Migrator) Pending(ctx context.Context) (int, error) {
	statuses, err := m.Status(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range statuses {
		if !s.Applied {
			n++
		}
	}
	return n, nil
}

func versions(results []*goose.MigrationResult) []int64 {
	out := make([]int64, len(results))
	for i, r := range results {
		out[i] = r.Source.Version
	}
	return out
}
