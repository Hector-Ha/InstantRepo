package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"instantrepo/internal/domain"
)

type fakePublicRepoChecker map[string]bool

func (c fakePublicRepoChecker) IsPublic(context.Context, string) bool {
	return false
}

type fakeContributionSender struct {
	err     error
	payload []domain.EnvContributionPayload
}

func (s *fakeContributionSender) SendEnvContribution(_ context.Context, payload domain.EnvContributionPayload) error {
	s.payload = append(s.payload, payload)
	return s.err
}

func (c fakePublicRepoChecker) IsPublicRepo(_ context.Context, normalizedURL string) bool {
	return c[normalizedURL]
}

func TestEnvContributionRepoClassifierRequiresConfirmedPublicHTTPS(t *testing.T) {
	ctx := context.Background()
	checker := fakePublicRepoChecker{"https://github.com/owner/repo": true}

	public := classifyEnvContributionRepo(ctx, "https://github.com/Owner/Repo.git", checker)
	if !public.Public || public.URL != "https://github.com/owner/repo" {
		t.Fatalf("expected normalized confirmed public repo, got %+v", public)
	}

	for _, rawURL := range []string{
		"git@github.com:owner/repo.git",
		"https://token@github.com/owner/repo.git",
		"C:\\Repos\\local-only",
		"https://github.com/owner/private.git",
	} {
		got := classifyEnvContributionRepo(ctx, rawURL, checker)
		if got.Public || got.URL != "" {
			t.Fatalf("expected private/uncertain repo for %q, got %+v", rawURL, got)
		}
	}
}

func TestGitPublicRepoCheckerDisablesCredentialPrompts(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "git.log")
	writeFakeGit(t, dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("INSTANTREPO_FAKE_GIT_LOG", logPath)

	if !(gitPublicRepoChecker{}).IsPublicRepo(context.Background(), "https://github.com/owner/repo") {
		raw, _ := os.ReadFile(logPath)
		t.Fatalf("expected public check to run without credential prompts/helpers:\n%s", string(raw))
	}
}

