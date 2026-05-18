package store

import (
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"instantrepo/internal/domain"

	_ "modernc.org/sqlite"
)

const currentSchemaVersion = 4
const maxSetupSessionsPerRepo = 10
const maxEnvContributionQueueItems = 100

const setupLogRetention = 7 * 24 * time.Hour
const envContributionQueueRetention = 30 * 24 * time.Hour
const envContributionSettingsKey = "env_pattern_contribution_settings"
const aiEnvReviewSettingsKey = "ai_env_review_settings"
const envVaultCredentialNamespaceKey = "env_vault_credential_namespace"
const envPortAssignmentKeyPrefix = "env_port_assignment:"

type SQLiteStore struct {
	db     *sql.DB
	logDir string
}

func DefaultDatabasePath() (string, error) {
	if appDataDir := appDataDirEnvValue(os.Environ()); strings.TrimSpace(appDataDir) != "" {
		return DatabasePathForAppDataDir(appDataDir)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(configDir, "InstantRepo", "instantrepo.db"), nil
}

func DatabasePathForAppDataDir(appDataDir string) (string, error) {
	appDataDir = strings.TrimSpace(appDataDir)
	if appDataDir == "" {
		return "", fmt.Errorf("app data dir is required")
	}
	if !filepath.IsAbs(appDataDir) {
		return "", fmt.Errorf("app data dir must be absolute")
	}
	cleanAppData, err := filepath.Abs(appDataDir)
	if err != nil {
		return "", fmt.Errorf("resolve app data dir: %w", err)
	}
	cleanAppData = filepath.Clean(cleanAppData)
	if filepath.Dir(cleanAppData) == cleanAppData {
		return "", fmt.Errorf("app data dir must not be filesystem root")
	}
	if volume := filepath.VolumeName(cleanAppData); volume != "" && strings.EqualFold(cleanAppData, volume+string(os.PathSeparator)) {
		return "", fmt.Errorf("app data dir must not be drive root")
	}
	homeDir, err := os.UserHomeDir()
	if err == nil && samePath(cleanAppData, homeDir) {
		return "", fmt.Errorf("app data dir must not be home dir")
	}
	if hasRepoRootMarker(cleanAppData) {
		return "", fmt.Errorf("app data dir must not be repo root")
	}
	return filepath.Join(cleanAppData, "instantrepo.db"), nil
}

func hasRepoRootMarker(path string) bool {
	for _, marker := range []string{".git", "go.mod"} {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	return false
}

func appDataDirEnvValue(environ []string) string {
	for _, item := range environ {
		name, value, ok := strings.Cut(item, "=")
		if ok && strings.EqualFold(name, "INSTANTREPO_APP_DATA_DIR") {
			return value
		}
	}
	return ""
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA == nil {
		a = aa
	}
	if errB == nil {
		b = bb
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
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

func (s *SQLiteStore) InstalledRepos(ctx context.Context) ([]domain.InstalledRepo, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, raw_url, normalized_url, local_path, status, created_at, updated_at, last_analyzed_at
FROM installed_repos
ORDER BY last_analyzed_at DESC, updated_at DESC, id DESC;
`)
	if err != nil {
		return nil, fmt.Errorf("query installed repos: %w", err)
	}
	defer rows.Close()

	var repos []domain.InstalledRepo
	for rows.Next() {
		repo, err := scanInstalledRepo(rows)
		if err != nil {
			return nil, fmt.Errorf("scan installed repo: %w", err)
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read installed repos: %w", err)
	}
	return repos, nil
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

func (s *SQLiteStore) EnvContributionSettings(ctx context.Context) (domain.EnvContributionSettings, error) {
	settings := defaultEnvContributionSettings()
	var raw string
	err := s.db.QueryRowContext(ctx, `
SELECT value
FROM app_settings
WHERE key = ?;
`, envContributionSettingsKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return settings, nil
	}
	if err != nil {
		return domain.EnvContributionSettings{}, fmt.Errorf("query env contribution settings: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return domain.EnvContributionSettings{}, fmt.Errorf("parse env contribution settings: %w", err)
	}
	return settings, nil
}

func (s *SQLiteStore) SaveEnvContributionSettings(ctx context.Context, settings domain.EnvContributionSettings) error {
	settings.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode env contribution settings: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO app_settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key)
DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;
`, envContributionSettingsKey, string(raw), formatTime(settings.UpdatedAt))
	if err != nil {
		return fmt.Errorf("save env contribution settings: %w", err)
	}
	return nil
}

func (s *SQLiteStore) AIEnvReviewSettings(ctx context.Context) (domain.AIEnvReviewSettings, error) {
	var settings domain.AIEnvReviewSettings
	var raw string
	err := s.db.QueryRowContext(ctx, `
SELECT value
FROM app_settings
WHERE key = ?;
`, aiEnvReviewSettingsKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return settings, nil
	}
	if err != nil {
		return domain.AIEnvReviewSettings{}, fmt.Errorf("query ai env review settings: %w", err)
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return domain.AIEnvReviewSettings{}, fmt.Errorf("parse ai env review settings: %w", err)
	}
	return settings, nil
}

func (s *SQLiteStore) EnvVaultCredentialNamespace(ctx context.Context) (string, error) {
	var namespace string
	err := s.db.QueryRowContext(ctx, `
SELECT value
FROM app_settings
WHERE key = ?;
`, envVaultCredentialNamespaceKey).Scan(&namespace)
	if err == nil && strings.TrimSpace(namespace) != "" {
		return namespace, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("query env vault credential namespace: %w", err)
	}

	generated, err := newEnvVaultCredentialNamespace()
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO app_settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO NOTHING;
`, envVaultCredentialNamespaceKey, generated, formatTime(time.Now().UTC())); err != nil {
		return "", fmt.Errorf("save env vault credential namespace: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
SELECT value
FROM app_settings
WHERE key = ?;
`, envVaultCredentialNamespaceKey).Scan(&namespace); err != nil {
		return "", fmt.Errorf("read env vault credential namespace: %w", err)
	}
	if strings.TrimSpace(namespace) == "" {
		return "", fmt.Errorf("env vault credential namespace is empty")
	}
	return namespace, nil
}

func (s *SQLiteStore) SaveAIEnvReviewSettings(ctx context.Context, settings domain.AIEnvReviewSettings) error {
	settings.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode ai env review settings: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO app_settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key)
DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;
`, aiEnvReviewSettingsKey, string(raw), formatTime(settings.UpdatedAt))
	if err != nil {
		return fmt.Errorf("save ai env review settings: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveEnvContributionQueueItem(ctx context.Context, item domain.EnvContributionQueueItem) (domain.EnvContributionQueueItem, error) {
	if strings.TrimSpace(item.EventType) == "" || strings.TrimSpace(item.PayloadJSON) == "" {
		return domain.EnvContributionQueueItem{}, fmt.Errorf("contribution event type and payload are required")
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	row := s.db.QueryRowContext(ctx, `
INSERT INTO env_pattern_contribution_queue (
	event_type,
	payload_json,
	created_at,
	attempts,
	last_attempt_at
) VALUES (?, ?, ?, ?, ?)
RETURNING id, event_type, payload_json, created_at, attempts, last_attempt_at;
`,
		item.EventType,
		item.PayloadJSON,
		formatTime(item.CreatedAt),
		item.Attempts,
		nullString(formatOptionalTime(item.LastAttemptAt)),
	)
	saved, err := scanEnvContributionQueueItem(row)
	if err != nil {
		return domain.EnvContributionQueueItem{}, fmt.Errorf("insert env contribution queue item: %w", err)
	}
	if err := s.PruneEnvContributionQueue(ctx, item.CreatedAt); err != nil {
		return domain.EnvContributionQueueItem{}, err
	}
	return saved, nil
}

func (s *SQLiteStore) EnvContributionQueueItems(ctx context.Context, limit int) ([]domain.EnvContributionQueueItem, error) {
	if limit <= 0 {
		limit = maxEnvContributionQueueItems
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, event_type, payload_json, created_at, attempts, last_attempt_at
FROM env_pattern_contribution_queue
ORDER BY created_at DESC, id DESC
LIMIT ?;
`, limit)
	if err != nil {
		return nil, fmt.Errorf("query env contribution queue: %w", err)
	}
	defer rows.Close()

	var items []domain.EnvContributionQueueItem
	for rows.Next() {
		item, err := scanEnvContributionQueueItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan env contribution queue item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read env contribution queue: %w", err)
	}
	return items, nil
}

