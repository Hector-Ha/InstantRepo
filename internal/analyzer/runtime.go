package analyzer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/util"
)

var envAssignmentPattern = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$`)

func (a *RepositoryAnalyzer) enrichRuntimeContext(analysis *domain.RepositoryAnalysis) {
	if analysis == nil {
		return
	}

	envConfig, services, requirements, steps, evidence, unknowns := detectRuntimeContext(analysis.RepoPath)

	analysis.Env = envConfig
	for _, service := range services {
		if !hasService(analysis.Services, service.Name, service.Source) {
			analysis.Services = append(analysis.Services, service)
		}
	}
	for _, req := range requirements {
		if !hasRequirement(analysis.Requirements, req.Tool) {
			analysis.Requirements = append(analysis.Requirements, req)
		}
	}
	for _, step := range steps {
		if !hasStep(analysis.Steps, step.ID) {
			analysis.Steps = append(analysis.Steps, step)
		}
	}
	for _, item := range evidence {
		if !slices.Contains(analysis.Evidence, item) {
			analysis.Evidence = append(analysis.Evidence, item)
		}
	}
	for _, item := range unknowns {
		if !slices.Contains(analysis.Unknowns, item) {
			analysis.Unknowns = append(analysis.Unknowns, item)
		}
	}
}

func detectRuntimeContext(repoPath string) (domain.EnvironmentConfig, []domain.ServiceDependency, []domain.ToolRequirement, []domain.ExecutionStep, []string, []string) {
	envConfig := domain.EnvironmentConfig{
		TargetPath: filepath.Join(repoPath, ".env"),
		Variables:  []domain.EnvVarRequirement{},
	}
	services := []domain.ServiceDependency{}
	requirements := []domain.ToolRequirement{}
	steps := []domain.ExecutionStep{}
	evidence := []string{}
	unknowns := []string{}

	envConfig.TargetExists = util.FileExists(envConfig.TargetPath)

	templatePath := findFirstExisting(repoPath, []string{
		".env.example",
		".env.sample",
		".env.template",
		".env.local.example",
		".env.development.example",
	})
	if templatePath != "" {
		envConfig.TemplatePath = templatePath
		evidence = append(evidence, filepath.Base(templatePath)+" found")
	}

	existingValues := map[string]string{}
	if envConfig.TargetExists {
		existingValues = parseEnvValues(envConfig.TargetPath)
		evidence = append(evidence, ".env found")
	}

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
			Risk:             domain.RiskMedium,
			RequiresApproval: true,
			Reason:           "Docker Compose services were detected and likely need to be running before the app starts.",
		})
	}

	templateVars := parseEnvTemplate(templatePath)
	for _, envVar := range templateVars {
		envRequirement := classifyEnvVar(envVar, existingValues, localServices)
		envConfig.Variables = append(envConfig.Variables, envRequirement)
	}

	if len(envConfig.Variables) > 0 {
		if hasUnconfiguredUserVars(envConfig.Variables) {
			steps = append(steps, domain.ExecutionStep{
				ID:               "review-env-values",
				Title:            "Review unresolved env variables",
				Command:          "manual env review required",
				Cwd:              repoPath,
				Type:             "env-review",
				Risk:             domain.RiskHigh,
				RequiresApproval: true,
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

	return envConfig, services, requirements, steps, evidence, unknowns
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
