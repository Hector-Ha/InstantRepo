package analyzer

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/util"
)

var (
	envAssignmentPattern = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$`)

	// Code scanner patterns
	jsEnvPattern        = regexp.MustCompile(`process\.env\.([A-Za-z_][A-Za-z0-9_]*)`)
	goEnvPattern        = regexp.MustCompile(`os\.Getenv\(['"]([A-Za-z_][A-Za-z0-9_]*)['"]\)`)
	pyEnvPattern        = regexp.MustCompile(`os\.(?:environ\.get|getenv)\(['"]([A-Za-z_][A-Za-z0-9_]*)['"]\)|os\.environ\[['"]([A-Za-z_][A-Za-z0-9_]*)['"]\]`)
	jsDotenvPathPattern = regexp.MustCompile(`(?:dotenv\.)?config\s*\(\s*\{[^}]*path\s*:\s*['"]([^'"]+)['"]`)
)

type envFileRole string

const (
	envFileRoleLocal    envFileRole = "local"
	envFileRoleTemplate envFileRole = "template"
	envFileRoleNonLocal envFileRole = "non_local"
)

type envFileFinding struct {
	Path       string
	Role       envFileRole
	Confidence float64
}

func (a *RepositoryAnalyzer) enrichRuntimeContext(analysis *domain.RepositoryAnalysis) {
	if analysis == nil {
		return
	}

	runtimeContext := detectRuntimeContext(analysis.RepoPath)

	analysis.Env = runtimeContext.envConfig
	for _, service := range runtimeContext.services {
		if !hasService(analysis.Services, service.Name, service.Source) {
			analysis.Services = append(analysis.Services, service)
		}
	}
	for _, req := range runtimeContext.requirements {
		if !hasRequirement(analysis.Requirements, req.Tool) {
			analysis.Requirements = append(analysis.Requirements, req)
		}
	}
	for _, step := range runtimeContext.steps {
		if !hasStep(analysis.Steps, step.ID) {
			analysis.Steps = append(analysis.Steps, step)
		}
	}
	for _, item := range runtimeContext.evidence {
		if !slices.Contains(analysis.Evidence, item) {
			analysis.Evidence = append(analysis.Evidence, item)
		}
	}
	for _, item := range runtimeContext.unknowns {
		if !slices.Contains(analysis.Unknowns, item) {
			analysis.Unknowns = append(analysis.Unknowns, item)
		}
	}
	analysis.Topology = detectAppTopology(analysis.RepoPath, analysis.Env, analysis.Services)
}

type runtimeContext struct {
	envConfig    domain.EnvironmentConfig
	services     []domain.ServiceDependency
	requirements []domain.ToolRequirement
	steps        []domain.ExecutionStep
	evidence     []string
	unknowns     []string
}

func detectRuntimeContext(repoPath string) runtimeContext {
	services := []domain.ServiceDependency{}
	requirements := []domain.ToolRequirement{}
	steps := []domain.ExecutionStep{}
	evidence := []string{}
	unknowns := []string{}

	envTargets := inferEnvTargets(repoPath)
	envConfig := envTargets.envConfig
	evidence = append(evidence, envTargets.evidence...)
	unknowns = append(unknowns, envTargets.unknowns...)

	composePath := findFirstExisting(repoPath, []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	})
	localServices := map[string]domain.ServiceDependency{}
	if composePath != "" {
		evidence = append(evidence, filepath.Base(composePath)+" found")
		requirements = append(requirements, domain.ToolRequirement{
			Tool:              "docker",
			VersionConstraint: "",
			Source:            filepath.Base(composePath),
			Required:          true,
		})

		for _, serviceName := range parseComposeServiceNames(util.ReadTextFile(composePath)) {
			dependency := composeServiceDependency(serviceName, filepath.Base(composePath))
			if dependency.Name == "" {
				continue
			}
			localServices[dependency.Name] = dependency
			if !hasService(services, dependency.Name, dependency.Source) {
				services = append(services, dependency)
			}
		}

		relativeComposePath := filepath.Base(composePath)
		if rel, err := filepath.Rel(repoPath, composePath); err == nil {
			relativeComposePath = rel
		}
		steps = append(steps, domain.ExecutionStep{
			ID:               "start-local-services",
			Title:            "Start local Docker services",
			Command:          fmt.Sprintf("docker compose -f \"%s\" up -d", relativeComposePath),
			Cwd:              repoPath,
			Type:             "service-start",
			Importance:       domain.StepRequired,
			Risk:             domain.RiskMedium,
			RequiresApproval: true,
			EvidenceSource:   "config",
			ConfirmedBy:      []string{filepath.Base(composePath)},
			Confidence:       0.91,
			Reason:           "Docker Compose services were detected and likely need to be running before the app starts.",
		})
	}

	existingValues := map[string]string{}
	if envConfig.TargetExists {
		existingValues = parseEnvValues(envConfig.TargetPath)
		evidence = append(evidence, ".env found in "+filepath.Dir(envConfig.TargetPath))
	}

	var classifiedVars []domain.EnvVarRequirement
	for _, envVar := range envConfig.Variables {
		envRequirement := classifyEnvVar(envVar, existingValues, localServices)
		classifiedVars = append(classifiedVars, envRequirement)
	}
	envConfig.Variables = classifiedVars

	if len(envConfig.Variables) > 0 {
		confirmedBy := []string{"source scan"}
		if envConfig.TemplatePath != "" {
			confirmedBy = []string{filepath.Base(envConfig.TemplatePath)}
		}

		if hasUnconfiguredUserVars(envConfig.Variables) {
			steps = append(steps, domain.ExecutionStep{
				ID:               "review-env-values",
				Title:            "Review unresolved env variables",
				Command:          "manual env review required",
				Cwd:              repoPath,
				Type:             "env-review",
				Importance:       domain.StepManual,
				Risk:             domain.RiskHigh,
				RequiresApproval: true,
				EvidenceSource:   "config",
				ConfirmedBy:      confirmedBy,
				Confidence:       0.93,
				Reason:           "Some required env variables depend on user-provided online services or secrets.",
			})
		}
		if hasAutoFillableEnvVars(envConfig.Variables) {
			evidence = append(evidence, "Some env variables appear auto-fillable from local service defaults")
		}
	}

	for _, envVar := range envConfig.Variables {
		if envVar.Service == "" {
			continue
		}
		if _, ok := localServices[envVar.Service]; ok {
			continue
		}

		dependency := externalServiceDependency(envVar)
		if dependency.Name == "" || hasService(services, dependency.Name, dependency.Source) {
			continue
		}
		services = append(services, dependency)
	}

	if composePath == "" && len(envConfig.Variables) > 0 && hasLikelyLocalDataStore(envConfig.Variables) {
		unknowns = append(unknowns, "Service-related env vars were detected, but no Docker Compose file was found for local provisioning")
	}

	return runtimeContext{
		envConfig:    envConfig,
		services:     services,
		requirements: requirements,
		steps:        steps,
		evidence:     evidence,
		unknowns:     unknowns,
	}
}

func choosePrimaryEnvTemplate(repoPath string, templates []string) string {
	best := templates[0]
	bestDepth := strings.Count(filepath.Clean(best), string(filepath.Separator))
	repoDepth := strings.Count(filepath.Clean(repoPath), string(filepath.Separator))

	for _, candidate := range templates[1:] {
		candidateDepth := strings.Count(filepath.Clean(candidate), string(filepath.Separator))
		bestRelativeDepth := bestDepth - repoDepth
		candidateRelativeDepth := candidateDepth - repoDepth

		if candidateRelativeDepth < bestRelativeDepth ||
			(candidateRelativeDepth == bestRelativeDepth && len(candidate) < len(best)) {
			best = candidate
			bestDepth = candidateDepth
		}
	}

	return best
}

func mergeEnvVars(existing, incoming []domain.EnvVarRequirement) []domain.EnvVarRequirement {
	if len(incoming) == 0 {
		return existing
	}

	seen := make(map[string]bool, len(existing))
	for _, item := range existing {
		seen[envVarMergeKey(item)] = true
	}

	for _, item := range incoming {
		key := envVarMergeKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true
		existing = append(existing, item)
	}

	return existing
}

func envVarMergeKey(item domain.EnvVarRequirement) string {
	return item.Name + "\x00" + item.TargetDir
}

func findAllTemplates(repoPath string, candidates []string) []string {
	var matches []string
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" || name == "build" || name == "dist" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		for _, candidate := range candidates {
			if name == candidate {
				matches = append(matches, path)
				return nil
			}
		}
		return nil
	})
	return matches
}

