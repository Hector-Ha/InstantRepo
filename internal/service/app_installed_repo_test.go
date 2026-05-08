package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"instantrepo/internal/domain"
	"instantrepo/internal/store"
)

type recordingInstalledRepoStore struct {
	repos         []domain.InstalledRepo
	sessions      []domain.SetupSession
	stepRuns      []domain.StepRun
	logs          []string
	nextRepoID    int64
	nextSessionID int64
	nextStepRunID int64
}

func (s *recordingInstalledRepoStore) SaveInstalledRepo(_ context.Context, repo domain.InstalledRepo) (domain.InstalledRepo, error) {
	if repo.ID == 0 {
		s.nextRepoID++
		repo.ID = s.nextRepoID
	}
	s.repos = append(s.repos, repo)
	return repo, nil
}

func (s *recordingInstalledRepoStore) StartSetupSession(_ context.Context, installedRepoID int64, repoPath string) (domain.SetupSession, error) {
	s.nextSessionID++
	session := domain.SetupSession{
		ID:              s.nextSessionID,
		InstalledRepoID: installedRepoID,
		RepoPath:        repoPath,
		Status:          domain.SetupSessionStatusRunning,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	s.sessions = append(s.sessions, session)
	return session, nil
}

func (s *recordingInstalledRepoStore) RecordStepRun(_ context.Context, run domain.StepRun, logContent string) (domain.StepRun, error) {
	s.nextStepRunID++
	run.ID = s.nextStepRunID
	if logContent != "" {
		run.LogPath = "memory://setup-log"
		s.logs = append(s.logs, logContent)
	}
	s.stepRuns = append(s.stepRuns, run)
	return run, nil
}

func (s *recordingInstalledRepoStore) CleanupSetupSessionRetention(_ context.Context, _ time.Time) error {
	return nil
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

func TestExecutePersistsSuccessfulGuardedSetupStep(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("## Run\n\n```sh\ngo version\n```\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)

	analyzed, err := app.Analyze(context.Background(), domain.AnalyzeRequest{LocalPath: repoPath})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	stepID := stepIDForCommand(t, analyzed.Plan.Steps, "go version")

	resp, err := app.Execute(context.Background(), domain.ExecuteRequest{
		LocalPath:     repoPath,
		StepID:        stepID,
		ApproveRisky:  true,
		ExecutionMode: "guarded",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !resp.Result.Succeeded {
		t.Fatalf("expected successful step result, got %+v", resp.Result)
	}

	if len(setupStore.sessions) != 1 {
		t.Fatalf("expected one setup session, got %d", len(setupStore.sessions))
	}
	session := setupStore.sessions[0]
	if session.InstalledRepoID == 0 {
		t.Fatalf("expected setup session to belong to installed repo")
	}
	if session.RepoPath != resp.Source.Path {
		t.Fatalf("expected session repo path %q, got %q", resp.Source.Path, session.RepoPath)
	}

	if len(setupStore.stepRuns) != 1 {
		t.Fatalf("expected one step run, got %d", len(setupStore.stepRuns))
	}
	run := setupStore.stepRuns[0]
	if run.SetupSessionID != session.ID {
		t.Fatalf("expected step run session ID %d, got %d", session.ID, run.SetupSessionID)
	}
	if run.StepID != stepID {
		t.Fatalf("expected step ID %q, got %q", stepID, run.StepID)
	}
	if run.Status != domain.StepRunStatusSucceeded {
		t.Fatalf("expected succeeded status, got %q", run.Status)
	}
	if run.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", run.ExitCode)
	}
	if run.CommandHash == "" || run.CommandPreview != "go version" {
		t.Fatalf("expected command hash and preview, got hash %q preview %q", run.CommandHash, run.CommandPreview)
	}
	if run.Duration == "" || run.StartedAt.IsZero() || run.FinishedAt.IsZero() {
		t.Fatalf("expected duration and timestamps, got %+v", run)
	}
	if run.LogPath == "" || len(setupStore.logs) != 1 {
		t.Fatalf("expected persistent log reference and content, got run %+v logs %d", run, len(setupStore.logs))
	}
	if !strings.Contains(setupStore.logs[0], "go version") {
		t.Fatalf("expected persistent log to include command output, got %q", setupStore.logs[0])
	}
}

func TestExecutePersistsFailedGuardedSetupStep(t *testing.T) {
	repoPath := t.TempDir()
	command := "go test ./definitely-missing-package"
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("## Run\n\n```sh\n"+command+"\n```\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)

	analyzed, err := app.Analyze(context.Background(), domain.AnalyzeRequest{LocalPath: repoPath})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	stepID := stepIDForCommand(t, analyzed.Plan.Steps, command)

	resp, err := app.Execute(context.Background(), domain.ExecuteRequest{
		LocalPath:    repoPath,
		StepID:       stepID,
		ApproveRisky: true,
	})
	if err != nil {
		t.Fatalf("Execute returned error for failed process: %v", err)
	}
	if resp.Result.Succeeded {
		t.Fatalf("expected failed step result, got %+v", resp.Result)
	}

	if len(setupStore.stepRuns) != 1 {
		t.Fatalf("expected one step run, got %d", len(setupStore.stepRuns))
	}
	run := setupStore.stepRuns[0]
	if run.Status != domain.StepRunStatusFailed {
		t.Fatalf("expected failed status, got %q", run.Status)
	}
	if run.ExitCode == 0 {
		t.Fatalf("expected nonzero exit code for failed command")
	}
	if run.CommandHash == "" || run.CommandPreview != command {
		t.Fatalf("expected command hash and preview, got hash %q preview %q", run.CommandHash, run.CommandPreview)
	}
	if run.Duration == "" || run.StartedAt.IsZero() || run.FinishedAt.IsZero() {
		t.Fatalf("expected duration and timestamps, got %+v", run)
	}
	if run.LogPath == "" || len(setupStore.logs) != 1 {
		t.Fatalf("expected persistent log reference and content, got run %+v logs %d", run, len(setupStore.logs))
	}
}

func TestExecuteRedactsSecretsFromLiveAndPersistentLogs(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module example.com/redact\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "main.go"), []byte(`package main

import "fmt"

func main() {
	fmt.Println("OPENAI_API_KEY=sk-live-secret")
	fmt.Println("token: abc123")
}
`), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)
	events := make([]ExecutionEvent, 0, 4)

	resp, err := app.ExecuteWithEvents(context.Background(), domain.ExecuteRequest{
		LocalPath:    repoPath,
		StepID:       "go-run",
		ApproveRisky: true,
	}, func(event ExecutionEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("ExecuteWithEvents returned error: %v", err)
	}
	if !resp.Result.Succeeded {
		t.Fatalf("expected successful result, got %+v", resp.Result)
	}

	combinedEvents := ""
	for _, event := range events {
		combinedEvents += event.Message + "\n"
	}
	if strings.Contains(combinedEvents, "sk-live-secret") || strings.Contains(combinedEvents, "abc123") {
		t.Fatalf("expected live events to redact secrets, got %q", combinedEvents)
	}
	if strings.Contains(resp.Result.Stdout, "sk-live-secret") || strings.Contains(resp.Result.Stdout, "abc123") {
		t.Fatalf("expected result stdout to redact secrets, got %q", resp.Result.Stdout)
	}
	if len(setupStore.logs) != 1 {
		t.Fatalf("expected one persistent log, got %d", len(setupStore.logs))
	}
	if strings.Contains(setupStore.logs[0], "sk-live-secret") || strings.Contains(setupStore.logs[0], "abc123") {
		t.Fatalf("expected persistent log to redact secrets, got %q", setupStore.logs[0])
	}
	if !strings.Contains(setupStore.logs[0], "[REDACTED]") {
		t.Fatalf("expected redaction marker in persistent log, got %q", setupStore.logs[0])
	}
}

func TestExecuteReusesSetupSessionForSameRepoGuardedActions(t *testing.T) {
	repoPath := t.TempDir()
	firstCommand := "go version"
	secondCommand := "go env GOVERSION"
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("## Run\n\n```sh\n"+firstCommand+"\n"+secondCommand+"\n```\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)

	analyzed, err := app.Analyze(context.Background(), domain.AnalyzeRequest{LocalPath: repoPath})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	firstStepID := stepIDForCommand(t, analyzed.Plan.Steps, firstCommand)
	secondStepID := stepIDForCommand(t, analyzed.Plan.Steps, secondCommand)

	for _, stepID := range []string{firstStepID, secondStepID} {
		if _, err := app.Execute(context.Background(), domain.ExecuteRequest{
			LocalPath:    repoPath,
			StepID:       stepID,
			ApproveRisky: true,
		}); err != nil {
			t.Fatalf("Execute returned error for step %q: %v", stepID, err)
		}
	}

	if len(setupStore.sessions) != 1 {
		t.Fatalf("expected one setup session reused for repo, got %d", len(setupStore.sessions))
	}
	if len(setupStore.stepRuns) != 2 {
		t.Fatalf("expected two step runs, got %d", len(setupStore.stepRuns))
	}
	sessionID := setupStore.sessions[0].ID
	for _, run := range setupStore.stepRuns {
		if run.SetupSessionID != sessionID {
			t.Fatalf("expected step run session ID %d, got %d", sessionID, run.SetupSessionID)
		}
	}
}

func TestSaveRawEnvPersistsGuardedSetupStepWithoutRawEnvContent(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\nDATABASE_URL=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)
	rawEnv := "OPENAI_API_KEY=raw-secret\nDATABASE_URL=postgres://user:pass@localhost:5432/app\n"

	resp, err := app.SaveRawEnv(context.Background(), repoPath, rawEnv)
	if err != nil {
		t.Fatalf("SaveRawEnv returned error: %v", err)
	}
	if !resp.Result.Succeeded {
		t.Fatalf("expected env save to succeed, got %+v", resp.Result)
	}

	if len(setupStore.sessions) != 1 {
		t.Fatalf("expected one setup session, got %d", len(setupStore.sessions))
	}
	if len(setupStore.stepRuns) != 1 {
		t.Fatalf("expected one step run, got %d", len(setupStore.stepRuns))
	}
	run := setupStore.stepRuns[0]
	if run.SetupSessionID != setupStore.sessions[0].ID {
		t.Fatalf("expected step run session ID %d, got %d", setupStore.sessions[0].ID, run.SetupSessionID)
	}
	if run.StepID != resp.Result.StepID {
		t.Fatalf("expected step ID %q, got %q", resp.Result.StepID, run.StepID)
	}
	if run.Status != domain.StepRunStatusSucceeded || run.ExitCode != 0 {
		t.Fatalf("expected successful step run, got %+v", run)
	}
	if run.CommandHash == "" || run.CommandPreview == "" {
		t.Fatalf("expected command metadata, got %+v", run)
	}
	if run.LogPath == "" || len(setupStore.logs) != 1 {
		t.Fatalf("expected log ref and content, got run %+v logs %d", run, len(setupStore.logs))
	}
	if strings.Contains(setupStore.logs[0], "raw-secret") || strings.Contains(setupStore.logs[0], "user:pass") {
		t.Fatalf("expected persistent log to omit raw env content, got %q", setupStore.logs[0])
	}
}

func TestSaveRawEnvFailurePersistsFailedGuardedSetupStepWithoutRawEnvContent(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	if err := os.Mkdir(filepath.Join(repoPath, ".env"), 0o755); err != nil {
		t.Fatalf("create blocking env directory: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)
	rawEnv := "OPENAI_API_KEY=raw-secret\n"

	_, err := app.SaveRawEnv(context.Background(), repoPath, rawEnv)
	if err == nil {
		t.Fatalf("expected SaveRawEnv to fail")
	}

	if len(setupStore.sessions) != 1 {
		t.Fatalf("expected one setup session, got %d", len(setupStore.sessions))
	}
	if len(setupStore.stepRuns) != 1 {
		t.Fatalf("expected one failed step run, got %d", len(setupStore.stepRuns))
	}
	run := setupStore.stepRuns[0]
	if run.StepID != "save-env-file" {
		t.Fatalf("expected save-env-file step, got %q", run.StepID)
	}
	if run.Status != domain.StepRunStatusFailed {
		t.Fatalf("expected failed status, got %+v", run)
	}
	if run.CommandHash == "" || run.CommandPreview == "" || run.Duration == "" || run.StartedAt.IsZero() || run.FinishedAt.IsZero() {
		t.Fatalf("expected failed run metadata, got %+v", run)
	}
	if run.LogPath == "" || len(setupStore.logs) != 1 {
		t.Fatalf("expected failed run log ref and content, got run %+v logs %d", run, len(setupStore.logs))
	}
	if strings.Contains(setupStore.logs[0], "raw-secret") {
		t.Fatalf("expected failed log to omit raw env content, got %q", setupStore.logs[0])
	}
}

func TestSaveEnvValuesPersistsGuardedSetupStepWithoutSecretValues(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\nDATABASE_URL=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)

	resp, err := app.SaveEnvValues(context.Background(), repoPath, map[string]string{
		"OPENAI_API_KEY": "value-secret",
		"DATABASE_URL":   "postgres://user:pass@localhost:5432/app",
	})
	if err != nil {
		t.Fatalf("SaveEnvValues returned error: %v", err)
	}
	if !resp.Result.Succeeded {
		t.Fatalf("expected env values save to succeed, got %+v", resp.Result)
	}

	if len(setupStore.sessions) != 1 {
		t.Fatalf("expected one setup session, got %d", len(setupStore.sessions))
	}
	if len(setupStore.stepRuns) != 1 {
		t.Fatalf("expected one step run, got %d", len(setupStore.stepRuns))
	}
	run := setupStore.stepRuns[0]
	if run.StepID != "create-env-file" {
		t.Fatalf("expected create-env-file step, got %q", run.StepID)
	}
	if run.LogPath == "" || len(setupStore.logs) != 1 {
		t.Fatalf("expected log ref and content, got run %+v logs %d", run, len(setupStore.logs))
	}
	if strings.Contains(setupStore.logs[0], "value-secret") || strings.Contains(setupStore.logs[0], "user:pass") {
		t.Fatalf("expected persistent log to omit secret values, got %q", setupStore.logs[0])
	}
}

func TestSaveEnvValuesFailurePersistsFailedGuardedSetupStepWithoutSecretValues(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\nDATABASE_URL=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	if err := os.Mkdir(filepath.Join(repoPath, ".env"), 0o755); err != nil {
		t.Fatalf("create blocking env directory: %v", err)
	}

	setupStore := &recordingInstalledRepoStore{}
	app := NewAppServiceWithInstalledRepoStore(setupStore)

	_, err := app.SaveEnvValues(context.Background(), repoPath, map[string]string{
		"OPENAI_API_KEY": "value-secret",
		"DATABASE_URL":   "postgres://user:pass@localhost:5432/app",
	})
	if err == nil {
		t.Fatalf("expected SaveEnvValues to fail")
	}

	if len(setupStore.sessions) != 1 {
		t.Fatalf("expected one setup session, got %d", len(setupStore.sessions))
	}
	if len(setupStore.stepRuns) != 1 {
		t.Fatalf("expected one failed step run, got %d", len(setupStore.stepRuns))
	}
	run := setupStore.stepRuns[0]
	if run.StepID != "create-env-file" {
		t.Fatalf("expected create-env-file step, got %q", run.StepID)
	}
	if run.Status != domain.StepRunStatusFailed {
		t.Fatalf("expected failed status, got %+v", run)
	}
	if run.CommandHash == "" || run.CommandPreview == "" || run.Duration == "" || run.StartedAt.IsZero() || run.FinishedAt.IsZero() {
		t.Fatalf("expected failed run metadata, got %+v", run)
	}
	if run.LogPath == "" || len(setupStore.logs) != 1 {
		t.Fatalf("expected failed run log ref and content, got run %+v logs %d", run, len(setupStore.logs))
	}
	if strings.Contains(setupStore.logs[0], "value-secret") || strings.Contains(setupStore.logs[0], "user:pass") {
		t.Fatalf("expected failed log to omit secret values, got %q", setupStore.logs[0])
	}
}

func stepIDForCommand(t *testing.T, steps []domain.ExecutionStep, command string) string {
	t.Helper()
	for _, step := range steps {
		if step.Command == command {
			return step.ID
		}
	}
	t.Fatalf("step command %q not found in %+v", command, steps)
	return ""
}
