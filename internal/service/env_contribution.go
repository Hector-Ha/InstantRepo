package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"instantrepo/internal/domain"
	"instantrepo/internal/envcatalog"
)

const (
	envContributionSchemaVersion = "2026-05-23"
	envContributionAppVersion    = "0.1.0"
)

var ErrEnvContributionSendUnavailable = errors.New("env contribution sender unavailable")

type EnvContributionStore interface {
	EnvContributionSettings(ctx context.Context) (domain.EnvContributionSettings, error)
	SaveEnvContributionSettings(ctx context.Context, settings domain.EnvContributionSettings) error
	SaveEnvContributionQueueItem(ctx context.Context, item domain.EnvContributionQueueItem) (domain.EnvContributionQueueItem, error)
	EnvContributionQueueStatus(ctx context.Context) (domain.EnvContributionQueueStatus, error)
	ClearEnvContributionQueue(ctx context.Context) error
}

type EnvContributionSender interface {
	SendEnvContribution(ctx context.Context, payload domain.EnvContributionPayload) error
}

type EnvContributionPublicChecker interface {
	IsPublicRepo(ctx context.Context, normalizedURL string) bool
}

type EnvContributionService struct {
	store         EnvContributionStore
	sender        EnvContributionSender
	publicChecker EnvContributionPublicChecker
	catalog       envcatalog.Catalog
}

func NewEnvContributionService(store EnvContributionStore, sender EnvContributionSender) *EnvContributionService {
	if sender == nil {
		sender = disabledEnvContributionSender{}
	}
	return &EnvContributionService{
		store:         store,
		sender:        sender,
		publicChecker: gitPublicRepoChecker{},
		catalog:       envcatalog.DefaultCatalog(),
	}
}

func (s *AppService) EnvContributionSettings(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	if s.contribution == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	return s.contribution.Settings(ctx)
}

func (s *AppService) SaveEnvContributionSettings(ctx context.Context, settings domain.EnvContributionSettings) (domain.EnvContributionSettingsResponse, error) {
	if s.contribution == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	return s.contribution.SaveSettings(ctx, settings)
}

func (s *AppService) RecordEnvContributionConsent(ctx context.Context, publicEnabled bool) (domain.EnvContributionSettingsResponse, error) {
	if s.contribution == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	return s.contribution.RecordConsent(ctx, publicEnabled)
}

func (s *AppService) ClearEnvContributionQueue(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	if s.contribution == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	return s.contribution.ClearQueue(ctx)
}

func (s *EnvContributionService) Settings(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	if s == nil || s.store == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	settings, err := s.store.EnvContributionSettings(ctx)
	if err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	queue, err := s.store.EnvContributionQueueStatus(ctx)
	if err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return domain.EnvContributionSettingsResponse{Settings: settings, Queue: queue}, nil
}

func (s *EnvContributionService) SaveSettings(ctx context.Context, settings domain.EnvContributionSettings) (domain.EnvContributionSettingsResponse, error) {
	if s == nil || s.store == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	if err := s.store.SaveEnvContributionSettings(ctx, settings); err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return s.Settings(ctx)
}

func (s *EnvContributionService) RecordConsent(ctx context.Context, publicEnabled bool) (domain.EnvContributionSettingsResponse, error) {
	if s == nil || s.store == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	settings, err := s.store.EnvContributionSettings(ctx)
	if err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	settings.PublicEnvPatternsEnabled = publicEnabled
	settings.ConsentShown = true
	if err := s.store.SaveEnvContributionSettings(ctx, settings); err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return s.Settings(ctx)
}

func (s *EnvContributionService) ClearQueue(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	if s == nil || s.store == nil {
		return domain.EnvContributionSettingsResponse{}, nil
	}
	if err := s.store.ClearEnvContributionQueue(ctx); err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return s.Settings(ctx)
}

func (s *EnvContributionService) BuildAnalysisPayload(ctx context.Context, resp domain.AnalyzeResponse, settings domain.EnvContributionSettings) (domain.EnvContributionPayload, bool, error) {
	if !settings.ConsentShown {
		return domain.EnvContributionPayload{}, false, nil
	}
	repo := classifyEnvContributionRepo(ctx, resp.Source.RepoURL, s.publicChecker)
	if repo.Public {
		if !settings.PublicEnvPatternsEnabled {
			return domain.EnvContributionPayload{}, false, nil
		}
		repo.CommitSHA = gitCommitSHA(ctx, resp.Source.Path)
		repo.EnvRelevantDirty = gitEnvRelevantDirty(ctx, resp.Source.Path)
	} else {
		if !settings.PrivateLocalEnvPatternsEnabled {
			return domain.EnvContributionPayload{}, false, nil
		}
		repo.IdentityOmitted = true
		repo.PrivateOrUncertain = true
	}
	payload := s.basePayload(domain.EnvContributionEventAnalysis, resp.Environment, repo)
	payload.EnvNames = envContributionNames(resp.Plan.Env.Variables)
	payload.Targets = envContributionTargets(resp.Source.Path, resp.Plan.Env)
	payload.Stacks = envContributionStacks(resp.Analysis)
	return payload, true, nil
}