func findEnvFiles(repoPath string) []envFileFinding {
	var matches []envFileFinding
	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" || name == "build" || name == "dist" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if !strings.HasPrefix(name, ".env") {
			return nil
		}
		matches = append(matches, envFileFinding{
			Path:       path,
			Role:       classifyEnvFileRole(name),
			Confidence: envFileConfidence(name),
		})
		return nil
	})
	return matches
}

func classifyEnvFileRole(name string) envFileRole {
	lowerName := strings.ToLower(name)
	for _, marker := range []string{"production", "prod", "staging", "test", "ci"} {
		if envFileNameHasMarker(lowerName, marker) {
			return envFileRoleNonLocal
		}
	}
	if strings.Contains(lowerName, "example") || strings.Contains(lowerName, "sample") || strings.Contains(lowerName, "template") {
		return envFileRoleTemplate
	}
	return envFileRoleLocal
}

func envFileConfidence(name string) float64 {
	switch strings.ToLower(name) {
	case ".env", ".env.local", ".env.development", ".env.dev", ".env.example", ".env.sample", ".env.template", ".env.local.example", ".env.development.example":
		return 0.9
	default:
		return 0.45
	}
}

func envFileNameHasMarker(name, marker string) bool {
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '.' || r == '-' || r == '_'
	}) {
		if part == marker {
			return true
		}
	}
	return false
}