func writeFakeGit(t *testing.T, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, "git.bat")
		script := `@echo off
setlocal
>>"%INSTANTREPO_FAKE_GIT_LOG%" echo args=%*
>>"%INSTANTREPO_FAKE_GIT_LOG%" echo terminal=%GIT_TERMINAL_PROMPT%
>>"%INSTANTREPO_FAKE_GIT_LOG%" echo gcm=%GCM_INTERACTIVE%
echo %* | findstr /C:"credential.helper=" >nul || exit /b 7
if not "%GIT_TERMINAL_PROMPT%"=="0" exit /b 8
if /I not "%GCM_INTERACTIVE%"=="never" exit /b 9
exit /b 0
`
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
		return
	}
	path := filepath.Join(dir, "git")
	script := `#!/bin/sh
{
  printf 'args=%s\n' "$*"
  printf 'terminal=%s\n' "$GIT_TERMINAL_PROMPT"
  printf 'gcm=%s\n' "$GCM_INTERACTIVE"
} >> "$INSTANTREPO_FAKE_GIT_LOG"
case " $* " in
  *" credential.helper= "*) ;;
  *) exit 7 ;;
esac
[ "$GIT_TERMINAL_PROMPT" = "0" ] || exit 8
[ "$GCM_INTERACTIVE" = "never" ] || exit 9
exit 0
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
}

func TestEnvContributionAnalysisPayloadIsValueFreeAndPublicFiltered(t *testing.T) {
	ctx := context.Background()
	repoPath := filepath.Join(t.TempDir(), "repo")
	targetPath := filepath.Join(repoPath, "api", ".env")
	resp := domain.AnalyzeResponse{
		Source: domain.RepoSource{
			Type:    "github",
			RepoURL: "https://github.com/Owner/Repo.git",
			Path:    repoPath,
		},
		Analysis: domain.RepositoryAnalysis{
			ProjectName: "repo",
			ProjectType: "node",
			RepoPath:    repoPath,
			Requirements: []domain.ToolRequirement{
				{Tool: "node", VersionConstraint: "20", Required: true},
			},
		},
		Environment: domain.EnvironmentReport{OS: "windows", Arch: "amd64"},
		Plan: domain.SetupPlan{
			Env: domain.EnvironmentConfig{
				TargetPath: targetPath,
				Variables: []domain.EnvVarRequirement{
					{Name: "OPENAI_API_KEY", SuggestedValue: "sk-real-secret", FillStrategy: "leave_blank", CurrentStatus: "user_required", Secret: true},
					{Name: "OPENAI_API_KEY", SuggestedValue: "sk-real-secret", FillStrategy: "leave_blank", CurrentStatus: "user_required", Secret: true},
					{Name: "DATABASE_URL", SuggestedValue: "postgres://user:pass@localhost/db", FillStrategy: "dev_default", CurrentStatus: "missing"},
				},
			},
		},
	}
	service := NewEnvContributionService(nil, nil)
	service.publicChecker = fakePublicRepoChecker{"https://github.com/owner/repo": true}
	payload, ok, err := service.BuildAnalysisPayload(ctx, resp, domain.EnvContributionSettings{
		PublicEnvPatternsEnabled: true,
		ConsentShown:             true,
	})
	if err != nil {
		t.Fatalf("BuildAnalysisPayload returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected public analysis payload")
	}
	if payload.Repo.URL != "https://github.com/owner/repo" {
		t.Fatalf("expected public repo identity, got %+v", payload.Repo)
	}
	if strings.Join(payload.EnvNames, ",") != "DATABASE_URL,OPENAI_API_KEY" {
		t.Fatalf("expected deduped sorted env names, got %+v", payload.EnvNames)
	}
	if len(payload.Targets) != 1 || payload.Targets[0].RelativePath != "api/.env" {
		t.Fatalf("expected relative target path, got %+v", payload.Targets)
	}
	if len(payload.Stacks) == 0 || payload.Stacks[0].Name != "node" {
		t.Fatalf("expected recognized stack metadata, got %+v", payload.Stacks)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	for _, forbidden := range []string{
		"sk-real-secret",
		"postgres://user:pass",
		repoPath,
		"branch",
		"dependencies",
		"vault",
	} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("analysis payload leaked %q:\n%s", forbidden, string(raw))
		}
	}
}

func TestEnvContributionPrivatePayloadOmitsRepoIdentity(t *testing.T) {
	ctx := context.Background()
	resp := domain.AnalyzeResponse{
		Source: domain.RepoSource{Type: "local", Path: filepath.Join(t.TempDir(), "repo")},
		Plan:   domain.SetupPlan{Env: domain.EnvironmentConfig{Variables: []domain.EnvVarRequirement{{Name: "OPENAI_API_KEY"}}}},
	}
	service := NewEnvContributionService(nil, nil)

	_, ok, err := service.BuildAnalysisPayload(ctx, resp, domain.EnvContributionSettings{
		PublicEnvPatternsEnabled: false,
		ConsentShown:             true,
	})
	if err != nil {
		t.Fatalf("BuildAnalysisPayload returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected private/local repo to be skipped by default")
	}

	payload, ok, err := service.BuildAnalysisPayload(ctx, resp, domain.EnvContributionSettings{
		PrivateLocalEnvPatternsEnabled: true,
		ConsentShown:                   true,
	})
	if err != nil {
		t.Fatalf("BuildAnalysisPayload private returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected private/local env-name payload when enabled")
	}
	if payload.Repo.URL != "" || payload.Repo.CommitSHA != "" {
		t.Fatalf("expected private payload to omit repo identity, got %+v", payload.Repo)
	}
}

func TestEnvContributionSaveOutcomePayloadIsValueFree(t *testing.T) {
	ctx := context.Background()
	repoPath := filepath.Join(t.TempDir(), "repo")
	draft := domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{{
			RelativePath: "api/.env",
			AbsolutePath: filepath.Join(repoPath, "api", ".env"),
			Values: []domain.EnvDraftValue{
				{Name: "OPENAI_API_KEY", Value: "sk-save-secret", ValueClass: domain.EnvValueClassServiceCredential, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceDraft}},
				{Name: "DATABASE_URL", Value: "", ValueClass: domain.EnvValueClassDevDefault, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceAllocator}},
			},
		}},
	}
	resp := domain.ExecuteResponse{
		Source: domain.RepoSource{Path: repoPath},
		Result: domain.ExecutionResult{Succeeded: true},
	}
	service := NewEnvContributionService(nil, nil)
	payload, ok, err := service.BuildSaveOutcomePayload(ctx, resp, draft, domain.EnvContributionSettings{
		PrivateLocalEnvPatternsEnabled: true,
		ConsentShown:                   true,
	})
	if err != nil {
		t.Fatalf("BuildSaveOutcomePayload returned error: %v", err)
	}
	if !ok || len(payload.Outcomes) != 2 {
		t.Fatalf("expected two value-free outcomes, got ok=%v payload=%+v", ok, payload)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal save payload: %v", err)
	}
	for _, forbidden := range []string{"sk-save-secret", repoPath, "vault", "branch"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("save outcome payload leaked %q:\n%s", forbidden, string(raw))
		}
	}
}

func TestEnvContributionQueuesWhenFakeSenderFails(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	if err := sqliteStore.SaveEnvContributionSettings(ctx, domain.EnvContributionSettings{
		PublicEnvPatternsEnabled: true,
		ConsentShown:             true,
	}); err != nil {
		t.Fatalf("SaveEnvContributionSettings returned error: %v", err)
	}
	sender := &fakeContributionSender{err: fmt.Errorf("offline")}
	service := NewEnvContributionService(sqliteStore, sender)
	service.publicChecker = fakePublicRepoChecker{"https://github.com/owner/repo": true}

	resp := domain.AnalyzeResponse{
		Source:      domain.RepoSource{RepoURL: "https://github.com/owner/repo.git", Path: t.TempDir()},
		Environment: domain.EnvironmentReport{OS: "windows"},
		Plan: domain.SetupPlan{Env: domain.EnvironmentConfig{Variables: []domain.EnvVarRequirement{
			{Name: "OPENAI_API_KEY", SuggestedValue: "sk-queued-secret"},
		}}},
	}
	if err := service.RecordAnalysis(ctx, resp); err != nil {
		t.Fatalf("RecordAnalysis returned error: %v", err)
	}
	status, err := sqliteStore.EnvContributionQueueStatus(ctx)
	if err != nil {
		t.Fatalf("EnvContributionQueueStatus returned error: %v", err)
	}
	if status.Count != 1 || len(sender.payload) != 1 {
		t.Fatalf("expected failed send to queue once, status=%+v sent=%d", status, len(sender.payload))
	}
	items, err := sqliteStore.EnvContributionQueueItems(ctx, 1)
	if err != nil {
		t.Fatalf("EnvContributionQueueItems returned error: %v", err)
	}
	if strings.Contains(items[0].PayloadJSON, "sk-queued-secret") {
		t.Fatalf("queued payload leaked env value:\n%s", items[0].PayloadJSON)
	}
}

func TestEnvContributionRetriesQueuedPayloadsWhenSenderRecovers(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	if _, err := sqliteStore.SaveEnvContributionQueueItem(ctx, domain.EnvContributionQueueItem{
		EventType:   domain.EnvContributionEventAnalysis,
		PayloadJSON: `{"schemaVersion":"2026-05-23","eventType":"analysis","envNames":["DATABASE_URL"]}`,
	}); err != nil {
		t.Fatalf("SaveEnvContributionQueueItem returned error: %v", err)
	}
	sender := &fakeContributionSender{}
	service := NewEnvContributionService(sqliteStore, sender)

	if err := service.RetryQueue(ctx); err != nil {
		t.Fatalf("RetryQueue returned error: %v", err)
	}
	status, err := sqliteStore.EnvContributionQueueStatus(ctx)
	if err != nil {
		t.Fatalf("EnvContributionQueueStatus returned error: %v", err)
	}
	if status.Count != 0 || len(sender.payload) != 1 {
		t.Fatalf("expected queue drained after retry, status=%+v sent=%d", status, len(sender.payload))
	}
	if strings.Join(sender.payload[0].EnvNames, ",") != "DATABASE_URL" {
		t.Fatalf("expected queued payload to be sent, got %+v", sender.payload[0])
	}
}

func TestEnvContributionDisabledSettingsDoNotQueue(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	if err := sqliteStore.SaveEnvContributionSettings(ctx, domain.EnvContributionSettings{
		PublicEnvPatternsEnabled: false,
		ConsentShown:             true,
	}); err != nil {
		t.Fatalf("SaveEnvContributionSettings returned error: %v", err)
	}
	service := NewEnvContributionService(sqliteStore, &fakeContributionSender{err: fmt.Errorf("offline")})
	service.publicChecker = fakePublicRepoChecker{"https://github.com/owner/repo": true}

	if err := service.RecordAnalysis(ctx, domain.AnalyzeResponse{
		Source: domain.RepoSource{RepoURL: "https://github.com/owner/repo.git", Path: t.TempDir()},
		Plan:   domain.SetupPlan{Env: domain.EnvironmentConfig{Variables: []domain.EnvVarRequirement{{Name: "OPENAI_API_KEY"}}}},
	}); err != nil {
		t.Fatalf("RecordAnalysis returned error: %v", err)
	}
	status, err := sqliteStore.EnvContributionQueueStatus(ctx)
	if err != nil {
		t.Fatalf("EnvContributionQueueStatus returned error: %v", err)
	}
	if status.Count != 0 {
		t.Fatalf("expected disabled settings not to queue, got %+v", status)
	}
}
