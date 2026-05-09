package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"instantrepo/internal/domain"

	_ "modernc.org/sqlite"
)

const currentSchemaVersion = 2
const maxSetupSessionsPerRepo = 10

const setupLogRetention = 7 * 24 * time.Hour

type SQLiteStore struct {
	db     *sql.DB
	logDir string
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

	store := &SQLiteStore{
		db:     db,
		logDir: filepath.Join(filepath.Dir(path), "setup-logs"),
	}
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

func (s *SQLiteStore) InstalledRepoByID(ctx context.Context, id int64) (domain.InstalledRepo, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, raw_url, normalized_url, local_path, status, created_at, updated_at, last_analyzed_at
FROM installed_repos
WHERE id = ?;
`, id)

	repo, err := scanInstalledRepo(row)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("find installed repo by ID: %w", err)
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

func (s *SQLiteStore) StartSetupSession(ctx context.Context, installedRepoID int64, repoPath string) (domain.SetupSession, error) {
	if repoPath == "" {
		return domain.SetupSession{}, fmt.Errorf("repo path is required")
	}

	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, `
INSERT INTO setup_sessions (
	installed_repo_id,
	repo_path,
	status,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?)
RETURNING id, installed_repo_id, repo_path, status, created_at, updated_at;
`,
		installedRepoID,
		repoPath,
		domain.SetupSessionStatusRunning,
		formatTime(now),
		formatTime(now),
	)

	session, err := scanSetupSession(row)
	if err != nil {
		return domain.SetupSession{}, fmt.Errorf("insert setup session: %w", err)
	}
	return session, nil
}

func (s *SQLiteStore) SetupSessionsByInstalledRepoID(ctx context.Context, installedRepoID int64) ([]domain.SetupSession, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, installed_repo_id, repo_path, status, created_at, updated_at
FROM setup_sessions
WHERE installed_repo_id = ?
ORDER BY created_at DESC, id DESC;
`, installedRepoID)
	if err != nil {
		return nil, fmt.Errorf("query setup sessions by installed repo ID: %w", err)
	}
	defer rows.Close()

	var sessions []domain.SetupSession
	for rows.Next() {
		session, err := scanSetupSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan setup session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read setup sessions: %w", err)
	}
	return sessions, nil
}