func findFirstExisting(repoPath string, candidates []string) string {
	for _, candidate := range candidates {
		fullPath := filepath.Join(repoPath, candidate)
		if util.FileExists(fullPath) {
			return fullPath
		}
	}
	return ""
}

func parseEnvValues(path string) map[string]string {
	result := map[string]string{}
	if path == "" || !util.FileExists(path) {
		return result
	}

	for _, item := range parseEnvTemplate(path) {
		result[item.Name] = item.SuggestedValue
	}
	return result
}

func parseEnvTemplate(path string) []domain.EnvVarRequirement {
	if path == "" || !util.FileExists(path) {
		return []domain.EnvVarRequirement{}
	}

	raw := util.ReadTextFile(path)
	var variables []domain.EnvVarRequirement
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		matches := envAssignmentPattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if len(matches) != 3 {
			continue
		}

		name := strings.TrimSpace(matches[1])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		variables = append(variables, domain.EnvVarRequirement{
			Name:           name,
			Source:         filepath.Base(path),
			Required:       true,
			Secret:         looksSensitiveEnvVar(name),
			CurrentStatus:  "missing",
			FillStrategy:   "user_required",
			SuggestedValue: cleanEnvValue(matches[2]),
		})
	}

	return variables
}

func findNearestManifest(repoRoot, startDir string) string {
	current := startDir
	for {
		if util.FileExists(filepath.Join(current, "package.json")) ||
			util.FileExists(filepath.Join(current, "pyproject.toml")) ||
			util.FileExists(filepath.Join(current, "go.mod")) ||
			util.FileExists(filepath.Join(current, "requirements.txt")) {
			return current
		}
		if current == repoRoot || current == filepath.Dir(current) || current == "." || current == "/" {
			break
		}
		current = filepath.Dir(current)
	}
	return repoRoot
}

