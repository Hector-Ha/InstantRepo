package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"instantrepo/internal/domain"
)

const (
	aiEnvReviewSchemaVersion = "2026-05-24"
	aiEnvReviewMaxFiles      = 80
	aiEnvReviewMaxExcerpt    = 600
	aiEnvReviewMaxSnippet    = 240
	aiEnvReviewMaxSnippets   = 20
	aiEnvReviewMaxConfidence = 0.90
)

var (
	aiEnvReviewSecretTokenPattern = regexp.MustCompile(`(?i)(sk-[a-z0-9_-]{6,}|ghp_[a-z0-9_]{6,}|github_pat_[a-z0-9_]{6,}|xox[baprs]-[a-z0-9-]{6,}|sg\.[a-z0-9._-]{6,})`)
	aiEnvReviewAbsPathPattern     = regexp.MustCompile(`([A-Za-z]:\\[^\s"']+|/[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+){1,})`)
)

type AIEnvReviewService struct {
	publicChecker EnvContributionPublicChecker
	reviewer      AIEnvReviewer
}

func NewAIEnvReviewService(reviewer AIEnvReviewer) *AIEnvReviewService {
	return &AIEnvReviewService{publicChecker: gitPublicRepoChecker{}, reviewer: reviewer}
}

func (s *AIEnvReviewService) BuildBundle(ctx context.Context, resp domain.AnalyzeResponse, draft domain.EnvDraft) (domain.AIEnvReviewBundle, bool, error) {
	repoPath := strings.TrimSpace(resp.Source.Path)
	if repoPath == "" {
		repoPath = draft.RepoPath
	}
	candidates := aiEnvReviewCandidates(draft)
	if len(candidates) == 0 {
		return domain.AIEnvReviewBundle{}, false, nil
	}
	repo := aiEnvReviewRepo(ctx, resp.Source.RepoURL, repoPath, s.publicChecker)
	bundle := domain.AIEnvReviewBundle{
		SchemaVersion: aiEnvReviewSchemaVersion,
		Repo:          repo,
		FileTree:      aiEnvReviewFileTree(repoPath),
		Manifests:     aiEnvReviewManifests(repoPath),
		SetupExcerpts: aiEnvReviewSetupExcerpts(repoPath),
		EnvNames:      aiEnvReviewEnvNames(resp.Analysis.Env.Variables, draft),
		Targets:       aiEnvReviewTargets(draft),
		UsageSnippets: aiEnvReviewUsageSnippets(repoPath, resp.Analysis.Env.Variables),
		Topology:      sanitizeAIEnvReviewTopology(repoPath, resp.Analysis.Topology),
		Candidates:    candidates,
	}
	return bundle, true, nil
}

func (s *AIEnvReviewService) ReviewDraft(ctx context.Context, resp domain.AnalyzeResponse, draft *domain.EnvDraft) error {
	if s == nil || s.reviewer == nil || draft == nil {
		return nil
	}
	bundle, ok, err := s.BuildBundle(ctx, resp, *draft)
	if err != nil || !ok {
		return err
	}
	patch, err := s.reviewer.ReviewEnv(ctx, bundle)
	if err != nil {
		return nil
	}
	if err := s.ApplyPatch(draft, patch); err != nil {
		return nil
	}
	return nil
}

func (s *AIEnvReviewService) ValidatePatch(draft domain.EnvDraft, patch domain.EnvPatch) error {
	for index, op := range patch.Operations {
		if strings.TrimSpace(op.Path) != "" || strings.TrimSpace(op.Command) != "" {
			return fmt.Errorf("env patch operation %d is not draft-only", index)
		}
		if op.Op != "set_env" {
			return fmt.Errorf("env patch operation %d is not allowed", index)
		}
		targetIndex, valueIndex, ok := findEnvPatchValue(draft, op.TargetRelativePath, op.VariableName)
		if !ok {
			return fmt.Errorf("env patch operation %d targets unknown env value", index)
		}
		value := draft.Targets[targetIndex].Values[valueIndex]
		if !canAIEnvPatchValue(value, op.Value) {
			return fmt.Errorf("env patch operation %d cannot change protected env value", index)
		}
	}
	return nil
}

func (s *AIEnvReviewService) ApplyPatch(draft *domain.EnvDraft, patch domain.EnvPatch) error {
	if draft == nil {
		return fmt.Errorf("env draft is required")
	}
	if err := s.ValidatePatch(*draft, patch); err != nil {
		return err
	}
	for _, op := range patch.Operations {
		targetIndex, valueIndex, _ := findEnvPatchValue(*draft, op.TargetRelativePath, op.VariableName)
		value := &draft.Targets[targetIndex].Values[valueIndex]
		value.Value = strings.TrimSpace(op.Value)
		if op.Confidence > 0 {
			value.Confidence = op.Confidence
		}
		value.Provenance.Source = domain.EnvValueSourceAIPatch
		if !stringSliceContains(value.Attention, "AI reviewed") {
			value.Attention = append(value.Attention, "AI reviewed")
		}
	}
	return nil
}

