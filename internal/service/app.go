package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"instantrepo/internal/analyzer"
	"instantrepo/internal/detector"
	"instantrepo/internal/domain"
	"instantrepo/internal/store"
)

type AppService struct {
	fetcher        *RepoFetcher
	analyzer       *analyzer.RepositoryAnalyzer
	detector       EnvironmentDetector
	planner        *Planner
	executor       *Executor
	envDrafts      *EnvDraftManager
	vault          *EnvVaultService
	installedRepos InstalledRepoStore
	setupRecorder  *setupSessionRecorder
	disk           DiskChecker
}

type EnvironmentDetector interface {
	Detect() domain.EnvironmentReport
}

type InstalledRepoStore interface {
	SaveInstalledRepo(ctx context.Context, repo domain.InstalledRepo) (domain.InstalledRepo, error)
}

type InstalledRepoLookup interface {
	InstalledRepoByNormalizedURL(ctx context.Context, normalizedURL string) (domain.InstalledRepo, error)
	InstalledRepoByLocalPath(ctx context.Context, localPath string) (domain.InstalledRepo, error)
}

type SetupSessionStore interface {
	StartSetupSession(ctx context.Context, installedRepoID int64, repoPath string) (domain.SetupSession, error)
	RecordStepRun(ctx context.Context, run domain.StepRun, logContent string) (domain.StepRun, error)
	CleanupSetupSessionRetention(ctx context.Context, now time.Time) error
}

func NewAppService() *AppService {
	installedRepos, err := store.OpenDefaultSQLiteStore()
	if err != nil {
		return NewAppServiceWithInstalledRepoStore(nil)
	}
	return NewAppServiceWithInstalledRepoStore(installedRepos)
}

func NewAppServiceWithInstalledRepoStore(installedRepos InstalledRepoStore) *AppService {
	app := &AppService{
		fetcher:        NewRepoFetcher(),
		analyzer:       analyzer.NewRepositoryAnalyzer(),
		detector:       detector.NewEnvironmentDetector(),
		planner:        NewPlanner(),
		executor:       NewExecutor(),
		envDrafts:      NewEnvDraftManager(),
		installedRepos: installedRepos,
		setupRecorder:  newSetupSessionRecorder(installedRepos),
		disk:           osDiskChecker{},
	}
	if vaultStore, ok := installedRepos.(EnvVaultStore); ok {
		app.vault = NewEnvVaultService(vaultStore, NewOSCredentialStore())
	}
	return app
}

func (s *AppService) Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.AnalyzeResponse, error) {
	req.RepoURL = strings.TrimSpace(req.RepoURL)
	req.LocalPath = strings.TrimSpace(req.LocalPath)

	if req.RepoURL == "" && req.LocalPath == "" {
		return domain.AnalyzeResponse{}, fmt.Errorf("repoUrl or localPath is required")
	}

	repoPath := req.LocalPath
	sourceType := "local"
	cleanup := func() {}

	if req.RepoURL != "" {
		sourceType = inferRepoSourceType(req.RepoURL)
	}

	if req.RepoURL != "" && req.LocalPath == "" {
		clonedPath, release, err := s.fetcher.Clone(ctx, req.RepoURL)
		if err != nil {
			return domain.AnalyzeResponse{}, err
		}
		repoPath = clonedPath
		cleanup = release
	}
	defer cleanup()

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return domain.AnalyzeResponse{}, fmt.Errorf("resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return domain.AnalyzeResponse{}, fmt.Errorf("repository path not found: %w", err)
	}

	environment := s.detector.Detect()

	analysis, err := s.analyzer.Analyze(absPath, environment)
	if err != nil {
		return domain.AnalyzeResponse{}, err
	}

	plan := s.planner.BuildPlan(analysis, environment)

	if err := s.persistAnalyzedRepo(ctx, req.RepoURL, absPath); err != nil {
		return domain.AnalyzeResponse{}, err
	}

	return domain.AnalyzeResponse{
		Source: domain.RepoSource{
			Type:    sourceType,
			RepoURL: req.RepoURL,
			Path:    absPath,
		},
		Analysis:    analysis,
		Environment: environment,
		Plan:        plan,
	}, nil
}

func (s *AppService) persistAnalyzedRepo(ctx context.Context, repoURL, localPath string) error {
	if s.installedRepos == nil {
		return nil
	}

	now := time.Now().UTC()
	_, err := s.installedRepos.SaveInstalledRepo(ctx, domain.InstalledRepo{
		RawURL:         repoURL,
		NormalizedURL:  normalizeRepoURL(repoURL),
		LocalPath:      localPath,
		Status:         domain.InstalledRepoStatusAnalyzed,
		LastAnalyzedAt: now,
	})
	if err != nil {
		return fmt.Errorf("save installed repo: %w", err)
	}
	return nil
}

