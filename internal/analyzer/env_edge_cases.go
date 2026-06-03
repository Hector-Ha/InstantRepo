package analyzer

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/envcatalog"
	"instantrepo/internal/util"
)

func applyRepoEnvEdgeCases(analysis *domain.RepositoryAnalysis) {
	if analysis == nil {
		return
	}
	catalog := envcatalog.DefaultRepoEdgeCases()
	remotes := repoRemoteIDs(analysis.RepoPath)
	for _, rule := range catalog.Rules {
		if !repoEdgeCaseMatches(rule, analysis.ProjectName, remotes) {
			continue
		}
		version := repoManifestVersion(analysis.RepoPath, rule.Manifest.Path)
		if !versionConstraintMatches(version, rule.Manifest.VersionConstraint) {
			continue
		}
		for _, envDefault := range rule.Env {
			applyRepoEnvEdgeCaseDefault(analysis, envDefault)
		}
		if rule.ID != "" {
			analysis.Evidence = appendUniqueString(analysis.Evidence, "Repo env edge case matched: "+rule.ID)
		}
	}
}

func repoEdgeCaseMatches(rule envcatalog.RepoEdgeCaseRule, projectName string, remotes []string) bool {
	if len(rule.Repo.Remotes) == 0 && len(rule.Repo.ProjectNames) == 0 {
		return true
	}
	if len(rule.Repo.Remotes) > 0 && len(remotes) > 0 {
		for _, remote := range remotes {
			if stringInFoldedSet(remote, rule.Repo.Remotes) {
				return true
			}
		}
		return false
	}
	if len(rule.Repo.ProjectNames) > 0 {
		return stringInFoldedSet(projectName, rule.Repo.ProjectNames)
	}
	return false
}

func applyRepoEnvEdgeCaseDefault(analysis *domain.RepositoryAnalysis, envDefault envcatalog.RepoEdgeCaseDefault) {
	if strings.TrimSpace(envDefault.Name) == "" || strings.TrimSpace(envDefault.DefaultValue) == "" {
		return
	}
	expectedTargetDir := repoEdgeCaseTargetDir(analysis.RepoPath, envDefault.TargetDir)
	for i := range analysis.Env.Variables {
		item := &analysis.Env.Variables[i]
		if !strings.EqualFold(item.Name, envDefault.Name) {
			continue
		}
		if expectedTargetDir != "" && filepath.Clean(item.TargetDir) != expectedTargetDir {
			continue
		}
		item.SuggestedValue = envDefault.DefaultValue
		item.DefaultSource = domain.EnvValueSourceCatalog
		item.DefaultClass = envDefault.ValueClass
		if item.DefaultClass == "" {
			item.DefaultClass = domain.EnvValueClassDevDefault
		}
		if envDefault.Confidence > item.Confidence {
			item.Confidence = envDefault.Confidence
		}
		item.Instructions = appendUniqueString(item.Instructions, envDefault.Instructions...)
	}
}

func repoEdgeCaseTargetDir(repoPath, targetDir string) string {
	targetDir = strings.TrimSpace(targetDir)
	if targetDir == "" {
		return ""
	}
	if filepath.IsAbs(targetDir) {
		return filepath.Clean(targetDir)
	}
	return filepath.Clean(filepath.Join(repoPath, filepath.FromSlash(targetDir)))
}

func repoManifestVersion(repoPath, manifestPath string) string {
	manifestPath = strings.TrimSpace(manifestPath)
	if manifestPath == "" {
		return ""
	}
	path := manifestPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoPath, filepath.FromSlash(path))
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(util.ReadTextFile(path)), &pkg); err != nil {
		return ""
	}
	return strings.TrimSpace(pkg.Version)
}

func repoRemoteIDs(repoPath string) []string {
	raw := util.ReadTextFile(filepath.Join(repoPath, ".git", "config"))
	if raw == "" {
		return nil
	}
	ids := []string{}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "url =") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if id := normalizeRepoRemoteID(parts[1]); id != "" {
			ids = appendUniqueString(ids, id)
		}
	}
	return ids
}

func normalizeRepoRemoteID(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.TrimSuffix(value, ".git")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "ssh://")
	value = strings.TrimPrefix(value, "git@")
	value = strings.Replace(value, "github.com:", "github.com/", 1)
	if at := strings.Index(value, "@"); at >= 0 {
		value = value[at+1:]
	}
	value = strings.Trim(value, "/")
	if strings.Count(value, "/") < 2 {
		return ""
	}
	return value
}

func versionConstraintMatches(version, constraint string) bool {
	version = strings.TrimSpace(version)
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true
	}
	if version == "" {
		return false
	}
	terms := strings.Fields(strings.ReplaceAll(constraint, ",", " "))
	for _, term := range terms {
		if !versionTermMatches(version, term) {
			return false
		}
	}
	return true
}

func versionTermMatches(version, term string) bool {
	term = strings.TrimSpace(term)
	if term == "" {
		return true
	}
	op := "="
	for _, candidate := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(term, candidate) {
			op = candidate
			term = strings.TrimSpace(strings.TrimPrefix(term, candidate))
			break
		}
	}
	compare, ok := compareVersions(version, term)
	if !ok {
		return false
	}
	switch op {
	case ">=":
		return compare >= 0
	case "<=":
		return compare <= 0
	case ">":
		return compare > 0
	case "<":
		return compare < 0
	default:
		return compare == 0
	}
}

func compareVersions(left, right string) (int, bool) {
	leftParts, ok := semverParts(left)
	if !ok {
		return 0, false
	}
	rightParts, ok := semverParts(right)
	if !ok {
		return 0, false
	}
	for i := 0; i < len(leftParts); i++ {
		if leftParts[i] < rightParts[i] {
			return -1, true
		}
		if leftParts[i] > rightParts[i] {
			return 1, true
		}
	}
	return 0, true
}

func semverParts(value string) ([]int, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	if cut := strings.IndexAny(value, "-+"); cut >= 0 {
		value = value[:cut]
	}
	rawParts := strings.Split(value, ".")
	if len(rawParts) == 0 || len(rawParts) > 3 {
		return nil, false
	}
	parts := []int{0, 0, 0}
	for i, raw := range rawParts {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, false
		}
		parts[i] = n
	}
	return parts, true
}

func stringInFoldedSet(value string, candidates []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range candidates {
		if value == strings.ToLower(strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func appendUniqueString(existing []string, additions ...string) []string {
	for _, addition := range additions {
		if strings.TrimSpace(addition) == "" {
			continue
		}
		found := false
		for _, item := range existing {
			if item == addition {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, addition)
		}
	}
	return existing
}
