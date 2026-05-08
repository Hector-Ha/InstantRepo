package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"instantrepo/internal/domain"
)

func TestSQLiteStoreSavesInstalledRepoOnCleanDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "instantrepo.db")

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	analyzedAt := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	saved, err := store.SaveInstalledRepo(context.Background(), domain.InstalledRepo{
		RawURL:         "https://github.com/Example/InstantRepo.git",
		NormalizedURL:  "https://github.com/example/instantrepo",
		LocalPath:      filepath.Join(t.TempDir(), "InstantRepo"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: analyzedAt,
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo returned error: %v", err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected saved repo ID")
	}

	got, err := store.InstalledRepoByLocalPath(context.Background(), saved.LocalPath)
	if err != nil {
		t.Fatalf("InstalledRepoByLocalPath returned error: %v", err)
	}

	if got.RawURL != "https://github.com/Example/InstantRepo.git" {
		t.Fatalf("expected raw URL to round trip, got %q", got.RawURL)
	}
	if got.NormalizedURL != "https://github.com/example/instantrepo" {
		t.Fatalf("expected normalized URL to round trip, got %q", got.NormalizedURL)
	}
	if got.LocalPath != saved.LocalPath {
		t.Fatalf("expected local path %q, got %q", saved.LocalPath, got.LocalPath)
	}
	if got.Status != domain.InstalledRepoStatusAnalyzed {
		t.Fatalf("expected analyzed status, got %q", got.Status)
	}
	if !got.LastAnalyzedAt.Equal(analyzedAt) {
		t.Fatalf("expected last analyzed time %s, got %s", analyzedAt, got.LastAnalyzedAt)
	}
}

func TestSQLiteStoreInitializesFoundationTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "instantrepo.db")

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	rows, err := store.db.Query(`
SELECT name
FROM sqlite_master
WHERE type = 'table'
	AND name IN ('schema_migrations', 'installed_repos', 'setup_sessions', 'step_runs', 'app_settings')
ORDER BY name;
`)
	if err != nil {
		t.Fatalf("query schema tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read table names: %v", err)
	}

	sort.Strings(names)
	want := []string{"app_settings", "installed_repos", "schema_migrations", "setup_sessions", "step_runs"}
	for i := range want {
		if i >= len(names) || names[i] != want[i] {
			t.Fatalf("expected foundation tables %v, got %v", want, names)
		}
	}

	var version int
	if err := store.db.QueryRow(`SELECT version FROM schema_migrations WHERE version = 1;`).Scan(&version); err != nil {
		t.Fatalf("expected schema migration version 1: %v", err)
	}
}

func TestSQLiteStoreUpdatesInstalledRepoForRepeatedAnalyze(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "instantrepo.db")

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	first, err := store.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/Example/InstantRepo.git",
		NormalizedURL:  "https://github.com/example/instantrepo",
		LocalPath:      filepath.Join(t.TempDir(), "old"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("first SaveInstalledRepo returned error: %v", err)
	}

	nextPath := filepath.Join(t.TempDir(), "new")
	nextAnalyzedAt := time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)
	second, err := store.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/Example/InstantRepo",
		NormalizedURL:  "https://github.com/example/instantrepo",
		LocalPath:      nextPath,
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: nextAnalyzedAt,
	})
	if err != nil {
		t.Fatalf("second SaveInstalledRepo returned error: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("expected repeated analyze to update ID %d, got %d", first.ID, second.ID)
	}
	if second.LocalPath != nextPath {
		t.Fatalf("expected local path to update to %q, got %q", nextPath, second.LocalPath)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("expected created time to stay %s, got %s", first.CreatedAt, second.CreatedAt)
	}
	if !second.LastAnalyzedAt.Equal(nextAnalyzedAt) {
		t.Fatalf("expected last analyzed time %s, got %s", nextAnalyzedAt, second.LastAnalyzedAt)
	}
}