func (s *AppService) ImportRepository(ctx context.Context, repoURL, destinationRoot string) (domain.AnalyzeResponse, error) {
	repoURL = strings.TrimSpace(repoURL)
	destinationRoot = strings.TrimSpace(destinationRoot)

	if repoURL == "" {
		return domain.AnalyzeResponse{}, fmt.Errorf("repo URL is required")
	}

	clonedPath, err := s.fetcher.CloneTo(ctx, repoURL, destinationRoot)
	if err != nil {
		return domain.AnalyzeResponse{}, err
	}

	resp, err := s.Analyze(ctx, domain.AnalyzeRequest{RepoURL: repoURL, LocalPath: clonedPath})
	if err != nil {
		return domain.AnalyzeResponse{}, err
	}
	resp.Source = domain.RepoSource{
		Type:    inferRepoSourceType(repoURL),
		RepoURL: repoURL,
		Path:    clonedPath,
	}
	return resp, nil
}

func (s *AppService) Execute(ctx context.Context, req domain.ExecuteRequest) (domain.ExecuteResponse, error) {
	return s.ExecuteWithEvents(ctx, req, nil)
}

func (s *AppService) ExecuteWithEvents(ctx context.Context, req domain.ExecuteRequest, onEvent func(ExecutionEvent)) (domain.ExecuteResponse, error) {
	req.RepoURL = strings.TrimSpace(req.RepoURL)
	req.LocalPath = strings.TrimSpace(req.LocalPath)
	req.StepID = strings.TrimSpace(req.StepID)

	if req.StepID == "" {
		return domain.ExecuteResponse{}, fmt.Errorf("stepId is required")
	}

	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{
		RepoURL:   req.RepoURL,
		LocalPath: req.LocalPath,
	})
	if err != nil {
		return domain.ExecuteResponse{}, err
	}

	var selected *domain.ExecutionStep
	for i := range analyzeResp.Plan.Steps {
		if analyzeResp.Plan.Steps[i].ID == req.StepID {
			selected = &analyzeResp.Plan.Steps[i]
			break
		}
	}
	if selected == nil {
		return domain.ExecuteResponse{}, fmt.Errorf("step %q not found", req.StepID)
	}

	if selected.RequiresApproval && !req.ApproveRisky {
		return domain.ExecuteResponse{}, fmt.Errorf("step %q requires approval; retry with approveRisky=true", req.StepID)
	}

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var setupSession domain.SetupSession
	if s.setupRecorder != nil {
		setupSession, err = s.setupRecorder.start(ctx, req.RepoURL, analyzeResp.Source.Path)
		if err != nil {
			return domain.ExecuteResponse{}, err
		}
	}

	startedAt := time.Now().UTC()
	var result domain.ExecutionResult
	switch selected.Type {
	case "env-setup":
		result, err = s.prepareEnvSetup(ctx, analyzeResp.Analysis, startedAt)
	default:
		result, err = s.executor.RunStepWithEvents(runCtx, *selected, redactExecutionEventSink(onEvent))
	}
	finishedAt := time.Now().UTC()
	if err != nil {
		return domain.ExecuteResponse{}, err
	}
	result.Stdout = RedactLikelySecrets(result.Stdout)
	result.Stderr = RedactLikelySecrets(result.Stderr)

	if s.setupRecorder != nil {
		if err := s.setupRecorder.recordStepRun(ctx, setupSession, *selected, result, startedAt, finishedAt); err != nil {
			return domain.ExecuteResponse{}, err
		}
		if err := s.setupRecorder.cleanup(ctx, time.Now().UTC()); err != nil {
			return domain.ExecuteResponse{}, fmt.Errorf("cleanup setup session retention: %w", err)
		}
	}

	if selected.Type == "env-setup" {
		refreshed, refreshErr := s.Analyze(ctx, domain.AnalyzeRequest{
			LocalPath: analyzeResp.Source.Path,
		})
		if refreshErr == nil {
			analyzeResp = refreshed
			analyzeResp.Source = domain.RepoSource{
				Type:    inferRepoSourceType(req.RepoURL),
				RepoURL: req.RepoURL,
				Path:    analyzeResp.Source.Path,
			}
			if req.RepoURL == "" {
				analyzeResp.Source.Type = "local"
			}
		}
	}

	return domain.ExecuteResponse{
		Source:      analyzeResp.Source,
		Analysis:    analyzeResp.Analysis,
		Environment: analyzeResp.Environment,
		Plan:        analyzeResp.Plan,
		Result:      result,
	}, nil
}