func (s *SQLiteStore) EnvContributionQueueStatus(ctx context.Context) (domain.EnvContributionQueueStatus, error) {
	var status domain.EnvContributionQueueStatus
	var oldest sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*), MIN(created_at)
FROM env_pattern_contribution_queue;
`).Scan(&status.Count, &oldest)
	if err != nil {
		return domain.EnvContributionQueueStatus{}, fmt.Errorf("query env contribution queue status: %w", err)
	}
	if oldest.Valid && oldest.String != "" {
		parsed, err := parseTime(oldest.String)
		if err != nil {
			return domain.EnvContributionQueueStatus{}, fmt.Errorf("parse env contribution queue oldest: %w", err)
		}
		status.OldestCreatedAt = parsed
	}
	return status, nil
}

func (s *SQLiteStore) ClearEnvContributionQueue(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM env_pattern_contribution_queue;`); err != nil {
		return fmt.Errorf("clear env contribution queue: %w", err)
	}
	return nil
}

func (s *SQLiteStore) MarkEnvContributionQueueAttempt(ctx context.Context, id int64, attemptedAt time.Time) error {
	if id == 0 {
		return fmt.Errorf("env contribution queue item is required")
	}
	if attemptedAt.IsZero() {
		attemptedAt = time.Now().UTC()
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE env_pattern_contribution_queue
SET attempts = attempts + 1,
	last_attempt_at = ?
WHERE id = ?;
`, formatTime(attemptedAt), id); err != nil {
		return fmt.Errorf("mark env contribution queue attempt: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteEnvContributionQueueItem(ctx context.Context, id int64) error {
	if id == 0 {
		return fmt.Errorf("env contribution queue item is required")
	}
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM env_pattern_contribution_queue
WHERE id = ?;
`, id); err != nil {
		return fmt.Errorf("delete env contribution queue item: %w", err)
	}
	return nil
}

