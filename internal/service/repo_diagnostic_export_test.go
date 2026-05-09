package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"instantrepo/internal/domain"
	"instantrepo/internal/store"
)

func TestExportRepoDiagnosticsIncludesRepoIdentityEnvironmentAnalysisPlanAndStepRuns(t *testing.T) {
	ctx := context.Background()
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module example.com/diagnostic\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	sqliteStore := openDiagnosticTestStore(t)
	defer sqliteStore.Close()

	repo, err := sqliteStore.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         "https://github.com/example/diagnostic.git",
		NormalizedURL:  "https://github.com/example/diagnostic",
		LocalPath:      repoPath,
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo returned error: %v", err)
	}
	session, err := sqliteStore.StartSetupSession(ctx, repo.ID, repo.LocalPath)
	if err != nil {
		t.Fatalf("StartSetupSession returned error: %v", err)
	}
	if _, err := sqliteStore.RecordStepRun(ctx, domain.StepRun{
		SetupSessionID: session.ID,
		StepID:         "go-run",
		Title:          "Run Go project",
		CommandHash:    commandHash("go run ."),
		CommandPreview: "go run .",
		Cwd:            repo.LocalPath,
		Status:         domain.StepRunStatusSucceeded,
		ExitCode:       0,
		Duration:       "250ms",
		StartedAt:      time.Date(2026, 5, 8, 12, 1, 0, 0, time.UTC),
		FinishedAt:     time.Date(2026, 5, 8, 12, 1, 0, int(250*time.Millisecond), time.UTC),
	}, "setup completed\n"); err != nil {
		t.Fatalf("RecordStepRun returned error: %v", err)
	}

	app := newDiagnosticTestApp(sqliteStore)
	export, err := app.ExportRepoDiagnostics(ctx, domain.RepoDiagnosticExportRequest{LocalPath: repoPath})
	if err != nil {
		t.Fatalf("ExportRepoDiagnostics returned error: %v", err)
	}

	if export.SchemaVersion == "" || export.GeneratedAt.IsZero() {
		t.Fatalf("expected export schema and timestamp, got %+v", export)
	}
	if export.Repo.ID != repo.ID || export.Repo.LocalPath != repoPath || export.Repo.NormalizedURL != repo.NormalizedURL {
		t.Fatalf("expected repo identity to round trip, got %+v", export.Repo)
	}
	if export.App.Name != "InstantRepo" || export.App.Version == "" {
		t.Fatalf("expected app identity and version, got %+v", export.App)
	}
	if export.Environment.OS == "" || export.Environment.Arch == "" || len(export.Environment.Tools) == 0 {
		t.Fatalf("expected environment versions, got %+v", export.Environment)
	}
	if export.Analysis.ProjectName == "" || export.Analysis.ProjectType != "go-project" {
		t.Fatalf("expected analysis summary, got %+v", export.Analysis)
	}
	if export.SetupPlan.ProjectName == "" || len(export.SetupPlan.Steps) == 0 {
		t.Fatalf("expected setup plan summary, got %+v", export.SetupPlan)
	}
	if len(export.SetupSessions) != 1 || len(export.SetupSessions[0].Steps) != 1 {
		t.Fatalf("expected one setup session with one step, got %+v", export.SetupSessions)
	}
	step := export.SetupSessions[0].Steps[0]
	if step.StepID != "go-run" || step.Status != domain.StepRunStatusSucceeded || step.ExitCode != 0 {
		t.Fatalf("expected step status and exit code, got %+v", step)
	}
	if !strings.Contains(step.Log, "setup completed") {
		t.Fatalf("expected step log content, got %q", step.Log)
	}
	if export.AIReview.Available || len(export.AIReview.Entries) != 0 {
		t.Fatalf("expected absent AI Review Log metadata, got %+v", export.AIReview)
	}
}