func (s *SQLiteStore) StepRunsBySetupSessionID(ctx context.Context, setupSessionID int64) ([]domain.StepRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, setup_session_id, step_id, title, command_hash, command_preview, cwd, status, exit_code, duration, log_path, started_at, finished_at, created_at, updated_at
FROM step_runs
WHERE setup_session_id = ?
ORDER BY created_at ASC, id ASC;
`, setupSessionID)
	if err != nil {
		return nil, fmt.Errorf("query step runs by setup session ID: %w", err)
	}
	defer rows.Close()

	var runs []domain.StepRun
	for rows.Next() {
		run, err := scanStepRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan step run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read step runs: %w", err)
	}
	return runs, nil
}

func (s *SQLiteStore) RecordStepRun(ctx context.Context, run domain.StepRun, logContent string) (domain.StepRun, error) {
	if run.SetupSessionID == 0 {
		return domain.StepRun{}, fmt.Errorf("setup session ID is required")
	}
	if run.StepID == "" {
		return domain.StepRun{}, fmt.Errorf("step ID is required")
	}
	if run.Status == "" {
		return domain.StepRun{}, fmt.Errorf("step run status is required")
	}

	logPath := ""
	if logContent != "" {
		var err error
		logPath, err = s.writeStepLog(run.SetupSessionID, run.StepID, logContent)
		if err != nil {
			return domain.StepRun{}, err
		}
	}

	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, `
INSERT INTO step_runs (
	setup_session_id,
	step_id,
	title,
	command,
	command_hash,
	command_preview,
	cwd,
	status,
	exit_code,
	log_path,
	duration,
	started_at,
	finished_at,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, log_path, created_at, updated_at;
`,
		run.SetupSessionID,
		run.StepID,
		run.Title,
		run.CommandPreview,
		run.CommandHash,
		run.CommandPreview,
		run.Cwd,
		run.Status,
		run.ExitCode,
		nullString(logPath),
		run.Duration,
		formatTime(run.StartedAt),
		formatTime(run.FinishedAt),
		formatTime(now),
		formatTime(now),
	)

	var saved domain.StepRun
	var savedLogPath sql.NullString
	var createdAt string
	var updatedAt string
	if err := row.Scan(&saved.ID, &savedLogPath, &createdAt, &updatedAt); err != nil {
		return domain.StepRun{}, fmt.Errorf("insert step run: %w", err)
	}
	run.ID = saved.ID
	run.LogPath = savedLogPath.String
	var err error
	if run.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.StepRun{}, fmt.Errorf("parse step run created_at: %w", err)
	}
	if run.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.StepRun{}, fmt.Errorf("parse step run updated_at: %w", err)
	}

	if err := s.updateSetupSessionStatus(ctx, run.SetupSessionID, run.Status, now); err != nil {
		return domain.StepRun{}, err
	}
	return run, nil
}

func (s *SQLiteStore) CleanupSetupSessionRetention(ctx context.Context, now time.Time) error {
	sessions, err := s.setupSessionsForRetention(ctx)
	if err != nil {
		return err
	}

	cutoff := now.UTC().Add(-setupLogRetention)
	keptByRepo := map[string]int{}
	deleteIDs := map[int64]bool{}
	for _, session := range sessions {
		if session.createdAt.Before(cutoff) {
			deleteIDs[session.id] = true
			continue
		}

		keptByRepo[session.repoKey]++
		if keptByRepo[session.repoKey] > maxSetupSessionsPerRepo {
			deleteIDs[session.id] = true
		}
	}
	if len(deleteIDs) == 0 {
		return nil
	}

	logPaths, err := s.logPathsForSessions(ctx, deleteIDs)
	if err != nil {
		return err
	}
	for _, logPath := range logPaths {
		if err := os.Remove(logPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("delete setup log %s: %w", logPath, err)
		}
		_ = os.Remove(filepath.Dir(logPath))
	}
	if err := s.deleteSetupSessions(ctx, deleteIDs); err != nil {
		return err
	}
	return nil
}

type setupSessionRetentionRow struct {
	id        int64
	repoKey   string
	createdAt time.Time
}

func (s *SQLiteStore) setupSessionsForRetention(ctx context.Context) ([]setupSessionRetentionRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, installed_repo_id, repo_path, created_at
FROM setup_sessions
ORDER BY created_at DESC, id DESC;
`)
	if err != nil {
		return nil, fmt.Errorf("query setup sessions for retention: %w", err)
	}
	defer rows.Close()

	var sessions []setupSessionRetentionRow
	for rows.Next() {
		var id int64
		var installedRepoID sql.NullInt64
		var repoPath string
		var createdAtRaw string
		if err := rows.Scan(&id, &installedRepoID, &repoPath, &createdAtRaw); err != nil {
			return nil, fmt.Errorf("scan setup session for retention: %w", err)
		}
		createdAt, err := parseTime(createdAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse setup session created_at: %w", err)
		}
		repoKey := "path:" + repoPath
		if installedRepoID.Valid && installedRepoID.Int64 > 0 {
			repoKey = fmt.Sprintf("repo:%d", installedRepoID.Int64)
		}
		sessions = append(sessions, setupSessionRetentionRow{
			id:        id,
			repoKey:   repoKey,
			createdAt: createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read setup sessions for retention: %w", err)
	}
	return sessions, nil
}

func (s *SQLiteStore) logPathsForSessions(ctx context.Context, sessionIDs map[int64]bool) ([]string, error) {
	var paths []string
	for sessionID := range sessionIDs {
		rows, err := s.db.QueryContext(ctx, `
SELECT log_path
FROM step_runs
WHERE setup_session_id = ?
	AND log_path IS NOT NULL
	AND log_path != '';
`, sessionID)
		if err != nil {
			return nil, fmt.Errorf("query setup logs for session %d: %w", sessionID, err)
		}
		for rows.Next() {
			var logPath string
			if err := rows.Scan(&logPath); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan setup log path: %w", err)
			}
			paths = append(paths, logPath)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("read setup log paths: %w", err)
		}
		rows.Close()
	}
	return paths, nil
}

func (s *SQLiteStore) deleteSetupSessions(ctx context.Context, sessionIDs map[int64]bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin setup session cleanup: %w", err)
	}
	defer tx.Rollback()

	for sessionID := range sessionIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM step_runs WHERE setup_session_id = ?;`, sessionID); err != nil {
			return fmt.Errorf("delete step runs for setup session %d: %w", sessionID, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM setup_sessions WHERE id = ?;`, sessionID); err != nil {
			return fmt.Errorf("delete setup session %d: %w", sessionID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit setup session cleanup: %w", err)
	}
	return nil
}

