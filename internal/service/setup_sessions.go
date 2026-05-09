package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"instantrepo/internal/domain"
)

const commandPreviewLimit = 160

var (
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b([A-Z0-9_]*(?:SECRET|TOKEN|PASSWORD|PASSWD|API_KEY|APIKEY|PRIVATE_KEY|CLIENT_SECRET|ACCESS_KEY)[A-Z0-9_]*)\s*=\s*("[^"]*"|'[^']*'|[^\s]+)`)
	secretKeyValuePattern   = regexp.MustCompile(`(?i)\b((?:secret|token|password|passwd|api[_-]?key|private[_-]?key|client[_-]?secret)\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s]+)`)
	quotedSecretKeyPattern  = regexp.MustCompile(`(?i)(["'][A-Z0-9_]*(?:SECRET|TOKEN|PASSWORD|PASSWD|API_KEY|APIKEY|PRIVATE_KEY|CLIENT_SECRET|ACCESS_KEY)[A-Z0-9_]*["']\s*:\s*)("[^"]*"|'[^']*'|[^\s,}]+)`)
	secretFlagPattern       = regexp.MustCompile(`(?i)(--?(?:secret|token|password|api-key|apikey|private-key|client-secret)\s+)([^\s]+)`)
	bearerTokenPattern      = regexp.MustCompile(`(?i)\b(Bearer)\s+[A-Za-z0-9._~+/-]+=*`)
	urlCredentialPattern    = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^:\s/@]+:)[^@\s]+@`)
	bareProviderKeyPattern  = regexp.MustCompile(`\b(?:sk-[A-Za-z0-9][A-Za-z0-9_-]{6,}|sk_(?:live|test)_[A-Za-z0-9]{8,}|gh[pousr]_[A-Za-z0-9_]{12,}|github_pat_[A-Za-z0-9_]{12,})\b`)
)

func (s *AppService) saveInstalledRepoForSetup(ctx context.Context, repoURL, localPath string) (domain.InstalledRepo, error) {
	if s.installedRepos == nil {
		return domain.InstalledRepo{}, fmt.Errorf("installed repo store is not configured")
	}

	now := time.Now().UTC()
	repo, err := s.installedRepos.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         repoURL,
		NormalizedURL:  normalizeRepoURL(repoURL),
		LocalPath:      localPath,
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: now,
	})
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("save installed repo for setup session: %w", err)
	}
	return repo, nil
}

func (s *AppService) setupSessionForRepo(ctx context.Context, installedRepoID int64, repoPath string) (domain.SetupSession, error) {
	s.setupSessionMu.Lock()
	defer s.setupSessionMu.Unlock()

	if s.activeSessions == nil {
		s.activeSessions = map[string]domain.SetupSession{}
	}
	if session, ok := s.activeSessions[repoPath]; ok {
		return session, nil
	}

	session, err := s.setupSessions.StartSetupSession(ctx, installedRepoID, repoPath)
	if err != nil {
		return domain.SetupSession{}, err
	}
	s.activeSessions[repoPath] = session
	return session, nil
}

func (s *AppService) recordStepRun(ctx context.Context, session domain.SetupSession, step domain.ExecutionStep, result domain.ExecutionResult, startedAt, finishedAt time.Time) error {
	status := domain.StepRunStatusSucceeded
	if !result.Succeeded {
		status = domain.StepRunStatusFailed
	}

	duration := result.Duration
	if duration == "" && !startedAt.IsZero() && !finishedAt.IsZero() {
		duration = finishedAt.Sub(startedAt).String()
	}

	run := domain.StepRun{
		SetupSessionID: session.ID,
		StepID:         step.ID,
		Title:          step.Title,
		CommandHash:    commandHash(step.Command),
		CommandPreview: commandPreview(step.Command),
		Cwd:            step.Cwd,
		Status:         status,
		ExitCode:       result.ExitCode,
		Duration:       duration,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	}

	if _, err := s.setupSessions.RecordStepRun(ctx, run, formatStepRunLog(step, result)); err != nil {
		return fmt.Errorf("record setup step run: %w", err)
	}
	return nil
}

func (s *AppService) recordGuardedSetupAction(ctx context.Context, repoURL, repoPath, title string, result domain.ExecutionResult, startedAt, finishedAt time.Time) error {
	if s.setupSessions == nil {
		return nil
	}

	installedRepo, err := s.saveInstalledRepoForSetup(ctx, repoURL, repoPath)
	if err != nil {
		return err
	}
	setupSession, err := s.setupSessionForRepo(ctx, installedRepo.ID, repoPath)
	if err != nil {
		return fmt.Errorf("start setup session: %w", err)
	}

	stepID := strings.TrimSpace(result.StepID)
	if stepID == "" {
		stepID = "setup-action"
	}
	command := strings.TrimSpace(result.Command)
	if command == "" {
		command = "instantrepo internal:" + stepID
	}
	cwd := strings.TrimSpace(result.Cwd)
	if cwd == "" {
		cwd = repoPath
	}

	step := domain.ExecutionStep{
		ID:               stepID,
		Title:            title,
		Command:          command,
		Cwd:              cwd,
		Type:             "env-setup",
		Importance:       domain.StepRequired,
		Risk:             domain.RiskLow,
		RequiresApproval: true,
	}
	if step.Title == "" {
		step.Title = "Run guarded setup action"
	}

	if err := s.recordStepRun(ctx, setupSession, step, result, startedAt, finishedAt); err != nil {
		return err
	}
	if err := s.setupSessions.CleanupSetupSessionRetention(ctx, time.Now().UTC()); err != nil {
		return fmt.Errorf("cleanup setup session retention: %w", err)
	}
	return nil
}

func failedGuardedSetupResult(stepID, command, cwd string, runErr error, startedAt, finishedAt time.Time) domain.ExecutionResult {
	duration := ""
	if !startedAt.IsZero() && !finishedAt.IsZero() {
		duration = finishedAt.Sub(startedAt).String()
	}

	return domain.ExecutionResult{
		StepID:    stepID,
		Command:   command,
		Cwd:       cwd,
		ExitCode:  1,
		Stderr:    RedactLikelySecrets(runErr.Error()),
		Duration:  duration,
		Succeeded: false,
	}
}

func commandHash(command string) string {
	sum := sha256.Sum256([]byte(command))
	return hex.EncodeToString(sum[:])
}

func commandPreview(command string) string {
	preview := strings.Join(strings.Fields(RedactLikelySecrets(command)), " ")
	if len(preview) <= commandPreviewLimit {
		return preview
	}
	return strings.TrimSpace(preview[:commandPreviewLimit])
}

func formatStepRunLog(step domain.ExecutionStep, result domain.ExecutionResult) string {
	var builder strings.Builder
	builder.WriteString("Step: ")
	builder.WriteString(step.ID)
	builder.WriteString("\nTitle: ")
	builder.WriteString(step.Title)
	builder.WriteString("\nCommand: ")
	builder.WriteString(commandPreview(step.Command))
	builder.WriteString("\nStatus: ")
	if result.Succeeded {
		builder.WriteString(domain.StepRunStatusSucceeded)
	} else {
		builder.WriteString(domain.StepRunStatusFailed)
	}
	builder.WriteString(fmt.Sprintf("\nExit code: %d\nDuration: %s\n", result.ExitCode, result.Duration))
	if strings.TrimSpace(result.Stdout) != "" {
		builder.WriteString("\n[stdout]\n")
		builder.WriteString(RedactLikelySecrets(result.Stdout))
	}
	if strings.TrimSpace(result.Stderr) != "" {
		builder.WriteString("\n[stderr]\n")
		builder.WriteString(RedactLikelySecrets(result.Stderr))
	}
	return builder.String()
}

func redactExecutionEventSink(onEvent func(ExecutionEvent)) func(ExecutionEvent) {
	if onEvent == nil {
		return nil
	}
	return func(event ExecutionEvent) {
		event.Message = RedactLikelySecrets(event.Message)
		onEvent(event)
	}
}

func RedactLikelySecrets(input string) string {
	if input == "" {
		return input
	}

	redacted := secretAssignmentPattern.ReplaceAllString(input, `${1}=[REDACTED]`)
	redacted = secretKeyValuePattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	redacted = quotedSecretKeyPattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	redacted = secretFlagPattern.ReplaceAllString(redacted, `${1}[REDACTED]`)
	redacted = bearerTokenPattern.ReplaceAllString(redacted, `${1} [REDACTED]`)
	redacted = urlCredentialPattern.ReplaceAllString(redacted, `${1}[REDACTED]@`)
	redacted = bareProviderKeyPattern.ReplaceAllString(redacted, `[REDACTED]`)
	return redacted
}