func (s *SQLiteStore) EnvPortAssignments(ctx context.Context, repoPath string) ([]domain.EnvPortAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT value
FROM app_settings
WHERE key LIKE ?;
`, envPortAssignmentKeyPrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("query env port assignments: %w", err)
	}
	defer rows.Close()

	var assignments []domain.EnvPortAssignment
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan env port assignment: %w", err)
		}
		var assignment domain.EnvPortAssignment
		if err := json.Unmarshal([]byte(raw), &assignment); err != nil {
			return nil, fmt.Errorf("parse env port assignment: %w", err)
		}
		if assignment.RepoPath == repoPath {
			assignments = append(assignments, assignment)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read env port assignments: %w", err)
	}
	return assignments, nil
}

func (s *SQLiteStore) SaveEnvPortAssignment(ctx context.Context, assignment domain.EnvPortAssignment) error {
	if strings.TrimSpace(assignment.RepoPath) == "" || strings.TrimSpace(assignment.TargetDir) == "" || strings.TrimSpace(assignment.Purpose) == "" || assignment.Port <= 0 {
		return fmt.Errorf("repo, target, purpose, and port are required")
	}
	raw, err := json.Marshal(assignment)
	if err != nil {
		return fmt.Errorf("encode env port assignment: %w", err)
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO app_settings (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key)
DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;
`, envPortAssignmentKey(assignment), string(raw), formatTime(now))
	if err != nil {
		return fmt.Errorf("save env port assignment: %w", err)
	}
	return nil
}