func findEnvPatchValue(draft domain.EnvDraft, targetRelativePath, variableName string) (int, int, bool) {
	targetRelativePath = filepath.ToSlash(strings.TrimSpace(targetRelativePath))
	variableName = strings.ToUpper(strings.TrimSpace(variableName))
	if !safeRelativeEnvPath(targetRelativePath) || variableName == "" {
		return 0, 0, false
	}
	for targetIndex, target := range draft.Targets {
		if filepath.ToSlash(target.RelativePath) != targetRelativePath {
			continue
		}
		for valueIndex, value := range target.Values {
			if strings.ToUpper(strings.TrimSpace(value.Name)) == variableName {
				return targetIndex, valueIndex, true
			}
		}
	}
	return 0, 0, false
}

func stringSliceContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func canAIEnvPatchValue(value domain.EnvDraftValue, next string) bool {
	if value.Secret || value.VaultBinding != nil || value.HasExistingValue {
		return false
	}
	if value.ValueClass != domain.EnvValueClassDevDefault && value.ValueClass != domain.EnvValueClassProviderConfig {
		return false
	}
	if looksLikeProviderCredentialValue(next) {
		return false
	}
	if value.Provenance.Source == domain.EnvValueSourceDraft && strings.TrimSpace(value.Value) != "" {
		return false
	}
	if strings.TrimSpace(value.Value) == "" || isKnownWeakEnvPlaceholder(value.Value) {
		return true
	}
	switch value.Provenance.Source {
	case domain.EnvValueSourceCatalog, domain.EnvValueSourceAllocator:
		return value.Confidence > 0 && value.Confidence < aiEnvReviewMaxConfidence
	default:
		return false
	}
}

