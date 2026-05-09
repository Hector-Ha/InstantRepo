package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"instantrepo/internal/domain"
)

const (
	repoDiagnosticSchemaVersion = "repo-diagnostic-export/v1"
	repoDiagnosticAppVersion    = "dev"
	repoDiagnosticLogMaxRunes   = 4096
)

const repoDiagnosticLogTruncatedMarker = "\n[TRUNCATED]"

type repoDiagnosticStore interface {
	InstalledRepoByID(ctx context.Context, id int64) (domain.InstalledRepo, error)
	InstalledRepoByLocalPath(ctx context.Context, localPath string) (domain.InstalledRepo, error)
	SetupSessionsByInstalledRepoID(ctx context.Context, installedRepoID int64) ([]domain.SetupSession, error)
	StepRunsBySetupSessionID(ctx context.Context, setupSessionID int64) ([]domain.StepRun, error)
}

type repoDiagnosticSanitizer struct {
	logMaxRunes     int
	truncatedMarker string
}

func newRepoDiagnosticSanitizer() repoDiagnosticSanitizer {
	return repoDiagnosticSanitizer{
		logMaxRunes:     repoDiagnosticLogMaxRunes,
		truncatedMarker: repoDiagnosticLogTruncatedMarker,
	}
}

func (s *AppService) ExportRepoDiagnostics(ctx context.Context, req domain.RepoDiagnosticExportRequest) (domain.RepoDiagnosticExport, error) {
	diagnostics, ok := s.installedRepos.(repoDiagnosticStore)
	if !ok {
		return domain.RepoDiagnosticExport{}, fmt.Errorf("repo diagnostic export store is not configured")
	}

	repo, err := s.repoForDiagnosticExport(ctx, diagnostics, req)
	if err != nil {
		return domain.RepoDiagnosticExport{}, err
	}

	environment := s.detector.Detect()
	analysis, err := s.analyzer.Analyze(repo.LocalPath, environment)
	if err != nil {
		return domain.RepoDiagnosticExport{}, fmt.Errorf("analyze repo for diagnostic export: %w", err)
	}
	plan := s.planner.BuildPlan(analysis, environment)

	sessions, err := diagnostics.SetupSessionsByInstalledRepoID(ctx, repo.ID)
	if err != nil {
		return domain.RepoDiagnosticExport{}, fmt.Errorf("list setup sessions for diagnostic export: %w", err)
	}

	sanitizer := newRepoDiagnosticSanitizer()
	exportSessions := make([]domain.RepoDiagnosticSetupSession, 0, len(sessions))
	for _, session := range sessions {
		steps, err := s.diagnosticStepsForSession(ctx, diagnostics, sanitizer, session.ID)
		if err != nil {
			return domain.RepoDiagnosticExport{}, err
		}
		exportSessions = append(exportSessions, domain.RepoDiagnosticSetupSession{
			ID:              session.ID,
			InstalledRepoID: session.InstalledRepoID,
			RepoPath:        session.RepoPath,
			Status:          session.Status,
			CreatedAt:       session.CreatedAt,
			UpdatedAt:       session.UpdatedAt,
			Steps:           steps,
		})
	}

	return domain.RepoDiagnosticExport{
		SchemaVersion: repoDiagnosticSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Repo:          sanitizer.repoIdentity(repo),
		App: domain.RepoDiagnosticAppInfo{
			Name:    "InstantRepo",
			Version: repoDiagnosticAppVersion,
		},
		Environment: domain.RepoDiagnosticEnvironment{
			OS:    environment.OS,
			Arch:  environment.Arch,
			Tools: environment.Tools,
		},
		Analysis:      diagnosticAnalysisSummary(analysis),
		SetupPlan:     diagnosticSetupPlanSummary(plan, sanitizer),
		SetupSessions: exportSessions,
		AIReview: domain.RepoDiagnosticAIReviewMetadata{
			Available: false,
			Entries:   []domain.RepoDiagnosticAIReviewEntryMetadata{},
		},
	}, nil
}