func TestExportRepoDiagnosticsIsScopedToOneInstalledRepo(t *testing.T) {
	ctx := context.Background()
	firstPath := createGoDiagnosticRepo(t)
	secondPath := createGoDiagnosticRepo(t)

	sqliteStore := openDiagnosticTestStore(t)
	defer sqliteStore.Close()

	firstRepo := saveDiagnosticRepo(t, sqliteStore, "https://github.com/example/first", firstPath)
	firstSession := startDiagnosticSession(t, sqliteStore, firstRepo)
	recordDiagnosticStep(t, sqliteStore, firstSession.ID, firstRepo.LocalPath, "first-step", "first repo log\n")

	secondRepo := saveDiagnosticRepo(t, sqliteStore, "https://github.com/example/second", secondPath)
	secondSession := startDiagnosticSession(t, sqliteStore, secondRepo)
	recordDiagnosticStep(t, sqliteStore, secondSession.ID, secondRepo.LocalPath, "second-step", "second repo secret log\n")

	app := newDiagnosticTestApp(sqliteStore)
	export, err := app.ExportRepoDiagnostics(ctx, domain.RepoDiagnosticExportRequest{InstalledRepoID: firstRepo.ID})
	if err != nil {
		t.Fatalf("ExportRepoDiagnostics returned error: %v", err)
	}

	if export.Repo.ID != firstRepo.ID {
		t.Fatalf("expected first repo identity, got %+v", export.Repo)
	}
	if len(export.SetupSessions) != 1 || len(export.SetupSessions[0].Steps) != 1 {
		t.Fatalf("expected only first repo setup data, got %+v", export.SetupSessions)
	}
	if got := export.SetupSessions[0].Steps[0]; got.StepID != "first-step" || !strings.Contains(got.Log, "first repo log") {
		t.Fatalf("expected first repo step only, got %+v", got)
	}
	if strings.Contains(export.SetupSessions[0].Steps[0].Log, "second repo secret log") {
		t.Fatalf("expected second repo log to stay out of export")
	}
}

func TestExportRepoDiagnosticsTruncatesStepLogs(t *testing.T) {
	ctx := context.Background()
	repoPath := createGoDiagnosticRepo(t)
	longLog := strings.Repeat("x", repoDiagnosticLogMaxRunes+256)

	sqliteStore := openDiagnosticTestStore(t)
	defer sqliteStore.Close()

	repo := saveDiagnosticRepo(t, sqliteStore, "https://github.com/example/truncate", repoPath)
	session := startDiagnosticSession(t, sqliteStore, repo)
	recordDiagnosticStep(t, sqliteStore, session.ID, repo.LocalPath, "long-log", longLog)

	app := newDiagnosticTestApp(sqliteStore)
	export, err := app.ExportRepoDiagnostics(ctx, domain.RepoDiagnosticExportRequest{InstalledRepoID: repo.ID})
	if err != nil {
		t.Fatalf("ExportRepoDiagnostics returned error: %v", err)
	}

	got := export.SetupSessions[0].Steps[0].Log
	if !strings.Contains(got, repoDiagnosticLogTruncatedMarker) {
		t.Fatalf("expected truncation marker, got log length %d", len([]rune(got)))
	}
	if len([]rune(got)) > repoDiagnosticLogMaxRunes+len([]rune(repoDiagnosticLogTruncatedMarker)) {
		t.Fatalf("expected log to be bounded, got length %d", len([]rune(got)))
	}
	if strings.Contains(got, strings.Repeat("x", repoDiagnosticLogMaxRunes+1)) {
		t.Fatalf("expected log body to be truncated")
	}
}

