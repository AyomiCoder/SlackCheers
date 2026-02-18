package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type migrationFile struct {
	Version  int64
	Name     string
	UpPath   string
	DownPath string
}

func UpMigrations(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}

	migrations, err := loadMigrations(migrationsDir)
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		content, err := os.ReadFile(m.UpPath)
		if err != nil {
			return fmt.Errorf("read up migration %s: %w", m.UpPath, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", m.Version, err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply up migration %d: %w", m.Version, err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}

	return nil
}

func DownOneMigration(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return err
	}

	migrations, err := loadMigrations(migrationsDir)
	if err != nil {
		return err
	}

	version, err := currentVersion(ctx, db)
	if err != nil {
		return err
	}
	if version == 0 {
		return nil
	}

	var target *migrationFile
	for i := range migrations {
		if migrations[i].Version == version {
			target = &migrations[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("down migration file not found for version %d", version)
	}
	if target.DownPath == "" {
		return fmt.Errorf("down migration missing for version %d", version)
	}

	content, err := os.ReadFile(target.DownPath)
	if err != nil {
		return fmt.Errorf("read down migration %s: %w", target.DownPath, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for down migration %d: %w", target.Version, err)
	}

	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply down migration %d: %w", target.Version, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, target.Version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete migration record %d: %w", target.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit down migration %d: %w", target.Version, err)
	}

	return nil
}

func MigrationStatus(ctx context.Context, db *sql.DB, migrationsDir string) (string, error) {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return "", err
	}

	migrations, err := loadMigrations(migrationsDir)
	if err != nil {
		return "", err
	}

	version, err := currentVersion(ctx, db)
	if err != nil {
		return "", err
	}

	latest := int64(0)
	if len(migrations) > 0 {
		latest = migrations[len(migrations)-1].Version
	}

	return fmt.Sprintf("current=%d latest=%d", version, latest), nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	const q = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`
	if _, err := db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func currentVersion(ctx context.Context, db *sql.DB) (int64, error) {
	const q = `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`
	var version int64
	if err := db.QueryRowContext(ctx, q).Scan(&version); err != nil {
		return 0, fmt.Errorf("read current migration version: %w", err)
	}
	return version, nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan applied version: %w", err)
		}
		out[v] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied versions: %w", err)
	}

	return out, nil
}

func loadMigrations(migrationsDir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", migrationsDir, err)
	}

	byVersion := make(map[int64]migrationFile)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		parts := strings.Split(filename, "_")
		if len(parts) < 2 {
			continue
		}

		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}

		if strings.HasSuffix(filename, ".up.sql") {
			m := byVersion[version]
			m.Version = version
			m.Name = filename
			m.UpPath = filepath.Join(migrationsDir, filename)
			byVersion[version] = m
		}
		if strings.HasSuffix(filename, ".down.sql") {
			m := byVersion[version]
			m.Version = version
			m.Name = filename
			m.DownPath = filepath.Join(migrationsDir, filename)
			byVersion[version] = m
		}
	}

	migrations := make([]migrationFile, 0, len(byVersion))
	for _, m := range byVersion {
		if m.UpPath == "" {
			return nil, fmt.Errorf("missing up migration for version %d", m.Version)
		}
		migrations = append(migrations, m)
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })
	return migrations, nil
}