func (s *AppService) repoForDiagnosticExport(ctx context.Context, diagnostics repoDiagnosticStore, req domain.RepoDiagnosticExportRequest) (domain.InstalledRepo, error) {
	if req.InstalledRepoID > 0 {
		repo, err := diagnostics.InstalledRepoByID(ctx, req.InstalledRepoID)
		if err != nil {
			return domain.InstalledRepo{}, fmt.Errorf("find installed repo for diagnostic export: %w", err)
		}
		return repo, nil
	}

	localPath := strings.TrimSpace(req.LocalPath)
	if localPath == "" {
		return domain.InstalledRepo{}, fmt.Errorf("installedRepoId or localPath is required")
	}
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("resolve diagnostic export path: %w", err)
	}
	repo, err := diagnostics.InstalledRepoByLocalPath(ctx, absPath)
	if err != nil {
		return domain.InstalledRepo{}, fmt.Errorf("find installed repo for diagnostic export: %w", err)
	}
	return repo, nil
}

func (s *AppService) diagnosticStepsForSession(ctx context.Context, diagnostics repoDiagnosticStore, sanitizer repoDiagnosticSanitizer, setupSessionID int64) ([]domain.RepoDiagnosticStep, error) {
	runs, err := diagnostics.StepRunsBySetupSessionID(ctx, setupSessionID)
	if err != nil {
		return nil, fmt.Errorf("list step runs for diagnostic export: %w", err)
	}

	steps := make([]domain.RepoDiagnosticStep, 0, len(runs))
	for _, run := range runs {
		logContent := ""
		if run.LogPath != "" {
			raw, err := os.ReadFile(run.LogPath)
			if err != nil {
				return nil, fmt.Errorf("read setup log for diagnostic export: %w", err)
			}
			logContent = string(raw)
		}
		steps = append(steps, domain.RepoDiagnosticStep{
			ID:             run.ID,
			SetupSessionID: run.SetupSessionID,
			StepID:         run.StepID,
			Title:          run.Title,
			CommandHash:    run.CommandHash,
			CommandPreview: sanitizer.commandPreview(run.CommandPreview),
			Cwd:            run.Cwd,
			Status:         run.Status,
			ExitCode:       run.ExitCode,
			Duration:       run.Duration,
			Log:            sanitizer.log(logContent),
			StartedAt:      run.StartedAt,
			FinishedAt:     run.FinishedAt,
			CreatedAt:      run.CreatedAt,
			UpdatedAt:      run.UpdatedAt,
		})
	}
	return steps, nil
}

func (s repoDiagnosticSanitizer) repoIdentity(repo domain.InstalledRepo) domain.RepoDiagnosticRepoIdentity {
	return domain.RepoDiagnosticRepoIdentity{
		ID:             repo.ID,
		RawURL:         s.repoURL(repo.RawURL),
		NormalizedURL:  s.repoURL(repo.NormalizedURL),
		LocalPath:      repo.LocalPath,
		Status:         repo.Status,
		CreatedAt:      repo.CreatedAt,
		UpdatedAt:      repo.UpdatedAt,
		LastAnalyzedAt: repo.LastAnalyzedAt,
	}
}

func (s repoDiagnosticSanitizer) repoURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return RedactLikelySecrets(raw)
	}
	parsed.User = nil
	return parsed.String()
}

func (s repoDiagnosticSanitizer) commandPreview(command string) string {
	return commandPreview(command)
}

func diagnosticAnalysisSummary(analysis domain.RepositoryAnalysis) domain.RepoDiagnosticAnalysisSummary {
	return domain.RepoDiagnosticAnalysisSummary{
		ProjectName:  analysis.ProjectName,
		ProjectType:  analysis.ProjectType,
		Confidence:   analysis.Confidence,
		Evidence:     append([]string(nil), analysis.Evidence...),
		Requirements: append([]domain.ToolRequirement(nil), analysis.Requirements...),
		EnvVariables: diagnosticEnvVars(analysis.Env.Variables),
		Services:     append([]domain.ServiceDependency(nil), analysis.Services...),
		Unknowns:     append([]string(nil), analysis.Unknowns...),
	}
}