func scanCodeForEnvVars(repoPath string) ([]domain.EnvVarRequirement, string, []domain.SourceFixSuggestion, []string) {
	seen := map[string]bool{
		"NODE_ENV": true,
		"PATH":     true,
		"HOME":     true,
		"USER":     true,
		"PWD":      true,
		"TZ":       true,
		"PORT":     true,
	}

	var variables []domain.EnvVarRequirement
	targetPathVotes := make(map[string]int)
	var sourceFixes []domain.SourceFixSuggestion
	var unknowns []string

	_ = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" || name == "build" || name == "dist" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		var pattern *regexp.Regexp

		switch ext {
		case ".js", ".jsx", ".ts", ".tsx":
			pattern = jsEnvPattern
		case ".go":
			pattern = goEnvPattern
		case ".py":
			pattern = pyEnvPattern
		default:
			return nil
		}

		content := util.ReadTextFile(path)
		if content == "" {
			return nil
		}
		loaderTargetDir, hasUnsafeLoader := safeDotenvLoaderTargetDir(repoPath, content)
		if hasUnsafeLoader {
			relativeSource := relativeEnvSourcePath(repoPath, path)
			sourceFixes = append(sourceFixes, domain.SourceFixSuggestion{
				FilePath:      relativeSource,
				Summary:       "Env loader path points outside the repo.",
				SuggestedText: "Point dotenv loading at a repo-local env file such as ./.env.",
			})
			unknowns = append(unknowns, fmt.Sprintf("%s loads env from outside the repo; InstantRepo will not write that target", relativeSource))
		}

		matches := pattern.FindAllStringSubmatch(content, -1)
		foundAny := false

		for _, match := range matches {
			var name string
			if len(match) > 1 && match[1] != "" {
				name = match[1]
			} else if len(match) > 2 && match[2] != "" {
				name = match[2]
			}

			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			foundAny = true

			targetDir := inferEnvUsageTargetDir(repoPath, path)
			if loaderTargetDir != "" {
				targetDir = loaderTargetDir
			}
			if hasUnsafeLoader {
				targetDir = ""
			}

			variables = append(variables, domain.EnvVarRequirement{
				Name:           name,
				Source:         "code scan",
				Required:       true,
				Secret:         looksSensitiveEnvVar(name),
				Confidence:     0.72,
				CurrentStatus:  "missing",
				FillStrategy:   "user_required",
				TargetDir:      targetDir,
				SuggestedValue: "",
			})
		}

		if foundAny {
			targetDir := inferEnvUsageTargetDir(repoPath, path)
			if loaderTargetDir != "" {
				targetDir = loaderTargetDir
			}
			if hasUnsafeLoader {
				targetDir = ""
			}
			if targetDir != "" {
				targetPathVotes[targetDir] += len(matches)
			}
		}

		return nil
	})

	bestTarget := ""
	maxVotes := -1
	for dir, votes := range targetPathVotes {
		if votes > maxVotes {
			maxVotes = votes
			bestTarget = dir
		}
	}

	return variables, bestTarget, sourceFixes, unknowns
}

func safeDotenvLoaderTargetDir(repoRoot, content string) (string, bool) {
	matches := jsDotenvPathPattern.FindStringSubmatch(content)
	if len(matches) != 2 {
		return "", false
	}
	loaderPath := filepath.Clean(matches[1])
	if !filepath.IsAbs(loaderPath) {
		loaderPath = filepath.Join(repoRoot, loaderPath)
	}
	if !pathInsideRepo(repoRoot, loaderPath) {
		return "", true
	}
	return filepath.Dir(loaderPath), false
}

func relativeEnvSourcePath(repoRoot, sourcePath string) string {
	relative, err := filepath.Rel(repoRoot, sourcePath)
	if err != nil || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.Base(sourcePath)
	}
	return relative
}

func pathInsideRepo(repoRoot, path string) bool {
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(repoAbs, pathAbs)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func inferEnvUsageTargetDir(repoRoot, sourcePath string) string {
	sourceDir := filepath.Dir(sourcePath)
	manifestDir := findNearestManifest(repoRoot, sourceDir)
	if manifestDir != repoRoot {
		return manifestDir
	}

	relative, err := filepath.Rel(repoRoot, sourcePath)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return repoRoot
	}
	parts := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	if len(parts) < 2 {
		return repoRoot
	}
	switch strings.ToLower(parts[0]) {
	case "src", "lib", "pkg", "cmd", "internal":
		return repoRoot
	default:
		return filepath.Join(repoRoot, parts[0])
	}
}