func (s *SQLiteStore) PruneEnvContributionQueue(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.UTC().Add(-envContributionQueueRetention)
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM env_pattern_contribution_queue
WHERE created_at < ?;
`, formatTime(cutoff)); err != nil {
		return fmt.Errorf("prune old env contribution queue items: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM env_pattern_contribution_queue
WHERE id NOT IN (
	SELECT id
	FROM env_pattern_contribution_queue
	ORDER BY created_at DESC, id DESC
	LIMIT ?
);
`, maxEnvContributionQueueItems); err != nil {
		return fmt.Errorf("prune excess env contribution queue items: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveEnvVaultEntry(ctx context.Context, entry domain.EnvVaultEntryMetadata) (domain.EnvVaultEntryMetadata, error) {
	if strings.TrimSpace(entry.Provider) == "" || strings.TrimSpace(entry.VariableName) == "" {
		return domain.EnvVaultEntryMetadata{}, fmt.Errorf("vault provider and variable are required")
	}
	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, `
INSERT INTO env_vault_entries (
	provider,
	variable_name,
	display_name,
	credential_key,
	fingerprint_sha256,
	fingerprint_fragment,
	status,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, provider, variable_name, display_name, credential_key, fingerprint_sha256, fingerprint_fragment, status, created_at, updated_at;
`,
		entry.Provider,
		entry.VariableName,
		entry.DisplayName,
		entry.CredentialKey,
		entry.Fingerprint,
		entry.FingerprintFragment,
		entry.Status,
		formatTime(now),
		formatTime(now),
	)
	saved, err := scanEnvVaultEntry(row)
	if err != nil {
		return domain.EnvVaultEntryMetadata{}, fmt.Errorf("insert env vault entry: %w", err)
	}
	return saved, nil
}

func (s *SQLiteStore) UpdateEnvVaultEntryDisplayName(ctx context.Context, entryID int64, displayName string) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE env_vault_entries
SET display_name = ?, updated_at = ?
WHERE id = ?;
`, displayName, formatTime(time.Now().UTC()), entryID); err != nil {
		return fmt.Errorf("update env vault display name: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateEnvVaultEntryCredentialMetadata(ctx context.Context, entryID int64, fingerprint, fingerprintFragment, status string) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE env_vault_entries
SET fingerprint_sha256 = ?, fingerprint_fragment = ?, status = ?, updated_at = ?
WHERE id = ?;
`, fingerprint, fingerprintFragment, status, formatTime(time.Now().UTC()), entryID); err != nil {
		return fmt.Errorf("update env vault credential metadata: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateEnvVaultEntryCredentialKey(ctx context.Context, entryID int64, credentialKey string) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE env_vault_entries
SET credential_key = ?, updated_at = ?
WHERE id = ?;
`, credentialKey, formatTime(time.Now().UTC()), entryID); err != nil {
		return fmt.Errorf("update env vault credential key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteEnvVaultEntry(ctx context.Context, entryID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin env vault delete: %w", err)
	}
	for _, stmt := range []string{
		`DELETE FROM env_vault_approvals WHERE entry_id = ?;`,
		`DELETE FROM env_vault_use_records WHERE entry_id = ?;`,
		`DELETE FROM env_vault_entries WHERE id = ?;`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, entryID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete env vault entry: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit env vault delete: %w", err)
	}
	return nil
}

func (s *SQLiteStore) EnvVaultEntries(ctx context.Context) ([]domain.EnvVaultEntryMetadata, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, provider, variable_name, display_name, credential_key, fingerprint_sha256, fingerprint_fragment, status, created_at, updated_at
FROM env_vault_entries
ORDER BY provider ASC, variable_name ASC, display_name ASC, id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("query env vault entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.EnvVaultEntryMetadata
	for rows.Next() {
		entry, err := scanEnvVaultEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan env vault entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read env vault entries: %w", err)
	}
	return entries, nil
}

func (s *SQLiteStore) EnvVaultEntryByID(ctx context.Context, entryID int64) (domain.EnvVaultEntryMetadata, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, provider, variable_name, display_name, credential_key, fingerprint_sha256, fingerprint_fragment, status, created_at, updated_at
FROM env_vault_entries
WHERE id = ?;
`, entryID)
	entry, err := scanEnvVaultEntry(row)
	if err != nil {
		return domain.EnvVaultEntryMetadata{}, fmt.Errorf("find env vault entry: %w", err)
	}
	return entry, nil
}