func (s *EnvContributionService) BuildSaveOutcomePayload(ctx context.Context, resp domain.ExecuteResponse, draft domain.EnvDraft, settings domain.EnvContributionSettings) (domain.EnvContributionPayload, bool, error) {
	if !settings.ConsentShown {
		return domain.EnvContributionPayload{}, false, nil
	}
	repo := classifyEnvContributionRepo(ctx, resp.Source.RepoURL, s.publicChecker)
	if repo.Public {
		if !settings.PublicEnvPatternsEnabled {
			return domain.EnvContributionPayload{}, false, nil
		}
		repo.CommitSHA = gitCommitSHA(ctx, resp.Source.Path)
		repo.EnvRelevantDirty = gitEnvRelevantDirty(ctx, resp.Source.Path)
	} else {
		if !settings.PrivateLocalEnvPatternsEnabled {
			return domain.EnvContributionPayload{}, false, nil
		}
		repo.IdentityOmitted = true
		repo.PrivateOrUncertain = true
	}
	payload := s.basePayload(domain.EnvContributionEventSaveOutcome, resp.Environment, repo)
	payload.EnvNames = envContributionNamesFromDraft(draft)
	payload.Targets = envContributionTargetsFromDraft(draft)
	payload.Stacks = envContributionStacks(resp.Analysis)
	payload.Outcomes = envContributionOutcomes(draft, resp.Result)
	return payload, true, nil
}

func (s *EnvContributionService) RecordAnalysis(ctx context.Context, resp domain.AnalyzeResponse) error {
	if s == nil || s.store == nil {
		return nil
	}
	settings, err := s.store.EnvContributionSettings(ctx)
	if err != nil {
		return nil
	}
	payload, ok, err := s.BuildAnalysisPayload(ctx, resp, settings)
	if err != nil || !ok {
		return nil
	}
	return s.sendOrQueue(ctx, payload)
}

func (s *EnvContributionService) RecordSaveOutcome(ctx context.Context, resp domain.ExecuteResponse, draft domain.EnvDraft) error {
	if s == nil || s.store == nil {
		return nil
	}
	settings, err := s.store.EnvContributionSettings(ctx)
	if err != nil {
		return nil
	}
	payload, ok, err := s.BuildSaveOutcomePayload(ctx, resp, draft, settings)
	if err != nil || !ok {
		return nil
	}
	return s.sendOrQueue(ctx, payload)
}

func (s *EnvContributionService) sendOrQueue(ctx context.Context, payload domain.EnvContributionPayload) error {
	if s == nil || s.sender == nil {
		return nil
	}
	if err := s.sender.SendEnvContribution(ctx, payload); err == nil {
		return nil
	}
	if s.store == nil {
		return nil
	}
	raw, err := marshalEnvContributionPayload(payload)
	if err != nil {
		return nil
	}
	_, _ = s.store.SaveEnvContributionQueueItem(ctx, domain.EnvContributionQueueItem{
		EventType:   payload.EventType,
		PayloadJSON: raw,
	})
	return nil
}

func (s *EnvContributionService) basePayload(eventType string, environment domain.EnvironmentReport, repo domain.EnvContributionRepo) domain.EnvContributionPayload {
	return domain.EnvContributionPayload{
		SchemaVersion:  envContributionSchemaVersion,
		EventType:      eventType,
		AppVersion:     envContributionAppVersion,
		CatalogVersion: s.catalog.Version,
		OS: domain.EnvContributionOS{
			Name: strings.TrimSpace(environment.OS),
			Arch: strings.TrimSpace(environment.Arch),
		},
		Repo: repo,
	}
}

func classifyEnvContributionRepo(ctx context.Context, rawURL string, checker EnvContributionPublicChecker) domain.EnvContributionRepo {
	normalizedURL, ok := normalizeContributionRepoURL(rawURL)
	if !ok || checker == nil || !checker.IsPublicRepo(ctx, normalizedURL) {
		return domain.EnvContributionRepo{Public: false, IdentityOmitted: true, PrivateOrUncertain: true}
	}
	return domain.EnvContributionRepo{Public: true, URL: normalizedURL}
}

func normalizeContributionRepoURL(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.Contains(rawURL, "@") && strings.HasPrefix(strings.ToLower(rawURL), "git@") {
		return "", false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", false
	}
	if parsed.User != nil {
		return "", false
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), ".git")
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return "", false
	}
	parsed.Path = "/" + strings.ToLower(parts[0]) + "/" + strings.ToLower(parts[1])
	return parsed.String(), true
}