func TestSQLiteStoreUpdatesLocalOnlyRepoWhenURLBecomesKnown(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "instantrepo.db")

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	localPath := filepath.Join(t.TempDir(), "InstantRepo")
	first, err := store.SaveInstalledRepo(ctx, domain.InstalledRepo{
		LocalPath:      localPath,
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("first SaveInstalledRepo returned error: %v", err)
	}

	second, err := store.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/Example/InstantRepo.git",
		NormalizedURL:  "https://github.com/example/instantrepo",
		LocalPath:      localPath,
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("second SaveInstalledRepo returned error: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("expected local-only repo to update ID %d, got %d", first.ID, second.ID)
	}
	if second.NormalizedURL != "https://github.com/example/instantrepo" {
		t.Fatalf("expected normalized URL to be stored, got %q", second.NormalizedURL)
	}
}

func TestSQLiteStoreFindsInstalledRepoByNormalizedURL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "instantrepo.db")

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	saved, err := store.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/Example/InstantRepo.git",
		NormalizedURL:  "https://github.com/example/instantrepo",
		LocalPath:      filepath.Join(t.TempDir(), "InstantRepo"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo returned error: %v", err)
	}

	got, err := store.InstalledRepoByNormalizedURL(ctx, "https://github.com/example/instantrepo")
	if err != nil {
		t.Fatalf("InstalledRepoByNormalizedURL returned error: %v", err)
	}

	if got.ID != saved.ID {
		t.Fatalf("expected ID %d, got %d", saved.ID, got.ID)
	}
	if got.LocalPath != saved.LocalPath {
		t.Fatalf("expected local path %q, got %q", saved.LocalPath, got.LocalPath)
	}
}

func TestSQLiteStoreRecordsSetupSessionAndStepRun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "instantrepo.db")

	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	repo, err := store.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/example/app.git",
		NormalizedURL:  "https://github.com/example/app",
		LocalPath:      filepath.Join(t.TempDir(), "app"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo returned error: %v", err)
	}

	session, err := store.StartSetupSession(ctx, repo.ID, repo.LocalPath)
	if err != nil {
		t.Fatalf("StartSetupSession returned error: %v", err)
	}
	if session.ID == 0 {
		t.Fatalf("expected session ID")
	}
	if session.InstalledRepoID != repo.ID {
		t.Fatalf("expected installed repo ID %d, got %d", repo.ID, session.InstalledRepoID)
	}

	startedAt := time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(250 * time.Millisecond)
	run, err := store.RecordStepRun(ctx, domain.StepRun{
		SetupSessionID: session.ID,
		StepID:         "go-run",
		Title:          "Run Go project",
		CommandHash:    "8d5d97827f816338240d350bc7b7c5ac08a1f43d62e78a4b4c38dcbe8bb8960d",
		CommandPreview: "go run .",
		Cwd:            repo.LocalPath,
		Status:         domain.StepRunStatusSucceeded,
		ExitCode:       0,
		Duration:       "250ms",
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	}, "OPENAI_API_KEY=[REDACTED]\n")
	if err != nil {
		t.Fatalf("RecordStepRun returned error: %v", err)
	}
	if run.ID == 0 {
		t.Fatalf("expected step run ID")
	}
	if run.LogPath == "" {
		t.Fatalf("expected log path")
	}

	rawLog, err := os.ReadFile(run.LogPath)
	if err != nil {
		t.Fatalf("read step log: %v", err)
	}
	if !strings.Contains(string(rawLog), "[REDACTED]") {
		t.Fatalf("expected redacted log content, got %q", string(rawLog))
	}

	var commandHash, commandPreview, status, duration, started, finished, logPath string
	var exitCode int
	if err := store.db.QueryRow(`
SELECT command_hash, command_preview, status, exit_code, duration, started_at, finished_at, log_path
FROM step_runs
WHERE id = ?;
`, run.ID).Scan(&commandHash, &commandPreview, &status, &exitCode, &duration, &started, &finished, &logPath); err != nil {
		t.Fatalf("query step run: %v", err)
	}

	if commandHash != run.CommandHash || commandPreview != "go run ." {
		t.Fatalf("expected command metadata to persist, got hash %q preview %q", commandHash, commandPreview)
	}
	if status != domain.StepRunStatusSucceeded || exitCode != 0 || duration != "250ms" {
		t.Fatalf("expected status, exit code, duration to persist, got status %q exit %d duration %q", status, exitCode, duration)
	}
	if started != formatTime(startedAt) || finished != formatTime(finishedAt) {
		t.Fatalf("expected timestamps %q/%q, got %q/%q", formatTime(startedAt), formatTime(finishedAt), started, finished)
	}
	if logPath != run.LogPath {
		t.Fatalf("expected log path %q, got %q", run.LogPath, logPath)
	}
}

