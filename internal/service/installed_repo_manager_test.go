package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"instantrepo/internal/domain"
	"instantrepo/internal/store"
)

func TestListInstalledReposReturnsEmptyManagerList(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openManagerTestStore(t)
	defer sqliteStore.Close()

	app := NewAppServiceWithInstalledRepoStore(sqliteStore)
	manager, err := app.ListInstalledRepos(ctx)
	if err != nil {
		t.Fatalf("ListInstalledRepos returned error: %v", err)
	}

	if len(manager.Repos) != 0 {
		t.Fatalf("expected empty Installed Repo manager list, got %+v", manager.Repos)
	}
}

func TestListInstalledReposIncludesManagerRowShape(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openManagerTestStore(t)
	defer sqliteStore.Close()

	analyzedAt := time.Date(2026, 5, 8, 10, 30, 0, 0, time.UTC)
	repo, err := sqliteStore.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/example/instant-repo.git",
		NormalizedURL:  "https://github.com/example/instant-repo",
		LocalPath:      filepath.Join(t.TempDir(), "instant-repo"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: analyzedAt,
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo returned error: %v", err)
	}
	session, err := sqliteStore.StartSetupSession(ctx, repo.ID, repo.LocalPath)
	if err != nil {
		t.Fatalf("StartSetupSession returned error: %v", err)
	}

	app := NewAppServiceWithInstalledRepoStore(sqliteStore)
	manager, err := app.ListInstalledRepos(ctx)
	if err != nil {
		t.Fatalf("ListInstalledRepos returned error: %v", err)
	}

	if len(manager.Repos) != 1 {
		t.Fatalf("expected one Installed Repo row, got %+v", manager.Repos)
	}
	got := manager.Repos[0]
	if got.ID != repo.ID || got.ProjectName != "instant-repo" {
		t.Fatalf("expected repo identity in manager row, got %+v", got)
	}
	if got.LocalPath != repo.LocalPath || got.Status != domain.InstalledRepoStatusAnalyzed {
		t.Fatalf("expected stored local path and status, got %+v", got)
	}
	if !got.LastAnalyzedAt.Equal(analyzedAt) {
		t.Fatalf("expected last analyzed at %s, got %s", analyzedAt, got.LastAnalyzedAt)
	}
	if !got.LastSetupAt.Equal(session.UpdatedAt) {
		t.Fatalf("expected last setup at %s, got %s", session.UpdatedAt, got.LastSetupAt)
	}
	if !got.LastActivityAt.Equal(session.UpdatedAt) {
		t.Fatalf("expected last activity to prefer setup time %s, got %s", session.UpdatedAt, got.LastActivityAt)
	}
}

func TestInstalledRepoDetailsIncludesRecentSetupSessions(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openManagerTestStore(t)
	defer sqliteStore.Close()

	targetRepo, err := sqliteStore.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/example/target.git",
		NormalizedURL:  "https://github.com/example/target",
		LocalPath:      filepath.Join(t.TempDir(), "target"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo target returned error: %v", err)
	}
	otherRepo, err := sqliteStore.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/example/other.git",
		NormalizedURL:  "https://github.com/example/other",
		LocalPath:      filepath.Join(t.TempDir(), "other"),
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo other returned error: %v", err)
	}

	firstSession := recordManagerSession(t, sqliteStore, targetRepo, "first", domain.StepRunStatusSucceeded)
	secondSession := recordManagerSession(t, sqliteStore, targetRepo, "second", domain.StepRunStatusFailed)
	otherSession := recordManagerSession(t, sqliteStore, otherRepo, "other", domain.StepRunStatusSucceeded)

	app := NewAppServiceWithInstalledRepoStore(sqliteStore)
	details, err := app.InstalledRepoDetails(ctx, targetRepo.ID)
	if err != nil {
		t.Fatalf("InstalledRepoDetails returned error: %v", err)
	}

	if details.Repo.ID != targetRepo.ID || details.Repo.ProjectName != "target" {
		t.Fatalf("expected target repo summary, got %+v", details.Repo)
	}
	if len(details.SetupSessions) != 2 {
		t.Fatalf("expected two target setup sessions, got %+v", details.SetupSessions)
	}
	if details.SetupSessions[0].ID != secondSession.ID || details.SetupSessions[0].Status != domain.SetupSessionStatusFailed {
		t.Fatalf("expected newest failed setup session first, got %+v", details.SetupSessions[0])
	}
	if details.SetupSessions[1].ID != firstSession.ID || details.SetupSessions[1].Status != domain.SetupSessionStatusSucceeded {
		t.Fatalf("expected older succeeded setup session second, got %+v", details.SetupSessions[1])
	}
	for _, session := range details.SetupSessions {
		if session.ID == otherSession.ID || session.InstalledRepoID != targetRepo.ID {
			t.Fatalf("expected details to include only target repo sessions, got %+v", details.SetupSessions)
		}
	}
}

func recordManagerSession(t *testing.T, sqliteStore *store.SQLiteStore, repo domain.InstalledRepo, stepID, stepStatus string) domain.SetupSession {
	t.Helper()
	session, err := sqliteStore.StartSetupSession(context.Background(), repo.ID, repo.LocalPath)
	if err != nil {
		t.Fatalf("StartSetupSession returned error: %v", err)
	}
	if _, err := sqliteStore.RecordStepRun(context.Background(), domain.StepRun{
		SetupSessionID: session.ID,
		StepID:         stepID,
		Title:          "Run setup step",
		CommandHash:    commandHash("go version"),
		CommandPreview: "go version",
		Cwd:            repo.LocalPath,
		Status:         stepStatus,
		ExitCode:       0,
		Duration:       "1ms",
		StartedAt:      time.Now().UTC(),
		FinishedAt:     time.Now().UTC(),
	}, "setup log\n"); err != nil {
		t.Fatalf("RecordStepRun returned error: %v", err)
	}
	sessions, err := sqliteStore.SetupSessionsByInstalledRepoID(context.Background(), repo.ID)
	if err != nil {
		t.Fatalf("SetupSessionsByInstalledRepoID returned error: %v", err)
	}
	for _, saved := range sessions {
		if saved.ID == session.ID {
			return saved
		}
	}
	t.Fatalf("saved setup session %d not found", session.ID)
	return domain.SetupSession{}
}

func openManagerTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	sqliteStore, err := store.OpenSQLiteStore(filepath.Join(t.TempDir(), "instantrepo.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	return sqliteStore
}