func (s *SQLiteStore) EnvVaultEntryByProviderFingerprint(ctx context.Context, provider, fingerprint string) (domain.EnvVaultEntryMetadata, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, provider, variable_name, display_name, credential_key, fingerprint_sha256, fingerprint_fragment, status, created_at, updated_at
FROM env_vault_entries
WHERE provider = ? AND fingerprint_sha256 = ?
ORDER BY id ASC
LIMIT 1;
`, provider, fingerprint)
	entry, err := scanEnvVaultEntry(row)
	if err != nil {
		return domain.EnvVaultEntryMetadata{}, fmt.Errorf("find env vault entry by fingerprint: %w", err)
	}
	return entry, nil
}

func (s *SQLiteStore) SetEnvVaultEntryStatus(ctx context.Context, entryID int64, status string) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE env_vault_entries
SET status = ?, updated_at = ?
WHERE id = ?;
`, status, formatTime(time.Now().UTC()), entryID); err != nil {
		return fmt.Errorf("update env vault status: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveEnvVaultApproval(ctx context.Context, approval domain.EnvVaultApproval) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO env_vault_approvals (
	entry_id,
	repo_path,
	target_relative_path,
	variable_name,
	status,
	created_at,
	updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(entry_id, repo_path, target_relative_path, variable_name)
DO UPDATE SET status = excluded.status, updated_at = excluded.updated_at;
`,
		approval.EntryID,
		approval.RepoPath,
		approval.TargetRelativePath,
		approval.VariableName,
		approval.Status,
		formatTime(now),
		formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("save env vault approval: %w", err)
	}
	return nil
}

func (s *SQLiteStore) EnvVaultApprovals(ctx context.Context, entryID int64) ([]domain.EnvVaultApproval, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, entry_id, repo_path, target_relative_path, variable_name, status, created_at, updated_at
FROM env_vault_approvals
WHERE entry_id = ?
ORDER BY updated_at DESC, id DESC;
`, entryID)
	if err != nil {
		return nil, fmt.Errorf("query env vault approvals: %w", err)
	}
	defer rows.Close()

	var approvals []domain.EnvVaultApproval
	for rows.Next() {
		approval, err := scanEnvVaultApproval(rows)
		if err != nil {
			return nil, fmt.Errorf("scan env vault approval: %w", err)
		}
		approvals = append(approvals, approval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read env vault approvals: %w", err)
	}
	return approvals, nil
}

func (s *SQLiteStore) RevokeEnvVaultApproval(ctx context.Context, approvalID int64) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE env_vault_approvals
SET status = ?, updated_at = ?
WHERE id = ?;
`, domain.EnvVaultApprovalStatusRevoked, formatTime(time.Now().UTC()), approvalID); err != nil {
		return fmt.Errorf("revoke env vault approval: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ApprovedEnvVaultEntries(ctx context.Context, repoPath, targetRelativePath, variableName string) ([]domain.EnvVaultEntryMetadata, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT e.id, e.provider, e.variable_name, e.display_name, e.credential_key, e.fingerprint_sha256, e.fingerprint_fragment, e.status, e.created_at, e.updated_at
FROM env_vault_entries e
JOIN env_vault_approvals a ON a.entry_id = e.id
WHERE a.repo_path = ?
	AND a.target_relative_path = ?
	AND a.variable_name = ?
	AND a.status = 'approved'
	AND e.status = ?
ORDER BY e.updated_at DESC, e.id DESC;
`, repoPath, targetRelativePath, variableName, domain.EnvVaultStatusReady)
	if err != nil {
		return nil, fmt.Errorf("query approved env vault entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.EnvVaultEntryMetadata
	for rows.Next() {
		entry, err := scanEnvVaultEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan approved env vault entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read approved env vault entries: %w", err)
	}
	return entries, nil
}

func (s *SQLiteStore) RecordEnvVaultUse(ctx context.Context, record domain.EnvVaultUseRecord) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO env_vault_use_records (
	entry_id,
	repo_path,
	target_relative_path,
	variable_name,
	used_at,
	use_count
) VALUES (?, ?, ?, ?, ?, 1)
ON CONFLICT(entry_id, repo_path, target_relative_path, variable_name)
DO UPDATE SET used_at = excluded.used_at, use_count = env_vault_use_records.use_count + 1;
`,
		record.EntryID,
		record.RepoPath,
		record.TargetRelativePath,
		record.VariableName,
		formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("record env vault use: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SuppressEnvVaultPrompt(ctx context.Context, suppression domain.EnvVaultPromptSuppression) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO env_vault_prompt_suppressions (
	repo_path,
	target_relative_path,
	variable_name,
	suppressed_at
) VALUES (?, ?, ?, ?)
ON CONFLICT(repo_path, target_relative_path, variable_name)
DO UPDATE SET suppressed_at = excluded.suppressed_at;
`,
		suppression.RepoPath,
		suppression.TargetRelativePath,
		suppression.VariableName,
		formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("suppress env vault prompt: %w", err)
	}
	return nil
}