func looksLikeProviderCredentialValue(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	secretMarkers := []string{"sk-", "ghp_", "github_pat_", "xoxb-", "sg.", "api_key", "apikey", "secret", "token"}
	for _, marker := range secretMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func aiEnvReviewRepo(ctx context.Context, rawURL, repoPath string, checker EnvContributionPublicChecker) domain.AIEnvReviewRepo {
	classified := classifyEnvContributionRepo(ctx, rawURL, checker)
	if !classified.Public {
		return domain.AIEnvReviewRepo{Public: false, IdentityOmitted: true, PrivateOrUncertain: true}
	}
	return domain.AIEnvReviewRepo{
		Public:    true,
		URL:       classified.URL,
		CommitSHA: gitCommitSHA(ctx, repoPath),
	}
}

func aiEnvReviewCandidates(draft domain.EnvDraft) []domain.AIEnvReviewDraftCandidate {
	var out []domain.AIEnvReviewDraftCandidate
	for _, target := range draft.Targets {
		if !safeRelativeEnvPath(target.RelativePath) {
			continue
		}
		for _, value := range target.Values {
			if !aiEnvReviewCandidateValue(value) {
				continue
			}
			out = append(out, domain.AIEnvReviewDraftCandidate{
				TargetRelativePath: filepath.ToSlash(target.RelativePath),
				VariableName:       strings.ToUpper(strings.TrimSpace(value.Name)),
				ValueClass:         value.ValueClass,
				CurrentValueState:  aiEnvValueState(value.Value),
				Confidence:         value.Confidence,
				Provenance:         value.Provenance.Source,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TargetRelativePath == out[j].TargetRelativePath {
			return out[i].VariableName < out[j].VariableName
		}
		return out[i].TargetRelativePath < out[j].TargetRelativePath
	})
	return out
}

func aiEnvReviewCandidateValue(value domain.EnvDraftValue) bool {
	if value.Secret || value.VaultBinding != nil || value.HasExistingValue {
		return false
	}
	if value.ValueClass != domain.EnvValueClassDevDefault && value.ValueClass != domain.EnvValueClassProviderConfig {
		return false
	}
	return value.Confidence > 0 && value.Confidence < aiEnvReviewMaxConfidence
}

func aiEnvReviewFileTree(repoPath string) []string {
	var out []string
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == repoPath {
			return nil
		}
		name := d.Name()
		if d.IsDir() && ignoredAIEnvReviewDir(name) {
			return filepath.SkipDir
		}
		rel := safeRelativePath(repoPath, path)
		if rel == "" || strings.HasPrefix(filepath.Base(rel), ".env") {
			return nil
		}
		out = append(out, rel)
		if len(out) >= aiEnvReviewMaxFiles {
			return filepath.SkipAll
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func aiEnvReviewManifests(repoPath string) []domain.AIEnvReviewManifest {
	var out []domain.AIEnvReviewManifest
	for _, rel := range []string{"package.json", "go.mod", "requirements.txt", "pyproject.toml", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		path := filepath.Join(repoPath, rel)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		item := domain.AIEnvReviewManifest{RelativePath: filepath.ToSlash(rel)}
		if rel == "package.json" {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			if raw, err := os.ReadFile(path); err == nil && json.Unmarshal(raw, &pkg) == nil {
				item.Scripts = boundedStringMap(pkg.Scripts, 12, 120)
			}
		}
		out = append(out, item)
	}
	return out
}

func aiEnvReviewSetupExcerpts(repoPath string) []domain.AIEnvReviewExcerpt {
	var out []domain.AIEnvReviewExcerpt
	for _, rel := range []string{"README.md", "README", "docs/setup.md", "docs/SETUP.md"} {
		path := filepath.Join(repoPath, rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out = append(out, domain.AIEnvReviewExcerpt{
			RelativePath: filepath.ToSlash(rel),
			Text:         redactAIEnvReviewText(string(raw), aiEnvReviewMaxExcerpt),
		})
	}
	return out
}

func aiEnvReviewEnvNames(vars []domain.EnvVarRequirement, draft domain.EnvDraft) []string {
	seen := map[string]bool{}
	for _, item := range vars {
		name := strings.ToUpper(strings.TrimSpace(item.Name))
		if name != "" {
			seen[name] = true
		}
	}
	for _, target := range draft.Targets {
		for _, value := range target.Values {
			name := strings.ToUpper(strings.TrimSpace(value.Name))
			if name != "" {
				seen[name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func aiEnvReviewTargets(draft domain.EnvDraft) []domain.AIEnvReviewTarget {
	var out []domain.AIEnvReviewTarget
	for _, target := range draft.Targets {
		if !safeRelativeEnvPath(target.RelativePath) {
			continue
		}
		names := make([]string, 0, len(target.Values))
		for _, value := range target.Values {
			name := strings.ToUpper(strings.TrimSpace(value.Name))
			if name != "" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		out = append(out, domain.AIEnvReviewTarget{RelativePath: filepath.ToSlash(target.RelativePath), EnvNames: names})
	}
	return out
}

func aiEnvReviewUsageSnippets(repoPath string, vars []domain.EnvVarRequirement) []domain.AIEnvReviewUsageSnippet {
	names := map[string]bool{}
	for _, item := range vars {
		name := strings.ToUpper(strings.TrimSpace(item.Name))
		if name != "" {
			names[name] = true
		}
	}
	if len(names) == 0 {
		return nil
	}
	var out []domain.AIEnvReviewUsageSnippet
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && ignoredAIEnvReviewDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !aiEnvReviewSourceFile(path) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) > 128*1024 {
			return nil
		}
		rel := safeRelativePath(repoPath, path)
		if rel == "" {
			return nil
		}
		for _, line := range strings.Split(string(raw), "\n") {
			upper := strings.ToUpper(line)
			for name := range names {
				if !strings.Contains(upper, name) {
					continue
				}
				out = append(out, domain.AIEnvReviewUsageSnippet{
					RelativePath: rel,
					EnvName:      name,
					Snippet:      redactAIEnvReviewText(strings.TrimSpace(line), aiEnvReviewMaxSnippet),
				})
				if len(out) >= aiEnvReviewMaxSnippets {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return out
}

func sanitizeAIEnvReviewTopology(repoPath string, topology domain.AppTopology) domain.AppTopology {
	out := domain.AppTopology{Signals: make([]domain.AppTopologySignal, 0, len(topology.Signals))}
	for _, signal := range topology.Signals {
		signal.TargetDir = safeRelativePath(repoPath, signal.TargetDir)
		out.Signals = append(out.Signals, signal)
	}
	return out
}

func boundedStringMap(input map[string]string, limit int, valueLimit int) map[string]string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := map[string]string{}
	for _, key := range keys {
		if len(out) >= limit {
			break
		}
		out[key] = redactAIEnvReviewText(input[key], valueLimit)
	}
	return out
}

func safeRelativePath(root, path string) string {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(path) == "" {
		return ""
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return ""
	}
	return filepath.ToSlash(rel)
}

func safeRelativeEnvPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != "." && !strings.HasPrefix(clean, "..")
}

func ignoredAIEnvReviewDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", "node_modules", "vendor", "dist", "build", ".next", ".wails":
		return true
	default:
		return false
	}
}

func aiEnvReviewSourceFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".jsx", ".ts", ".tsx", ".go", ".py", ".rb", ".php", ".java", ".cs":
		return true
	default:
		return false
	}
}

func aiEnvValueState(value string) string {
	if strings.TrimSpace(value) == "" {
		return "blank"
	}
	if isKnownWeakEnvPlaceholder(value) {
		return "placeholder"
	}
	return "app_generated"
}

func redactAIEnvReviewText(text string, limit int) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if name, _, ok := strings.Cut(line, "="); ok && looksLikeEnvAssignmentName(strings.TrimSpace(name)) {
			lines[i] = strings.TrimSpace(name) + "="
			continue
		}
		line = aiEnvReviewSecretTokenPattern.ReplaceAllString(line, "[redacted]")
		line = aiEnvReviewAbsPathPattern.ReplaceAllString(line, "[path]")
		lines[i] = line
	}
	return truncateString(strings.TrimSpace(strings.Join(lines, "\n")), limit)
}

func looksLikeEnvAssignmentName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

type AIEnvReviewer interface {
	ReviewEnv(context.Context, domain.AIEnvReviewBundle) (domain.EnvPatch, error)
}