func envContributionNames(vars []domain.EnvVarRequirement) []string {
	names := map[string]bool{}
	for _, item := range vars {
		name := strings.ToUpper(strings.TrimSpace(item.Name))
		if name != "" {
			names[name] = true
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func envContributionNamesFromDraft(draft domain.EnvDraft) []string {
	names := map[string]bool{}
	for _, target := range draft.Targets {
		for _, value := range target.Values {
			name := strings.ToUpper(strings.TrimSpace(value.Name))
			if name != "" {
				names[name] = true
			}
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func envContributionTargets(repoPath string, env domain.EnvironmentConfig) []domain.EnvContributionTarget {
	relative := relativeContributionPath(repoPath, env.TargetPath)
	if relative == "" {
		return nil
	}
	return []domain.EnvContributionTarget{{RelativePath: relative}}
}

func envContributionTargetsFromDraft(draft domain.EnvDraft) []domain.EnvContributionTarget {
	seen := map[string]bool{}
	for _, target := range draft.Targets {
		relative := filepath.ToSlash(strings.TrimSpace(target.RelativePath))
		if relative != "" && !filepath.IsAbs(relative) {
			seen[relative] = true
		}
	}
	out := make([]domain.EnvContributionTarget, 0, len(seen))
	for relative := range seen {
		out = append(out, domain.EnvContributionTarget{RelativePath: relative})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelativePath < out[j].RelativePath })
	return out
}

func relativeContributionPath(repoPath, targetPath string) string {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return ""
	}
	if filepath.IsAbs(targetPath) && strings.TrimSpace(repoPath) != "" {
		relative, err := filepath.Rel(repoPath, targetPath)
		if err == nil && !strings.HasPrefix(relative, "..") && !filepath.IsAbs(relative) {
			return filepath.ToSlash(relative)
		}
		return ""
	}
	return filepath.ToSlash(targetPath)
}

func envContributionStacks(analysis domain.RepositoryAnalysis) []domain.EnvContributionStack {
	seen := map[string]domain.EnvContributionStack{}
	if name := strings.TrimSpace(analysis.ProjectType); name != "" {
		seen[strings.ToLower(name)] = domain.EnvContributionStack{Name: name}
	}
	for _, req := range analysis.Requirements {
		name := strings.ToLower(strings.TrimSpace(req.Tool))
		if !recognizedContributionStack(name) {
			continue
		}
		if _, ok := seen[name]; !ok {
			seen[name] = domain.EnvContributionStack{
				Name:    name,
				Version: strings.TrimSpace(req.VersionConstraint),
			}
		}
	}
	out := make([]domain.EnvContributionStack, 0, len(seen))
	for _, stack := range seen {
		out = append(out, stack)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func recognizedContributionStack(name string) bool {
	switch name {
	case "bun", "deno", "docker", "go", "node", "npm", "pnpm", "python", "ruby":
		return true
	default:
		return false
	}
}

func envContributionOutcomes(draft domain.EnvDraft, result domain.ExecutionResult) []domain.EnvContributionOutcome {
	var outcomes []domain.EnvContributionOutcome
	for _, target := range draft.Targets {
		relative := filepath.ToSlash(strings.TrimSpace(target.RelativePath))
		if relative == "" || filepath.IsAbs(relative) {
			continue
		}
		for _, value := range target.Values {
			name := strings.ToUpper(strings.TrimSpace(value.Name))
			if name == "" {
				continue
			}
			state := "saved"
			if strings.TrimSpace(value.Value) == "" && value.VaultBinding == nil {
				state = "cleared"
			}
			if !result.Succeeded {
				state = "failed"
			}
			outcomes = append(outcomes, domain.EnvContributionOutcome{
				TargetRelativePath: relative,
				VariableName:       name,
				ValueClass:         value.ValueClass,
				FillOutcome:        safeContributionProvenance(value.Provenance.Source),
				ValueState:         state,
				Saved:              result.Succeeded,
			})
		}
	}
	return outcomes
}

func safeContributionProvenance(source string) string {
	if source == domain.EnvValueSourceVault {
		return "credential_source"
	}
	return strings.TrimSpace(source)
}

func gitCommitSHA(ctx context.Context, repoPath string) string {
	if strings.TrimSpace(repoPath) == "" {
		return ""
	}
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(runCtx, "git", "-C", repoPath, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitEnvRelevantDirty(ctx context.Context, repoPath string) bool {
	if strings.TrimSpace(repoPath) == "" {
		return false
	}
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(runCtx, "git", "-C", repoPath, "status", "--porcelain", "--", ".env", ".env.*").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

type disabledEnvContributionSender struct{}

func (disabledEnvContributionSender) SendEnvContribution(context.Context, domain.EnvContributionPayload) error {
	return ErrEnvContributionSendUnavailable
}

type gitPublicRepoChecker struct{}

func (gitPublicRepoChecker) IsPublicRepo(ctx context.Context, normalizedURL string) bool {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := exec.CommandContext(runCtx, "git", "ls-remote", "--exit-code", normalizedURL, "HEAD").Run()
	return err == nil
}

func marshalEnvContributionPayload(payload domain.EnvContributionPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