func (s *SQLiteStore) IsEnvVaultPromptSuppressed(ctx context.Context, repoPath, targetRelativePath, variableName string) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
SELECT id
FROM env_vault_prompt_suppressions
WHERE repo_path = ? AND target_relative_path = ? AND variable_name = ?
LIMIT 1;
`, repoPath, targetRelativePath, variableName).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query env vault prompt suppression: %w", err)
	}
	return true, nil
}

func (s *SQLiteStore) EnvVaultUseRecords(ctx context.Context, entryID int64) ([]domain.EnvVaultUseRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, entry_id, repo_path, target_relative_path, variable_name, used_at, use_count
FROM env_vault_use_records
WHERE entry_id = ?
ORDER BY used_at DESC;
`, entryID)
	if err != nil {
		return nil, fmt.Errorf("query env vault use records: %w", err)
	}
	defer rows.Close()

	var records []domain.EnvVaultUseRecord
	for rows.Next() {
		record, err := scanEnvVaultUseRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan env vault use record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read env vault use records: %w", err)
	}
	return records, nil
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
		`CREATE TABLE IF NOT EXISTS env_vault_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			variable_name TEXT NOT NULL,
			display_name TEXT NOT NULL,
			credential_key TEXT NOT NULL DEFAULT '',
			fingerprint_sha256 TEXT NOT NULL,
			fingerprint_fragment TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS env_vault_entries_provider_fingerprint_idx
			ON env_vault_entries(provider, fingerprint_sha256);`,
		`CREATE TABLE IF NOT EXISTS env_vault_approvals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			repo_path TEXT NOT NULL,
			target_relative_path TEXT NOT NULL,
			variable_name TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(entry_id) REFERENCES env_vault_entries(id) ON DELETE CASCADE
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS env_vault_approvals_unique
			ON env_vault_approvals(entry_id, repo_path, target_relative_path, variable_name);`,
		`CREATE TABLE IF NOT EXISTS env_vault_use_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id INTEGER NOT NULL,
			repo_path TEXT NOT NULL,
			target_relative_path TEXT NOT NULL,
			variable_name TEXT NOT NULL,
			used_at TEXT NOT NULL,
			use_count INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY(entry_id) REFERENCES env_vault_entries(id) ON DELETE CASCADE
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS env_vault_use_records_unique
			ON env_vault_use_records(entry_id, repo_path, target_relative_path, variable_name);`,
		`CREATE TABLE IF NOT EXISTS env_vault_prompt_suppressions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_path TEXT NOT NULL,
			target_relative_path TEXT NOT NULL,
			variable_name TEXT NOT NULL,
			suppressed_at TEXT NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS env_vault_prompt_suppressions_unique
			ON env_vault_prompt_suppressions(repo_path, target_relative_path, variable_name);`,
		`CREATE TABLE IF NOT EXISTS env_pattern_contribution_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			last_attempt_at TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS env_pattern_contribution_queue_created_idx
			ON env_pattern_contribution_queue(created_at);`,
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
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (3, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));`); err != nil {
		return fmt.Errorf("record migration 3: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (4, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));`); err != nil {
		return fmt.Errorf("record migration 4: %w", err)
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

type envVaultEntryScanner interface {
	Scan(dest ...any) error
}

type envVaultApprovalScanner interface {
	Scan(dest ...any) error
}

type envVaultUseRecordScanner interface {
	Scan(dest ...any) error
}

type envContributionQueueItemScanner interface {
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

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return formatTime(value)
}

func defaultEnvContributionSettings() domain.EnvContributionSettings {
	return domain.EnvContributionSettings{
		PublicEnvPatternsEnabled:       true,
		PrivateLocalEnvPatternsEnabled: false,
		ConsentShown:                   false,
	}
}

func newEnvVaultCredentialNamespace() (string, error) {
	var raw [16]byte
	if _, err := crand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate env vault credential namespace: %w", err)
	}
	return "v1-" + hex.EncodeToString(raw[:]), nil
}