func diagnosticSetupPlanSummary(plan domain.SetupPlan, sanitizer repoDiagnosticSanitizer) domain.RepoDiagnosticSetupPlanSummary {
	steps := make([]domain.RepoDiagnosticPlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, domain.RepoDiagnosticPlanStep{
			ID:               step.ID,
			Title:            step.Title,
			CommandPreview:   sanitizer.commandPreview(step.Command),
			Type:             step.Type,
			Importance:       step.Importance,
			Risk:             step.Risk,
			RequiresApproval: step.RequiresApproval,
			EvidenceSource:   step.EvidenceSource,
			Confidence:       step.Confidence,
			Reason:           step.Reason,
		})
	}

	return domain.RepoDiagnosticSetupPlanSummary{
		ProjectName:  plan.ProjectName,
		ProjectType:  plan.ProjectType,
		Confidence:   plan.Confidence,
		Evidence:     append([]string(nil), plan.Evidence...),
		Gaps:         append([]domain.RequirementGap(nil), plan.Gaps...),
		EnvVariables: diagnosticEnvVars(plan.Env.Variables),
		Services:     append([]domain.ServiceDependency(nil), plan.Services...),
		Steps:        steps,
		Safety:       plan.Safety,
		Unknowns:     append([]string(nil), plan.Unknowns...),
	}
}

func diagnosticEnvVars(vars []domain.EnvVarRequirement) []domain.RepoDiagnosticEnvVar {
	result := make([]domain.RepoDiagnosticEnvVar, 0, len(vars))
	for _, item := range vars {
		result = append(result, domain.RepoDiagnosticEnvVar{
			Name:          item.Name,
			Source:        item.Source,
			Required:      item.Required,
			Secret:        item.Secret,
			CurrentStatus: item.CurrentStatus,
			Service:       item.Service,
			TargetDir:     item.TargetDir,
		})
	}
	return result
}

func (s repoDiagnosticSanitizer) log(logContent string) string {
	redacted := RedactLikelySecrets(logContent)
	redacted = s.aiLines(redacted)
	return s.truncateLog(redacted)
}

func (s repoDiagnosticSanitizer) aiLines(input string) string {
	if input == "" {
		return input
	}

	var builder strings.Builder
	for _, line := range strings.SplitAfter(input, "\n") {
		lineBody := line
		ending := ""
		if strings.HasSuffix(lineBody, "\n") {
			ending = "\n"
			lineBody = strings.TrimSuffix(lineBody, "\n")
			if strings.HasSuffix(lineBody, "\r") {
				ending = "\r\n"
				lineBody = strings.TrimSuffix(lineBody, "\r")
			}
		}

		lower := strings.ToLower(lineBody)
		if strings.Contains(lower, "ai prompt") || strings.Contains(lower, "ai response") || strings.Contains(lower, "full prompt") || strings.Contains(lower, "full response") {
			builder.WriteString(s.lineValue(lineBody))
			builder.WriteString(ending)
			continue
		}
		builder.WriteString(line)
	}
	return builder.String()
}

func (s repoDiagnosticSanitizer) lineValue(line string) string {
	idx := strings.IndexAny(line, ":=")
	if idx < 0 {
		return "[REDACTED]"
	}
	return strings.TrimRight(line[:idx+1], " \t") + " [REDACTED]"
}

func (s repoDiagnosticSanitizer) truncateLog(logContent string) string {
	runes := []rune(logContent)
	if len(runes) <= s.logMaxRunes {
		return logContent
	}
	return string(runes[:s.logMaxRunes]) + s.truncatedMarker
}