func TestExportRepoDiagnosticsRedactsStoredLogsAgainAndOmitsSensitiveContent(t *testing.T) {
	ctx := context.Background()
	repoPath := createGoDiagnosticRepo(t)
	sourceOnlyText := "source-file-content-must-not-be-exported"
	if err := os.WriteFile(filepath.Join(repoPath, "main.go"), []byte("package main\n\nconst marker = \""+sourceOnlyText+"\"\n"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	sqliteStore := openDiagnosticTestStore(t)
	defer sqliteStore.Close()

	repo := saveDiagnosticRepo(t, sqliteStore, "https://user:repo-secret-token@github.com/example/redact", repoPath)
	session := startDiagnosticSession(t, sqliteStore, repo)
	recordDiagnosticStep(t, sqliteStore, session.ID, repo.LocalPath, "raw-log", strings.Join([]string{
		"OPENAI_API_KEY=sk-export-secret",
		"password: hunter2",
		"Authorization: Bearer abcdef12345",
		"bare key sk-exportsecret123456",
		"bare github token ghp_ZYXWVUTSRQPONMLKJIHG9876543210ZZZZ",
		"bare stripe key sk_live_ZYXWVUTSRQPONMLK",
		`"NPM_TOKEN": "quoted-secret-token"`,
		"AI prompt: full prompt text must not leak",
		"AI response: full response text must not leak",
	}, "\n"))

	app := newDiagnosticTestApp(sqliteStore)
	export, err := app.ExportRepoDiagnostics(ctx, domain.RepoDiagnosticExportRequest{InstalledRepoID: repo.ID})
	if err != nil {
		t.Fatalf("ExportRepoDiagnostics returned error: %v", err)
	}

	payload, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	got := string(payload)
	for _, leaked := range []string{
		"sk-export-secret",
		"hunter2",
		"abcdef12345",
		"sk-exportsecret123456",
		"ghp_ZYXWVUTSRQPONMLKJIHG9876543210ZZZZ",
		"sk_live_ZYXWVUTSRQPONMLK",
		"quoted-secret-token",
		"repo-secret-token",
		"full prompt text must not leak",
		"full response text must not leak",
		sourceOnlyText,
	} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected diagnostic export to omit %q, got %s", leaked, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected redaction marker in diagnostic export, got %s", got)
	}
}

func openDiagnosticTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	sqliteStore, err := store.OpenSQLiteStore(filepath.Join(t.TempDir(), "instantrepo.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	return sqliteStore
}

func newDiagnosticTestApp(sqliteStore *store.SQLiteStore) *AppService {
	app := NewAppServiceWithInstalledRepoStore(sqliteStore)
	app.detector = staticDiagnosticEnvironmentDetector{
		report: domain.EnvironmentReport{
			OS:   "test-os",
			Arch: "test-arch",
			Tools: []domain.DetectedTool{
				{Name: "go", Version: "go1.26.2", Available: true},
			},
		},
	}
	return app
}

type staticDiagnosticEnvironmentDetector struct {
	report domain.EnvironmentReport
}

func (d staticDiagnosticEnvironmentDetector) Detect() domain.EnvironmentReport {
	return d.report
}

func createGoDiagnosticRepo(t *testing.T) string {
	t.Helper()
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module example.com/diagnostic\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	return repoPath
}

func saveDiagnosticRepo(t *testing.T, sqliteStore *store.SQLiteStore, repoURL, repoPath string) domain.InstalledRepo {
	t.Helper()
	repo, err := sqliteStore.SaveInstalledRepo(context.Background(), domain.InstalledRepo{
		RawURL:         repoURL + ".git",
		NormalizedURL:  repoURL,
		LocalPath:      repoPath,
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveInstalledRepo returned error: %v", err)
	}
	return repo
}

func startDiagnosticSession(t *testing.T, sqliteStore *store.SQLiteStore, repo domain.InstalledRepo) domain.SetupSession {
	t.Helper()
	session, err := sqliteStore.StartSetupSession(context.Background(), repo.ID, repo.LocalPath)
	if err != nil {
		t.Fatalf("StartSetupSession returned error: %v", err)
	}
	return session
}

func recordDiagnosticStep(t *testing.T, sqliteStore *store.SQLiteStore, setupSessionID int64, repoPath, stepID, logContent string) domain.StepRun {
	t.Helper()
	run, err := sqliteStore.RecordStepRun(context.Background(), domain.StepRun{
		SetupSessionID: setupSessionID,
		StepID:         stepID,
		Title:          "Run setup step",
		CommandHash:    commandHash("go version"),
		CommandPreview: "go version",
		Cwd:            repoPath,
		Status:         domain.StepRunStatusSucceeded,
		ExitCode:       0,
		Duration:       "1ms",
		StartedAt:      time.Date(2026, 5, 8, 12, 1, 0, 0, time.UTC),
		FinishedAt:     time.Date(2026, 5, 8, 12, 1, 0, int(time.Millisecond), time.UTC),
	}, logContent)
	if err != nil {
		t.Fatalf("RecordStepRun returned error: %v", err)
	}
	return run
}
