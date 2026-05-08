package store

import (
	"context"
	"path/filepath"
	"sort"
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