func envPortAssignmentKey(assignment domain.EnvPortAssignment) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		assignment.RepoPath,
		assignment.TargetDir,
		assignment.Purpose,
	}, "\x00")))
	return envPortAssignmentKeyPrefix + hex.EncodeToString(sum[:])
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

func scanEnvVaultEntry(row envVaultEntryScanner) (domain.EnvVaultEntryMetadata, error) {
	var entry domain.EnvVaultEntryMetadata
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&entry.ID,
		&entry.Provider,
		&entry.VariableName,
		&entry.DisplayName,
		&entry.CredentialKey,
		&entry.Fingerprint,
		&entry.FingerprintFragment,
		&entry.Status,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.EnvVaultEntryMetadata{}, err
	}
	var err error
	if entry.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.EnvVaultEntryMetadata{}, fmt.Errorf("parse env vault entry created_at: %w", err)
	}
	if entry.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.EnvVaultEntryMetadata{}, fmt.Errorf("parse env vault entry updated_at: %w", err)
	}
	return entry, nil
}

func scanEnvVaultApproval(row envVaultApprovalScanner) (domain.EnvVaultApproval, error) {
	var approval domain.EnvVaultApproval
	var createdAt string
	var updatedAt string
	if err := row.Scan(
		&approval.ID,
		&approval.EntryID,
		&approval.RepoPath,
		&approval.TargetRelativePath,
		&approval.VariableName,
		&approval.Status,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.EnvVaultApproval{}, err
	}
	var err error
	if approval.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.EnvVaultApproval{}, fmt.Errorf("parse env vault approval created_at: %w", err)
	}
	if approval.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.EnvVaultApproval{}, fmt.Errorf("parse env vault approval updated_at: %w", err)
	}
	return approval, nil
}

func scanEnvVaultUseRecord(row envVaultUseRecordScanner) (domain.EnvVaultUseRecord, error) {
	var record domain.EnvVaultUseRecord
	var usedAt string
	if err := row.Scan(
		&record.ID,
		&record.EntryID,
		&record.RepoPath,
		&record.TargetRelativePath,
		&record.VariableName,
		&usedAt,
		&record.UseCount,
	); err != nil {
		return domain.EnvVaultUseRecord{}, err
	}
	var err error
	if record.UsedAt, err = parseTime(usedAt); err != nil {
		return domain.EnvVaultUseRecord{}, fmt.Errorf("parse env vault use used_at: %w", err)
	}
	return record, nil
}

func scanEnvContributionQueueItem(row envContributionQueueItemScanner) (domain.EnvContributionQueueItem, error) {
	var item domain.EnvContributionQueueItem
	var createdAt string
	var lastAttemptAt sql.NullString
	if err := row.Scan(
		&item.ID,
		&item.EventType,
		&item.PayloadJSON,
		&createdAt,
		&item.Attempts,
		&lastAttemptAt,
	); err != nil {
		return domain.EnvContributionQueueItem{}, err
	}
	var err error
	if item.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.EnvContributionQueueItem{}, fmt.Errorf("parse env contribution queue created_at: %w", err)
	}
	if lastAttemptAt.Valid && lastAttemptAt.String != "" {
		if item.LastAttemptAt, err = parseTime(lastAttemptAt.String); err != nil {
			return domain.EnvContributionQueueItem{}, fmt.Errorf("parse env contribution queue last_attempt_at: %w", err)
		}
	}
	return item, nil
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