func classifyEnvVar(input domain.EnvVarRequirement, existingValues map[string]string, localServices map[string]domain.ServiceDependency) domain.EnvVarRequirement {
	envVar := input
	if value, ok := existingValues[envVar.Name]; ok {
		if strings.TrimSpace(value) != "" {
			envVar.CurrentStatus = "configured"
		} else {
			envVar.CurrentStatus = "present_empty"
		}
	}
	if envVar.CurrentStatus == "" {
		envVar.CurrentStatus = "missing"
	}

	service, scope := inferServiceFromEnvVar(envVar.Name, envVar.SuggestedValue)
	if service == "" {
		service = inferServiceFromLocalContext(envVar.Name, localServices)
	}
	envVar.Service = service

	if service != "" {
		if localService, ok := localServices[service]; ok {
			envVar.FillStrategy = "auto_fillable"
			envVar.Instructions = localEnvInstructions(service, envVar.Name)
			if envVar.CurrentStatus != "configured" {
				envVar.SuggestedValue = suggestedLocalValue(service, envVar.Name)
			}
			if localService.Provisioning == "docker-compose" && envVar.CurrentStatus == "configured" {
				envVar.FillStrategy = "configured"
			}
			return envVar
		}

		if scope == "external" || envVar.CurrentStatus != "configured" {
			envVar.FillStrategy = "user_required"
			envVar.SuggestedValue = ""
			envVar.Instructions = externalEnvInstructions(service, envVar.Name)
			return envVar
		}
	}

	if envVar.CurrentStatus == "configured" {
		envVar.FillStrategy = "configured"
		envVar.SuggestedValue = ""
		envVar.Instructions = nil
		return envVar
	}

	if envVar.Secret {
		envVar.FillStrategy = "user_required"
		envVar.SuggestedValue = ""
		envVar.Instructions = genericSecretInstructions(envVar.Name)
		return envVar
	}

	envVar.FillStrategy = "template_only"
	envVar.Instructions = []string{
		fmt.Sprintf("Review %s and replace the placeholder value if the application requires a real setting.", envVar.Name),
	}
	return envVar
}

func inferServiceFromLocalContext(varName string, localServices map[string]domain.ServiceDependency) string {
	lowerName := strings.ToLower(varName)
	switch {
	case lowerName == "database_url" || strings.Contains(lowerName, "db_"):
		for _, candidate := range []string{"postgres", "mysql", "mongodb"} {
			if _, ok := localServices[candidate]; ok {
				return candidate
			}
		}
	}
	return ""
}

func inferServiceFromEnvVar(name, value string) (string, string) {
	lowerName := strings.ToLower(name)
	lowerValue := strings.ToLower(value)

	switch {
	case strings.Contains(lowerName, "mongodb") || strings.Contains(lowerName, "mongo"):
		if strings.Contains(lowerValue, "mongodb+srv://") || strings.Contains(lowerValue, "atlas") {
			return "mongodb", "external"
		}
		return "mongodb", "unknown"
	case strings.Contains(lowerName, "postgres"):
		return "postgres", "unknown"
	case lowerName == "database_url" && strings.Contains(lowerValue, "postgres"):
		return "postgres", "unknown"
	case strings.Contains(lowerName, "redis"):
		return "redis", "unknown"
	case strings.Contains(lowerName, "mysql"):
		return "mysql", "unknown"
	case strings.Contains(lowerName, "stripe"):
		return "stripe", "external"
	case strings.Contains(lowerName, "openai"):
		return "openai", "external"
	case strings.Contains(lowerName, "supabase"):
		return "supabase", "external"
	case strings.Contains(lowerName, "firebase"):
		return "firebase", "external"
	case strings.Contains(lowerName, "clerk"):
		return "clerk", "external"
	default:
		return "", "unknown"
	}
}