func (s *AppService) prepareEnvSetup(ctx context.Context, analysis domain.RepositoryAnalysis, startedAt time.Time) (domain.ExecutionResult, error) {
	if s.vault == nil {
		return s.envDrafts.Prepare(analysis)
	}
	draft, err := s.envDrafts.BuildDraft(analysis)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	if err := s.vault.ApplyApprovedBindings(ctx, &draft); err != nil {
		return domain.ExecutionResult{}, err
	}
	pendingVaultUses, err := s.vault.ResolveBindings(ctx, &draft)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	saveResult, err := s.envDrafts.SaveAll(draft)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	result := envDraftExecutionResult("create-env-file", "instantrepo internal:prepare-env", analysis.RepoPath, saveResult, startedAt)
	if err := s.vault.RecordResolvedUses(ctx, pendingVaultUses); err != nil {
		appendEnvMetadataWarning(&result, "Env saved, but vault use metadata could not be updated.")
	}
	return result, nil
}

func (s *AppService) PreviewEnv(ctx context.Context, localPath string) (string, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return "", fmt.Errorf("localPath is required")
	}

	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if err != nil {
		return "", err
	}

	return s.envDrafts.Preview(analyzeResp.Analysis)
}

func (s *AppService) GenerateEnvDraft(ctx context.Context, localPath string) (domain.EnvDraft, error) {
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return domain.EnvDraft{}, fmt.Errorf("localPath is required")
	}

	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if err != nil {
		return domain.EnvDraft{}, err
	}
	draft, err := s.envDrafts.BuildDraft(analyzeResp.Analysis)
	if err != nil {
		return domain.EnvDraft{}, err
	}
	if s.vault != nil {
		if err := s.vault.ApplyApprovedBindings(ctx, &draft); err != nil {
			return domain.EnvDraft{}, err
		}
	}
	return draft, nil
}

func (s *AppService) SaveStructuredEnvDraft(ctx context.Context, localPath string, edited domain.EnvDraft) (domain.ExecuteResponse, error) {
	localPath = strings.TrimSpace(localPath)
	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if err != nil {
		return domain.ExecuteResponse{}, err
	}

	startedAt := time.Now().UTC()
	draft, err := s.envDrafts.BuildDraft(analyzeResp.Analysis)
	var pendingVaultUses []domain.EnvVaultUseRecord
	if err == nil {
		applyEditedEnvDraftValues(draft.Targets, edited.Targets)
		preserveExistingServiceCredentialValues(&draft)
		if s.vault != nil {
			pendingVaultUses, err = s.vault.ResolveBindings(ctx, &draft)
		}
	}
	var result domain.ExecutionResult
	var candidates []domain.EnvVaultPromptCandidate
	if err == nil {
		saveResult, saveErr := s.envDrafts.SaveAll(draft)
		err = saveErr
		if saveErr == nil {
			result = envDraftExecutionResult("create-env-file", "instantrepo internal:prepare-env", analyzeResp.Analysis.RepoPath, saveResult, startedAt)
			if s.vault != nil {
				if recordErr := s.vault.RecordResolvedUses(ctx, pendingVaultUses); recordErr != nil {
					appendEnvMetadataWarning(&result, "Env saved, but vault use metadata could not be updated.")
				} else {
					fromVault := map[string]bool{}
					for _, record := range pendingVaultUses {
						fromVault[promptCandidateKey(record.RepoPath, record.TargetRelativePath, record.VariableName)] = true
					}
					promptCandidates, promptErr := s.vault.PromptCandidates(ctx, &draft, fromVault)
					if promptErr != nil {
						appendEnvMetadataWarning(&result, "Env saved, but vault prompt metadata could not be updated.")
					} else {
						candidates = promptCandidates
					}
				}
			}
		}
	}
	finishedAt := time.Now().UTC()
	action := guardedSetupAction{
		repoPath: analyzeResp.Source.Path,
		stepID:   "create-env-file",
		title:    "Prepare local .env file",
		command:  "instantrepo internal:prepare-env",
		cwd:      analyzeResp.Analysis.RepoPath,
	}
	resp, finishErr := s.finishEnvDraftAction(ctx, localPath, analyzeResp, action, result, err, startedAt, finishedAt)
	if finishErr != nil {
		return resp, finishErr
	}
	resp.VaultPromptCandidates = candidates
	return resp, nil
}