func TestSQLiteStoreCleanupSetupSessionRetentionBoundsAgeAndCount(t *testing.T) {
	t.Run("removes logs older than seven days but keeps boundary", func(t *testing.T) {
		store := openTestSQLiteStore(t)
		defer store.Close()

		ctx := context.Background()
		repo := saveTestInstalledRepo(t, store)
		now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		expiredSessionID, expiredLog := createStoredSessionWithLog(t, store, repo, now.Add(-7*24*time.Hour-time.Nanosecond), "expired")
		boundarySessionID, boundaryLog := createStoredSessionWithLog(t, store, repo, now.Add(-7*24*time.Hour), "boundary")

		if err := store.CleanupSetupSessionRetention(ctx, now); err != nil {
			t.Fatalf("CleanupSetupSessionRetention returned error: %v", err)
		}

		if sessionExists(t, store, expiredSessionID) {
			t.Fatalf("expected expired setup session to be deleted")
		}
		if pathExists(expiredLog) {
			t.Fatalf("expected expired setup log to be deleted")
		}
		if !sessionExists(t, store, boundarySessionID) {
			t.Fatalf("expected boundary setup session to be kept")
		}
		if !pathExists(boundaryLog) {
			t.Fatalf("expected boundary setup log to be kept")
		}
	})

	t.Run("keeps last ten sessions per installed repo", func(t *testing.T) {
		store := openTestSQLiteStore(t)
		defer store.Close()

		ctx := context.Background()
		repo := saveTestInstalledRepo(t, store)
		now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
		sessionIDs := make([]int64, 0, 11)
		logPaths := make([]string, 0, 11)
		for i := 0; i < 11; i++ {
			sessionID, logPath := createStoredSessionWithLog(t, store, repo, now.Add(-time.Duration(11-i)*time.Minute), fmt.Sprintf("count-%02d", i))
			sessionIDs = append(sessionIDs, sessionID)
			logPaths = append(logPaths, logPath)
		}

		if err := store.CleanupSetupSessionRetention(ctx, now); err != nil {
			t.Fatalf("CleanupSetupSessionRetention returned error: %v", err)
		}

		if sessionExists(t, store, sessionIDs[0]) {
			t.Fatalf("expected oldest setup session past count limit to be deleted")
		}
		if pathExists(logPaths[0]) {
			t.Fatalf("expected oldest setup log past count limit to be deleted")
		}
		for i := 1; i < len(sessionIDs); i++ {
			if !sessionExists(t, store, sessionIDs[i]) {
				t.Fatalf("expected session %d to be kept", sessionIDs[i])
			}
			if !pathExists(logPaths[i]) {
				t.Fatalf("expected log %s to be kept", logPaths[i])
			}
		}
	})
}