func suggestedLocalValue(service, varName string) string {
	name := strings.ToLower(varName)
	switch service {
	case "postgres":
		switch {
		case strings.Contains(name, "url"):
			return "postgres://postgres:postgres@localhost:5432/postgres"
		case strings.Contains(name, "host"):
			return "localhost"
		case strings.Contains(name, "port"):
			return "5432"
		case strings.Contains(name, "user"):
			return "postgres"
		case strings.Contains(name, "password"), strings.Contains(name, "pass"):
			return "postgres"
		case strings.Contains(name, "database"), strings.Contains(name, "db"):
			return "postgres"
		}
	case "mongodb":
		switch {
		case strings.Contains(name, "url"), strings.Contains(name, "uri"):
			return "mongodb://localhost:27017/app"
		case strings.Contains(name, "host"):
			return "localhost"
		case strings.Contains(name, "port"):
			return "27017"
		}
	case "redis":
		switch {
		case strings.Contains(name, "url"):
			return "redis://localhost:6379"
		case strings.Contains(name, "host"):
			return "localhost"
		case strings.Contains(name, "port"):
			return "6379"
		}
	case "mysql":
		switch {
		case strings.Contains(name, "url"):
			return "mysql://root:root@localhost:3306/app"
		case strings.Contains(name, "host"):
			return "localhost"
		case strings.Contains(name, "port"):
			return "3306"
		case strings.Contains(name, "user"):
			return "root"
		case strings.Contains(name, "password"), strings.Contains(name, "pass"):
			return "root"
		}
	}
	return ""
}

func parseComposeServiceNames(raw string) []string {
	lines := strings.Split(raw, "\n")
	servicesIndent := -1
	inServices := false
	result := []string{}
	seen := map[string]bool{}

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		if !inServices {
			if trimmed == "services:" {
				inServices = true
				servicesIndent = indent
			}
			continue
		}

		if indent <= servicesIndent {
			break
		}
		if strings.HasSuffix(trimmed, ":") && !strings.Contains(trimmed, " ") {
			name := strings.TrimSuffix(trimmed, ":")
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	return result
}

func composeServiceDependency(serviceName, source string) domain.ServiceDependency {
	lowerName := strings.ToLower(serviceName)
	switch {
	case strings.Contains(lowerName, "postgres"):
		return domain.ServiceDependency{
			Name:         "postgres",
			Scope:        "local",
			Provisioning: "docker-compose",
			Source:       source,
			Status:       "provisionable",
			Details:      fmt.Sprintf("Compose service %q looks like a local PostgreSQL dependency.", serviceName),
		}
	case strings.Contains(lowerName, "mongo"):
		return domain.ServiceDependency{
			Name:         "mongodb",
			Scope:        "local",
			Provisioning: "docker-compose",
			Source:       source,
			Status:       "provisionable",
			Details:      fmt.Sprintf("Compose service %q looks like a local MongoDB dependency.", serviceName),
		}
	case strings.Contains(lowerName, "redis"):
		return domain.ServiceDependency{
			Name:         "redis",
			Scope:        "local",
			Provisioning: "docker-compose",
			Source:       source,
			Status:       "provisionable",
			Details:      fmt.Sprintf("Compose service %q looks like a local Redis dependency.", serviceName),
		}
	case strings.Contains(lowerName, "mysql"):
		return domain.ServiceDependency{
			Name:         "mysql",
			Scope:        "local",
			Provisioning: "docker-compose",
			Source:       source,
			Status:       "provisionable",
			Details:      fmt.Sprintf("Compose service %q looks like a local MySQL dependency.", serviceName),
		}
	default:
		return domain.ServiceDependency{}
	}
}

func externalServiceDependency(envVar domain.EnvVarRequirement) domain.ServiceDependency {
	if envVar.Service == "" {
		return domain.ServiceDependency{}
	}

	scope := "external"
	details := fmt.Sprintf("Env var %q appears to require a user-provided %s service configuration.", envVar.Name, envVar.Service)
	if envVar.FillStrategy != "user_required" {
		scope = "unknown"
	}

	return domain.ServiceDependency{
		Name:         envVar.Service,
		Scope:        scope,
		Provisioning: "user-provided",
		Source:       envVar.Source,
		Status:       "user_required",
		Details:      details,
		Instructions: envVar.Instructions,
	}
}

func hasUnconfiguredUserVars(vars []domain.EnvVarRequirement) bool {
	for _, item := range vars {
		if item.CurrentStatus != "configured" && item.FillStrategy == "user_required" {
			return true
		}
	}
	return false
}

func hasAutoFillableEnvVars(vars []domain.EnvVarRequirement) bool {
	for _, item := range vars {
		if item.FillStrategy == "auto_fillable" {
			return true
		}
	}
	return false
}

func hasLikelyLocalDataStore(vars []domain.EnvVarRequirement) bool {
	for _, item := range vars {
		switch item.Service {
		case "postgres", "mongodb", "redis", "mysql":
			return true
		}
	}
	return false
}

func looksSensitiveEnvVar(name string) bool {
	lowerName := strings.ToLower(name)
	sensitiveTerms := []string{
		"secret",
		"token",
		"password",
		"passwd",
		"api_key",
		"apikey",
		"private_key",
		"client_secret",
		"database_url",
		"uri",
	}
	for _, term := range sensitiveTerms {
		if strings.Contains(lowerName, term) {
			return true
		}
	}
	return false
}

func cleanEnvValue(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, `"'`)
	if strings.EqualFold(trimmed, "changeme") || strings.EqualFold(trimmed, "your_value_here") {
		return ""
	}
	return trimmed
}