func (s *AppService) SaveEnvValues(ctx context.Context, localPath string, values map[string]string) (domain.ExecuteResponse, error) {
	localPath = strings.TrimSpace(localPath)
	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if err != nil {
		return domain.ExecuteResponse{}, err
	}

	startedAt := time.Now().UTC()
	result, err := s.envDrafts.ApplyValues(analyzeResp.Analysis, values)
	finishedAt := time.Now().UTC()
	action := guardedSetupAction{
		repoPath: analyzeResp.Source.Path,
		stepID:   "create-env-file",
		title:    "Prepare local .env file",
		command:  "instantrepo internal:prepare-env",
		cwd:      analyzeResp.Analysis.RepoPath,
	}
	return s.finishEnvDraftAction(ctx, localPath, analyzeResp, action, result, err, startedAt, finishedAt)
}

func (s *AppService) SaveRawEnv(ctx context.Context, localPath, content string) (domain.ExecuteResponse, error) {
	localPath = strings.TrimSpace(localPath)
	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if err != nil {
		return domain.ExecuteResponse{}, err
	}

	startedAt := time.Now().UTC()
	draft, draftErr := s.envDrafts.BuildDraft(analyzeResp.Analysis)
	var targetPath string
	if draftErr == nil {
		if len(draft.Targets) != 1 {
			draftErr = fmt.Errorf("raw env save supports one env target; use structured env draft for multiple targets")
		} else {
			targetPath = draft.Targets[0].AbsolutePath
		}
	}
	var result domain.ExecutionResult
	if draftErr != nil {
		err = draftErr
	} else {
		result, err = s.envDrafts.SaveRaw(analyzeResp.Analysis.RepoPath, targetPath, content)
	}
	finishedAt := time.Now().UTC()
	action := guardedSetupAction{
		repoPath: analyzeResp.Source.Path,
		stepID:   "save-env-file",
		title:    "Save local .env file",
		command:  "instantrepo internal:save-env",
		cwd:      analyzeResp.Analysis.RepoPath,
	}
	return s.finishEnvDraftAction(ctx, localPath, analyzeResp, action, result, err, startedAt, finishedAt)
}

func (s *AppService) finishEnvDraftAction(ctx context.Context, localPath string, analyzeResp domain.AnalyzeResponse, action guardedSetupAction, result domain.ExecutionResult, runErr error, startedAt, finishedAt time.Time) (domain.ExecuteResponse, error) {
	if runErr != nil {
		failed := failedGuardedSetupResult(action.stepID, action.command, action.cwd, runErr, startedAt, finishedAt)
		if s.setupRecorder != nil {
			if recordErr := s.setupRecorder.recordGuardedSetupAction(ctx, action, failed, startedAt, finishedAt); recordErr != nil {
				return domain.ExecuteResponse{}, fmt.Errorf("%w; record failed setup action: %v", runErr, recordErr)
			}
		}
		return domain.ExecuteResponse{}, runErr
	}

	result.Stdout = RedactLikelySecrets(result.Stdout)
	result.Stderr = RedactLikelySecrets(result.Stderr)

	if s.setupRecorder != nil {
		if err := s.setupRecorder.recordGuardedSetupAction(ctx, action, result, startedAt, finishedAt); err != nil {
			return domain.ExecuteResponse{}, err
		}
	}

	refreshed, refreshErr := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if refreshErr == nil {
		analyzeResp = refreshed
	}
	return domain.ExecuteResponse{
		Source:      analyzeResp.Source,
		Analysis:    analyzeResp.Analysis,
		Environment: analyzeResp.Environment,
		Plan:        analyzeResp.Plan,
		Result:      result,
	}, nil
}

func appendEnvMetadataWarning(result *domain.ExecutionResult, message string) {
	if result == nil || strings.TrimSpace(message) == "" {
		return
	}
	if strings.TrimSpace(result.Stderr) != "" && !strings.HasSuffix(result.Stderr, "\n") {
		result.Stderr += "\n"
	}
	result.Stderr += message + "\n"
}

func inferRepoSourceType(repoURL string) string {
	lower := strings.ToLower(repoURL)
	switch {
	case strings.Contains(lower, "gitlab"):
		return "gitlab"
	case strings.Contains(lower, "github"):
		return "github"
	default:
		return "git"
	}
}

func normalizeRepoURL(repoURL string) string {
	normalized := strings.ToLower(strings.TrimSpace(repoURL))
	normalized = strings.TrimRight(normalized, "/")
	normalized = strings.TrimSuffix(normalized, ".git")
	normalized = strings.TrimRight(normalized, "/")
	return normalized
}