func (s *SQLiteStore) writeStepLog(setupSessionID int64, stepID, logContent string) (string, error) {
	if s.logDir == "" {
		return "", fmt.Errorf("setup log directory is not configured")
	}

	sessionDir := filepath.Join(s.logDir, fmt.Sprintf("session-%d", setupSessionID))
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return "", fmt.Errorf("create setup log directory: %w", err)
	}

	logPath := filepath.Join(sessionDir, fmt.Sprintf("%d-%s.log", time.Now().UTC().UnixNano(), sanitizeLogName(stepID)))
	if err := os.WriteFile(logPath, []byte(logContent), 0o600); err != nil {
		return "", fmt.Errorf("write setup step log: %w", err)
	}
	return logPath, nil
}

func (s *SQLiteStore) updateSetupSessionStatus(ctx context.Context, setupSessionID int64, stepStatus string, now time.Time) error {
	status := domain.SetupSessionStatusSucceeded
	if stepStatus == domain.StepRunStatusFailed {
		status = domain.SetupSessionStatusFailed
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE setup_sessions
SET status = ?, updated_at = ?
WHERE id = ?;
`, status, formatTime(now), setupSessionID); err != nil {
		return fmt.Errorf("update setup session status: %w", err)
	}
	return nil
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
			command_hash TEXT NOT NULL DEFAULT '',
			command_preview TEXT NOT NULL DEFAULT '',
			cwd TEXT NOT NULL,
			status TEXT NOT NULL,
			exit_code INTEGER,
			log_path TEXT,
			duration TEXT NOT NULL DEFAULT '',
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

	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "command_hash", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "command_preview", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "duration", definition: "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(ctx, tx, "step_runs", column.name, column.definition); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (2, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));`); err != nil {
		return fmt.Errorf("record migration 2: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

type installedRepoScanner interface {
	Scan(dest ...any) error
}

type setupSessionScanner interface {
	Scan(dest ...any) error
}

type stepRunScanner interface {
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

func scanSetupSession(row setupSessionScanner) (domain.SetupSession, error) {
	var session domain.SetupSession
	var installedRepoID sql.NullInt64
	var createdAt string
	var updatedAt string

	if err := row.Scan(
		&session.ID,
		&installedRepoID,
		&session.RepoPath,
		&session.Status,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.SetupSession{}, err
	}

	session.InstalledRepoID = installedRepoID.Int64
	var err error
	if session.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.SetupSession{}, fmt.Errorf("parse created_at: %w", err)
	}
	if session.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.SetupSession{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return session, nil
}

func scanStepRun(row stepRunScanner) (domain.StepRun, error) {
	var run domain.StepRun
	var exitCode sql.NullInt64
	var duration sql.NullString
	var logPath sql.NullString
	var startedAt sql.NullString
	var finishedAt sql.NullString
	var createdAt string
	var updatedAt string

	if err := row.Scan(
		&run.ID,
		&run.SetupSessionID,
		&run.StepID,
		&run.Title,
		&run.CommandHash,
		&run.CommandPreview,
		&run.Cwd,
		&run.Status,
		&exitCode,
		&duration,
		&logPath,
		&startedAt,
		&finishedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.StepRun{}, err
	}

	run.ExitCode = int(exitCode.Int64)
	run.Duration = duration.String
	run.LogPath = logPath.String

	var err error
	if startedAt.Valid && startedAt.String != "" {
		if run.StartedAt, err = parseTime(startedAt.String); err != nil {
			return domain.StepRun{}, fmt.Errorf("parse step run started_at: %w", err)
		}
	}
	if finishedAt.Valid && finishedAt.String != "" {
		if run.FinishedAt, err = parseTime(finishedAt.String); err != nil {
			return domain.StepRun{}, fmt.Errorf("parse step run finished_at: %w", err)
		}
	}
	if run.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.StepRun{}, fmt.Errorf("parse step run created_at: %w", err)
	}
	if run.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.StepRun{}, fmt.Errorf("parse step run updated_at: %w", err)
	}

	return run, nil
}

func ensureColumn(ctx context.Context, tx *sql.Tx, tableName, columnName, definition string) error {
	exists, err := columnExists(ctx, tx, tableName, columnName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", tableName, columnName, definition)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", tableName, columnName, err)
	}
	return nil
}

func columnExists(ctx context.Context, tx *sql.Tx, tableName, columnName string) (bool, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s);", tableName))
	if err != nil {
		return false, fmt.Errorf("inspect table %s: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan table %s column: %w", tableName, err)
		}
		if name == columnName {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("read table %s columns: %w", tableName, err)
	}
	return false, nil
}

func sanitizeLogName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "step"
	}

	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "step"
	}
	return result
}
