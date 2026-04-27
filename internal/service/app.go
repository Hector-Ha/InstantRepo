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
)

type AppService struct {
	fetcher  *RepoFetcher
	analyzer *analyzer.RepositoryAnalyzer
	detector *detector.EnvironmentDetector
	planner  *Planner
	executor *Executor
	envFiles *EnvFileManager
}

func NewAppService() *AppService {
	return &AppService{
		fetcher:  NewRepoFetcher(),
		analyzer: analyzer.NewRepositoryAnalyzer(),
		detector: detector.NewEnvironmentDetector(),
		planner:  NewPlanner(),
		executor: NewExecutor(),
		envFiles: NewEnvFileManager(),
	}
}

func (s *AppService) Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.AnalyzeResponse, error) {
	if req.RepoURL == "" && req.LocalPath == "" {
		return domain.AnalyzeResponse{}, fmt.Errorf("repoUrl or localPath is required")
	}

	repoPath := req.LocalPath
	sourceType := "local"
	cleanup := func() {}

	if req.RepoURL != "" {
		clonedPath, release, err := s.fetcher.Clone(ctx, req.RepoURL)
		if err != nil {
			return domain.AnalyzeResponse{}, err
		}
		repoPath = clonedPath
		sourceType = "github"
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

	analysis, err := s.analyzer.Analyze(absPath)
	if err != nil {
		return domain.AnalyzeResponse{}, err
	}

	environment := s.detector.Detect()
	plan := s.planner.BuildPlan(analysis, environment)

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

func (s *AppService) ImportRepository(ctx context.Context, repoURL, destinationRoot string) (domain.AnalyzeResponse, error) {
	if strings.TrimSpace(repoURL) == "" {
		return domain.AnalyzeResponse{}, fmt.Errorf("repo URL is required")
	}

	clonedPath, err := s.fetcher.CloneTo(ctx, repoURL, destinationRoot)
	if err != nil {
		return domain.AnalyzeResponse{}, err
	}

	resp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: clonedPath})
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

	var result domain.ExecutionResult
	switch selected.Type {
	case "env-setup":
		result, err = s.envFiles.Prepare(analyzeResp.Analysis)
	default:
		result, err = s.executor.RunStep(runCtx, *selected)
	}
	if err != nil {
		return domain.ExecuteResponse{}, err
	}

	if selected.Type == "env-setup" {
		refreshed, refreshErr := s.Analyze(ctx, domain.AnalyzeRequest{
			LocalPath: analyzeResp.Source.Path,
		})
		if refreshErr == nil {
			analyzeResp = refreshed
			analyzeResp.Source = domain.RepoSource{
				Type:    "github",
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

func (s *AppService) SaveEnvValues(ctx context.Context, localPath string, values map[string]string) (domain.ExecuteResponse, error) {
	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if err != nil {
		return domain.ExecuteResponse{}, err
	}

	result, err := s.envFiles.ApplyValues(analyzeResp.Analysis, values)
	if err != nil {
		return domain.ExecuteResponse{}, err
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

func (s *AppService) SaveRawEnv(ctx context.Context, localPath, content string) (domain.ExecuteResponse, error) {
	analyzeResp, err := s.Analyze(ctx, domain.AnalyzeRequest{LocalPath: localPath})
	if err != nil {
		return domain.ExecuteResponse{}, err
	}

	result, err := s.envFiles.WriteRaw(analyzeResp.Analysis.RepoPath, analyzeResp.Analysis.Env.TargetPath, content)
	if err != nil {
		return domain.ExecuteResponse{}, err
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
