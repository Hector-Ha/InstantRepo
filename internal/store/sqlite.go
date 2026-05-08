package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"instantrepo/internal/domain"

	_ "modernc.org/sqlite"
)

const currentSchemaVersion = 1

type SQLiteStore struct {
	db *sql.DB
}

func DefaultDatabasePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(configDir, "InstantRepo", "instantrepo.db"), nil
}

func OpenDefaultSQLiteStore() (*SQLiteStore, error) {
	path, err := DefaultDatabasePath()
	if err != nil {
		return nil, err
	}
	return OpenSQLiteStore(path)
}

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) SaveInstalledRepo(ctx context.Context, repo domain.InstalledRepo) (domain.InstalledRepo, error) {
	if repo.LocalPath == "" {
		return domain.InstalledRepo{}, fmt.Errorf("local path is required")
	}
	if repo.Status == "" {
		repo.Status = domain.InstalledRepoStatusAnalyzed
	}
	if repo.LastAnalyzedAt.IsZero() {
		repo.LastAnalyzedAt = time.Now().UTC()
	}

	now := time.Now().UTC()
	id, err := s.installedRepoID(ctx, repo.NormalizedURL, repo.LocalPath)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("find installed repo: %w", err)
	}
	if id != 0 {
		return s.updateInstalledRepo(ctx, id, repo, now)
	}
	return s.insertInstalledRepo(ctx, repo, now)
}

func (s *SQLiteStore) insertInstalledRepo(ctx context.Context, repo domain.InstalledRepo, now time.Time) (domain.InstalledRepo, error) {
	row := s.db.QueryRowContext(ctx, `
INSERT INTO installed_repos (
	raw_url,
	normalized_url,
	local_path,
	status,
	created_at,
	updated_at,
	last_analyzed_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, raw_url, normalized_url, local_path, status, created_at, updated_at, last_analyzed_at;
`,
		nullString(repo.RawURL),
		nullString(repo.NormalizedURL),
		repo.LocalPath,
		repo.Status,
		formatTime(now),
		formatTime(now),
		formatTime(repo.LastAnalyzedAt),
	)

	saved, err := scanInstalledRepo(row)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("insert installed repo: %w", err)
	}
	return saved, nil
}

func (s *SQLiteStore) updateInstalledRepo(ctx context.Context, id int64, repo domain.InstalledRepo, now time.Time) (domain.InstalledRepo, error) {
	row := s.db.QueryRowContext(ctx, `
UPDATE installed_repos
SET raw_url = ?,
	normalized_url = ?,
	local_path = ?,
	status = ?,
	updated_at = ?,
	last_analyzed_at = ?
WHERE id = ?
RETURNING id, raw_url, normalized_url, local_path, status, created_at, updated_at, last_analyzed_at;
`,
		nullString(repo.RawURL),
		nullString(repo.NormalizedURL),
		repo.LocalPath,
		repo.Status,
		formatTime(now),
		formatTime(repo.LastAnalyzedAt),
		id,
	)

	saved, err := scanInstalledRepo(row)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("update installed repo: %w", err)
	}
	return saved, nil
}

func (s *SQLiteStore) installedRepoID(ctx context.Context, normalizedURL, localPath string) (int64, error) {
	var row *sql.Row
	if normalizedURL != "" {
		row = s.db.QueryRowContext(ctx, `
SELECT id
FROM installed_repos
WHERE normalized_url = ? OR local_path = ?
ORDER BY CASE WHEN normalized_url = ? THEN 0 ELSE 1 END
LIMIT 1;
`, normalizedURL, localPath, normalizedURL)
	} else {
		row = s.db.QueryRowContext(ctx, `
SELECT id
FROM installed_repos
WHERE local_path = ?
LIMIT 1;
`, localPath)
	}

	var id int64
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return id, nil
}

func (s *SQLiteStore) InstalledRepoByLocalPath(ctx context.Context, localPath string) (domain.InstalledRepo, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, raw_url, normalized_url, local_path, status, created_at, updated_at, last_analyzed_at
FROM installed_repos
WHERE local_path = ?;
`, localPath)

	repo, err := scanInstalledRepo(row)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("find installed repo by local path: %w", err)
	}
	return repo, nil
}

func (s *SQLiteStore) InstalledRepoByNormalizedURL(ctx context.Context, normalizedURL string) (domain.InstalledRepo, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, raw_url, normalized_url, local_path, status, created_at, updated_at, last_analyzed_at
FROM installed_repos
WHERE normalized_url = ?;
`, normalizedURL)

	repo, err := scanInstalledRepo(row)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("find installed repo by normalized URL: %w", err)
	}
	return repo, nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS installed_repos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			raw_url TEXT,
			normalized_url TEXT,
			local_path TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_analyzed_at TEXT NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS installed_repos_normalized_url_unique
			ON installed_repos(normalized_url)
			WHERE normalized_url IS NOT NULL AND normalized_url != '';`,
		`CREATE TABLE IF NOT EXISTS setup_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			installed_repo_id INTEGER,
			repo_path TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(installed_repo_id) REFERENCES installed_repos(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS step_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			setup_session_id INTEGER NOT NULL,
			step_id TEXT NOT NULL,
			title TEXT NOT NULL,
			command TEXT NOT NULL,
			cwd TEXT NOT NULL,
			status TEXT NOT NULL,
			exit_code INTEGER,
			log_path TEXT,
			started_at TEXT,
			finished_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(setup_session_id) REFERENCES setup_sessions(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply migration %d: %w", currentSchemaVersion, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

type installedRepoScanner interface {
	Scan(dest ...any) error
}

func scanInstalledRepo(row installedRepoScanner) (domain.InstalledRepo, error) {
	var repo domain.InstalledRepo
	var rawURL sql.NullString
	var normalizedURL sql.NullString
	var createdAt string
	var updatedAt string
	var lastAnalyzedAt string

	if err := row.Scan(
		&repo.ID,
		&rawURL,
		&normalizedURL,
		&repo.LocalPath,
		&repo.Status,
		&createdAt,
		&updatedAt,
		&lastAnalyzedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.InstalledRepo{}, err
		}
		return domain.InstalledRepo{}, err
	}

	repo.RawURL = rawURL.String
	repo.NormalizedURL = normalizedURL.String

	var err error
	if repo.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("parse created_at: %w", err)
	}
	if repo.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("parse updated_at: %w", err)
	}
	if repo.LastAnalyzedAt, err = parseTime(lastAnalyzedAt); err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("parse last_analyzed_at: %w", err)
	}

	return repo, nil
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}
