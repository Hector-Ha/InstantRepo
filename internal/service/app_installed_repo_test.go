package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"instantrepo/internal/domain"
	"instantrepo/internal/store"
)

type recordingInstalledRepoStore struct {
	repos []domain.InstalledRepo
}

func (s *recordingInstalledRepoStore) SaveInstalledRepo(_ context.Context, repo domain.InstalledRepo) (domain.InstalledRepo, error) {
	s.repos = append(s.repos, repo)
	return repo, nil
}

func TestAnalyzePersistsInstalledRepo(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	store := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(store)

	resp, err := app.Analyze(context.Background(), domain.AnalyzeRequest{
		RepoURL:   " https://github.com/Example/InstantRepo.git ",
		LocalPath: repoPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(store.repos) != 1 {
		t.Fatalf("expected one installed repo save, got %d", len(store.repos))
	}

	got := store.repos[0]
	if got.RawURL != "https://github.com/Example/InstantRepo.git" {
		t.Fatalf("expected trimmed raw URL, got %q", got.RawURL)
	}
	if got.NormalizedURL != "https://github.com/example/instantrepo" {
		t.Fatalf("expected normalized URL, got %q", got.NormalizedURL)
	}
	if got.LocalPath != resp.Source.Path {
		t.Fatalf("expected local path %q, got %q", resp.Source.Path, got.LocalPath)
	}
	if got.Status != domain.InstalledRepoStatusAnalyzed {
		t.Fatalf("expected analyzed status, got %q", got.Status)
	}
	if got.LastAnalyzedAt.IsZero() {
		t.Fatalf("expected last analyzed time to be set")
	}
}

func TestAnalyzePersistsLocalPathOnlyInstalledRepo(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte(`{"name":"local-app"}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	store := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(store)

	resp, err := app.Analyze(context.Background(), domain.AnalyzeRequest{
		LocalPath: repoPath,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(store.repos) != 1 {
		t.Fatalf("expected one installed repo save, got %d", len(store.repos))
	}

	got := store.repos[0]
	if got.RawURL != "" {
		t.Fatalf("expected no raw URL, got %q", got.RawURL)
	}
	if got.NormalizedURL != "" {
		t.Fatalf("expected no normalized URL, got %q", got.NormalizedURL)
	}
	if got.LocalPath != resp.Source.Path {
		t.Fatalf("expected local path %q, got %q", resp.Source.Path, got.LocalPath)
	}
	if got.Status != domain.InstalledRepoStatusAnalyzed {
		t.Fatalf("expected analyzed status, got %q", got.Status)
	}
}

func TestAnalyzeUpdatesInstalledRepoForRepeatedAnalyze(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	sqliteStore, err := store.OpenSQLiteStore(filepath.Join(t.TempDir(), "instantrepo.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	app := NewAppServiceWithInstalledRepoStore(sqliteStore)
	req := domain.AnalyzeRequest{
		RepoURL:   "https://github.com/Example/InstantRepo.git",
		LocalPath: repoPath,
	}

	if _, err := app.Analyze(context.Background(), req); err != nil {
		t.Fatalf("first Analyze returned error: %v", err)
	}
	first, err := sqliteStore.InstalledRepoByLocalPath(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("first InstalledRepoByLocalPath returned error: %v", err)
	}

	if _, err := app.Analyze(context.Background(), req); err != nil {
		t.Fatalf("second Analyze returned error: %v", err)
	}
	second, err := sqliteStore.InstalledRepoByLocalPath(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("second InstalledRepoByLocalPath returned error: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("expected repeated analyze to update ID %d, got %d", first.ID, second.ID)
	}
	if second.UpdatedAt.Before(first.UpdatedAt) {
		t.Fatalf("expected updated time to move forward or remain equal, first %s second %s", first.UpdatedAt, second.UpdatedAt)
	}
}