func localEnvInstructions(service, varName string) []string {
	switch service {
	case "postgres", "mongodb", "redis", "mysql":
		return []string{
			fmt.Sprintf("InstantRepo can prefill %s with a local %s default.", varName, service),
			fmt.Sprintf("Start the local %s service before launching the application.", service),
			fmt.Sprintf("Review %s if your project expects non-default ports, users, or database names.", varName),
		}
	default:
		return []string{
			fmt.Sprintf("Review %s before running the project.", varName),
		}
	}
}

func externalEnvInstructions(service, varName string) []string {
	switch service {
	case "openai":
		return []string{
			"Sign in to your OpenAI account and create an API key.",
			fmt.Sprintf("Paste that key into %s in the generated .env file.", varName),
			"Keep the key private and do not commit it to source control.",
		}
	case "mongodb":
		return []string{
			"Create or open your MongoDB Atlas project and cluster.",
			"Create a database user and allow your IP or development network access.",
			fmt.Sprintf("Copy the Atlas connection string into %s in the generated .env file.", varName),
		}
	case "supabase":
		return []string{
			"Create or open your Supabase project.",
			fmt.Sprintf("Copy the project URL or API keys into %s in the generated .env file.", varName),
			"Use anon keys only in client-side code unless the repo explicitly requires server credentials.",
		}
	case "firebase":
		return []string{
			"Create or open your Firebase project in the Firebase console.",
			fmt.Sprintf("Copy the required config value into %s in the generated .env file.", varName),
			"If the project requires an admin SDK secret, keep it server-side only.",
		}
	case "stripe":
		return []string{
			"Create or open your Stripe account and locate the relevant API keys.",
			fmt.Sprintf("Paste the correct publishable or secret key into %s in the generated .env file.", varName),
			"Use test keys for local development unless the repo specifically requires live keys.",
		}
	case "clerk":
		return []string{
			"Create or open your Clerk application.",
			fmt.Sprintf("Copy the required publishable or secret key into %s in the generated .env file.", varName),
			"Match frontend and backend keys to the environment you are using.",
		}
	default:
		return genericSecretInstructions(varName)
	}
}

func genericSecretInstructions(varName string) []string {
	return []string{
		fmt.Sprintf("Obtain the required value for %s from the relevant service dashboard or project owner.", varName),
		fmt.Sprintf("Paste the real value into %s in the generated .env file.", varName),
		"Do not commit secrets to source control.",
	}
}

func hasRequirement(requirements []domain.ToolRequirement, tool string) bool {
	for _, item := range requirements {
		if item.Tool == tool {
			return true
		}
	}
	return false
}

func hasStep(steps []domain.ExecutionStep, id string) bool {
	for _, item := range steps {
		if item.ID == id {
			return true
		}
	}
	return false
}

func hasService(services []domain.ServiceDependency, name, source string) bool {
	for _, item := range services {
		if item.Name == name && item.Source == source {
			return true
		}
	}
	return false
}