func TestSQLiteStoreCleanupSetupSessionRetentionKeepsRowsWhenLogDeleteFails(t *testing.T) {
	store := openTestSQLiteStore(t)
	defer store.Close()

	ctx := context.Background()
	repo := saveTestInstalledRepo(t, store)
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	sessionID, logPath := createStoredSessionWithLog(t, store, repo, now.Add(-7*24*time.Hour-time.Nanosecond), "retry")

	if err := os.Remove(logPath); err != nil {
		t.Fatalf("remove setup log before blocking delete: %v", err)
	}
	if err := os.Mkdir(logPath, 0o700); err != nil {
		t.Fatalf("create blocking log path directory: %v", err)
	}
	blockerPath := filepath.Join(logPath, "blocker")
	if err := os.WriteFile(blockerPath, []byte("still here"), 0o600); err != nil {
		t.Fatalf("write blocking child file: %v", err)
	}

	err := store.CleanupSetupSessionRetention(ctx, now)
	if err == nil {
		t.Fatalf("expected cleanup to fail when setup log cannot be deleted")
	}
	if !sessionExists(t, store, sessionID) {
		t.Fatalf("expected setup session row to remain after log delete failure")
	}
	if stepRunCountForSession(t, store, sessionID) == 0 {
		t.Fatalf("expected step run row to remain after log delete failure")
	}

	if err := os.Remove(blockerPath); err != nil {
		t.Fatalf("remove blocking child file: %v", err)
	}
	if err := store.CleanupSetupSessionRetention(ctx, now); err != nil {
		t.Fatalf("cleanup retry returned error: %v", err)
	}
	if sessionExists(t, store, sessionID) {
		t.Fatalf("expected setup session row to be deleted on retry")
	}
	if stepRunCountForSession(t, store, sessionID) != 0 {
		t.Fatalf("expected step run row to be deleted on retry")
	}
	if pathExists(logPath) {
		t.Fatalf("expected setup log path to be deleted on retry")
	}
}

func openTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "instantrepo.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	return store
}

func saveTestInstalledRepo(t *testing.T, store *SQLiteStore) domain.InstalledRepo {
	t.Helper()
	repo, err := store.SaveInstalledRepo(context.Background(), domain.InstalledRepo{
		RawURL:         "https://github.com/example/app.git",
		NormalizedURL:  fmt.Sprintf("https://github.com/example/app-%d", time.Now().UnixNano()),
		LocalPath:      filepath.Join(t.TempDir(), "app"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo returned error: %v", err)
	}
	return repo
}

func createStoredSessionWithLog(t *testing.T, store *SQLiteStore, repo domain.InstalledRepo, createdAt time.Time, stepID string) (int64, string) {
	t.Helper()
	ctx := context.Background()
	session, err := store.StartSetupSession(ctx, repo.ID, repo.LocalPath)
	if err != nil {
		t.Fatalf("StartSetupSession returned error: %v", err)
	}
	run, err := store.RecordStepRun(ctx, domain.StepRun{
		SetupSessionID: session.ID,
		StepID:         stepID,
		Title:          "Run setup step",
		CommandHash:    "hash-" + stepID,
		CommandPreview: "go version",
		Cwd:            repo.LocalPath,
		Status:         domain.StepRunStatusSucceeded,
		ExitCode:       0,
		Duration:       "1ms",
		StartedAt:      createdAt,
		FinishedAt:     createdAt.Add(time.Millisecond),
	}, "log for "+stepID+"\n")
	if err != nil {
		t.Fatalf("RecordStepRun returned error: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `
UPDATE setup_sessions
SET created_at = ?, updated_at = ?
WHERE id = ?;
`, formatTime(createdAt), formatTime(createdAt), session.ID); err != nil {
		t.Fatalf("backdate setup session: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `
UPDATE step_runs
SET created_at = ?, updated_at = ?
WHERE id = ?;
`, formatTime(createdAt), formatTime(createdAt), run.ID); err != nil {
		t.Fatalf("backdate step run: %v", err)
	}
	return session.ID, run.LogPath
}

func sessionExists(t *testing.T, store *SQLiteStore, sessionID int64) bool {
	t.Helper()
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM setup_sessions WHERE id = ?;`, sessionID).Scan(&count); err != nil {
		t.Fatalf("query setup session count: %v", err)
	}
	return count > 0
}

func stepRunCountForSession(t *testing.T, store *SQLiteStore, sessionID int64) int {
	t.Helper()
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM step_runs WHERE setup_session_id = ?;`, sessionID).Scan(&count); err != nil {
		t.Fatalf("query step run count: %v", err)
	}
	return count
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
