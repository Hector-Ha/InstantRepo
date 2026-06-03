package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"instantrepo/internal/domain"
	"instantrepo/internal/envcatalog"
)

const envSaveAttempts = 3

type EnvDraftManager struct {
	writeEnvTarget   func(path string, content []byte) error
	generateSecret   func() (string, error)
	portAvailable    func(port int) bool
	portStore        EnvPortAssignmentStore
	catalog          envcatalog.Catalog
	generatedSecrets map[string]string
	assignedPorts    map[string]int
	loadedPortRepos  map[string]bool
}

type EnvPortAssignmentStore interface {
	EnvPortAssignments(ctx context.Context, repoPath string) ([]domain.EnvPortAssignment, error)
	SaveEnvPortAssignment(ctx context.Context, assignment domain.EnvPortAssignment) error
}

func NewEnvDraftManager() *EnvDraftManager {
	return &EnvDraftManager{
		writeEnvTarget:   atomicWriteEnvTarget,
		generateSecret:   generateEnvLocalSecret,
		portAvailable:    isLocalPortAvailable,
		catalog:          envcatalog.DefaultCatalog(),
		generatedSecrets: map[string]string{},
		assignedPorts:    map[string]int{},
		loadedPortRepos:  map[string]bool{},
	}
}

func (m *EnvDraftManager) Preview(analysis domain.RepositoryAnalysis) (string, error) {
	draft, err := m.BuildDraft(analysis)
	if err != nil {
		return "", err
	}
	return renderEnvDraft(draft), nil
}

func (m *EnvDraftManager) Prepare(analysis domain.RepositoryAnalysis) (domain.ExecutionResult, error) {
	started := time.Now()
	draft, err := m.BuildDraft(analysis)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	result, err := m.SaveAll(draft)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	return envDraftExecutionResult("create-env-file", "instantrepo internal:prepare-env", analysis.RepoPath, result, started), nil
}

func (m *EnvDraftManager) ApplyValues(analysis domain.RepositoryAnalysis, values map[string]string) (domain.ExecutionResult, error) {
	started := time.Now()
	draft, err := m.BuildDraft(analysis)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	applyEnvDraftValues(draft.Targets, values)
	result, err := m.SaveAll(draft)
	if err != nil {
		return domain.ExecutionResult{}, err
	}
	return envDraftExecutionResult("create-env-file", "instantrepo internal:prepare-env", analysis.RepoPath, result, started), nil
}

func (m *EnvDraftManager) SaveRaw(repoPath, targetPath, content string) (domain.ExecutionResult, error) {
	started := time.Now()
	if strings.TrimSpace(targetPath) == "" {
		return domain.ExecutionResult{}, fmt.Errorf("env target path is not available")
	}

	target := domain.EnvDraftTarget{
		RelativePath: relativeEnvTargetPath(repoPath, targetPath),
		AbsolutePath: targetPath,
	}
	if err := validateEnvDraftTarget(repoPath, target); err != nil {
		return domain.ExecutionResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("create env directory for %s: %w", target.RelativePath, err)
	}
	if err := m.writeEnvTargetWithRetry(targetPath, []byte(content)); err != nil {
		return domain.ExecutionResult{}, envSaveError(target.RelativePath, "write_failed")
	}

	return envDraftExecutionResult(
		"save-env-file",
		"instantrepo internal:save-env",
		repoPath,
		domain.EnvSaveResult{Targets: []domain.EnvSaveTargetResult{{
			RelativePath: target.RelativePath,
			Succeeded:    true,
		}}},
		started,
	), nil
}

func (m *EnvDraftManager) BuildDraft(analysis domain.RepositoryAnalysis) (domain.EnvDraft, error) {
	if strings.TrimSpace(analysis.Env.TargetPath) == "" && len(analysis.Env.Variables) == 0 {
		return domain.EnvDraft{}, fmt.Errorf("env target path is not available")
	}

	targetPaths, varsByTarget := envDraftTargets(analysis)
	if len(targetPaths) == 0 {
		return domain.EnvDraft{}, fmt.Errorf("env target path is not available")
	}
	draft := domain.EnvDraft{RepoPath: analysis.RepoPath}
	for _, targetPath := range targetPaths {
		target, err := m.buildEnvDraftTarget(analysis.RepoPath, targetPath, varsByTarget[targetPath])
		if err != nil {
			return domain.EnvDraft{}, err
		}
		draft.Targets = append(draft.Targets, target)
	}
	return draft, nil
}

func (m *EnvDraftManager) SaveAll(draft domain.EnvDraft) (domain.EnvSaveResult, error) {
	return m.savePolicy().SaveAll(draft)
}

func (m *EnvDraftManager) preserveUntrackedServiceCredentialValues(draft *domain.EnvDraft) {
	if draft == nil {
		return
	}
	for targetIndex := range draft.Targets {
		target := &draft.Targets[targetIndex]
		raw, err := os.ReadFile(target.AbsolutePath)
		if err != nil {
			continue
		}
		existing := parseEnvAssignments(string(raw))
		tracked := map[string]bool{}
		for _, value := range target.Values {
			tracked[value.Name] = true
		}
		preserved := map[string]string{}
		for name, existingValue := range existing {
			if tracked[name] || strings.TrimSpace(existingValue) == "" {
				continue
			}
			if decision, ok := m.envCatalog().Classify(name); ok && decision.ValueClass == domain.EnvValueClassServiceCredential {
				preserved[name] = existingValue
			}
		}
		target.OriginalContent = replaceEnvAssignments(target.OriginalContent, preserved)
	}
}

func preserveExistingServiceCredentialValues(draft *domain.EnvDraft) {
	if draft == nil {
		return
	}
	for targetIndex := range draft.Targets {
		target := &draft.Targets[targetIndex]
		raw, err := os.ReadFile(target.AbsolutePath)
		if err != nil {
			continue
		}
		existing := parseEnvAssignments(string(raw))
		for valueIndex := range target.Values {
			value := &target.Values[valueIndex]
			if value.ValueClass != domain.EnvValueClassServiceCredential {
				continue
			}
			if !value.HasExistingValue {
				continue
			}
			existingValue, ok := existing[value.Name]
			if !ok || strings.TrimSpace(existingValue) == "" {
				continue
			}
			if strings.TrimSpace(value.Value) != "" {
				value.VaultBinding = nil
				continue
			}
			value.Value = existingValue
			value.VaultBinding = nil
			value.Provenance.Source = domain.EnvValueSourceExistingFile
		}
	}
}

func applyEnvDraftValues(targets []domain.EnvDraftTarget, values map[string]string) {
	for targetIndex := range targets {
		for valueIndex := range targets[targetIndex].Values {
			name := targets[targetIndex].Values[valueIndex].Name
			nextValue, ok := values[name]
			if !ok {
				continue
			}
			trimmed := strings.TrimSpace(nextValue)
			if trimmed == "" {
				continue
			}
			targets[targetIndex].Values[valueIndex].Value = trimmed
			targets[targetIndex].Values[valueIndex].Confidence = 1
			targets[targetIndex].Values[valueIndex].Provenance.Source = domain.EnvValueSourceDraft
		}
	}
}

func applyEditedEnvDraftValues(targets []domain.EnvDraftTarget, edited []domain.EnvDraftTarget) {
	editedByTarget := map[string]map[string]domain.EnvDraftValue{}
	for _, target := range edited {
		key := strings.TrimSpace(target.RelativePath)
		if key == "" {
			continue
		}
		values := map[string]domain.EnvDraftValue{}
		for _, value := range target.Values {
			values[value.Name] = value
		}
		editedByTarget[key] = values
	}

	for targetIndex := range targets {
		values := editedByTarget[targets[targetIndex].RelativePath]
		if len(values) == 0 {
			continue
		}
		applied := map[string]bool{}
		for valueIndex := range targets[targetIndex].Values {
			name := targets[targetIndex].Values[valueIndex].Name
			editedValue, ok := values[name]
			if !ok {
				continue
			}
			applied[name] = true
			targets[targetIndex].Values[valueIndex].Value = editedValue.Value
			targets[targetIndex].Values[valueIndex].VaultBinding = editedValue.VaultBinding
			if editedValue.Provenance.Source != "" {
				targets[targetIndex].Values[valueIndex].Provenance = editedValue.Provenance
			}
		}
		for _, editedValue := range values {
			if applied[editedValue.Name] || strings.TrimSpace(editedValue.Name) == "" {
				continue
			}
			if editedValue.Confidence == 0 {
				editedValue.Confidence = 1
			}
			if editedValue.Provenance.Source == "" {
				editedValue.Provenance.Source = domain.EnvValueSourceDraft
			}
			targets[targetIndex].Values = append(targets[targetIndex].Values, editedValue)
		}
	}
}

func envDraftExecutionResult(stepID, command, cwd string, saveResult domain.EnvSaveResult, started time.Time) domain.ExecutionResult {
	return domain.ExecutionResult{
		StepID:    stepID,
		Command:   command,
		Cwd:       cwd,
		Stdout:    envSaveStdout(saveResult),
		Duration:  time.Since(started).String(),
		Succeeded: true,
	}
}

func envSaveStdout(result domain.EnvSaveResult) string {
	var builder strings.Builder
	for _, target := range result.Targets {
		if !target.Succeeded {
			continue
		}
		label := strings.TrimSpace(target.RelativePath)
		if label == "" {
			label = "env target"
		}
		builder.WriteString("Saved ")
		builder.WriteString(label)
		builder.WriteString("\n")
	}
	return builder.String()
}

func validateEnvDraftTargets(draft domain.EnvDraft) error {
	for _, target := range draft.Targets {
		if err := validateEnvDraftTarget(draft.RepoPath, target); err != nil {
			return err
		}
	}
	return nil
}

func validateEnvDraftTarget(repoPath string, target domain.EnvDraftTarget) error {
	if strings.TrimSpace(repoPath) == "" {
		return fmt.Errorf("env draft repo path is required")
	}
	if strings.TrimSpace(target.AbsolutePath) == "" {
		return fmt.Errorf("env target %s path is required", target.RelativePath)
	}
	repoAbs, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolve env draft repo path: %w", err)
	}
	targetAbs, err := filepath.Abs(target.AbsolutePath)
	if err != nil {
		return fmt.Errorf("resolve env target %s: %w", target.RelativePath, err)
	}
	relative, err := filepath.Rel(repoAbs, targetAbs)
	if err != nil {
		return fmt.Errorf("resolve env target %s: %w", target.RelativePath, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("env target %s is outside repo", target.RelativePath)
	}
	return nil
}

type envTargetRollback struct {
	path    string
	content []byte
	existed bool
}

func readEnvTargetRollback(path string) (envTargetRollback, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		return envTargetRollback{path: path, content: raw, existed: true}, nil
	}
	if os.IsNotExist(err) {
		return envTargetRollback{path: path}, nil
	}
	return envTargetRollback{}, err
}

func rollbackEnvTargets(rollbacks []envTargetRollback) {
	for i := len(rollbacks) - 1; i >= 0; i-- {
		rollback := rollbacks[i]
		if !rollback.existed {
			_ = os.Remove(rollback.path)
			continue
		}
		_ = atomicWriteEnvTargetWithRetry(rollback.path, rollback.content)
	}
}

func atomicWriteEnvTargetWithRetry(path string, content []byte) error {
	return writeEnvTargetWithRetry(atomicWriteEnvTarget, path, content)
}

func (m *EnvDraftManager) writeEnvTargetWithRetry(path string, content []byte) error {
	writer := m.writeEnvTarget
	if writer == nil {
		writer = atomicWriteEnvTarget
	}
	return writeEnvTargetWithRetry(writer, path, content)
}

func writeEnvTargetWithRetry(writer func(string, []byte) error, path string, content []byte) error {
	var lastErr error
	for attempt := 0; attempt < envSaveAttempts; attempt++ {
		if err := writer(path, content); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func atomicWriteEnvTarget(path string, content []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".instantrepo-env-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return err
	}
	return replaceEnvTarget(tempPath, path)
}

func (m *EnvDraftManager) buildEnvDraftTarget(repoPath, targetPath string, vars []domain.EnvVarRequirement) (domain.EnvDraftTarget, error) {
	if err := m.envCatalog().Validate(); err != nil {
		return domain.EnvDraftTarget{}, err
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return domain.EnvDraftTarget{}, fmt.Errorf("read env target: %w", err)
	}
	original := string(raw)
	valuesByName := parseEnvAssignments(original)

	target := domain.EnvDraftTarget{
		RelativePath: relativeEnvTargetPath(repoPath, targetPath),
		AbsolutePath: targetPath,
	}
	serviceCredentialNames := map[string]bool{}
	for _, item := range vars {
		if decision, ok := m.envCatalog().Classify(item.Name); ok && decision.ValueClass == domain.EnvValueClassServiceCredential {
			serviceCredentialNames[item.Name] = true
		}
	}
	for name := range valuesByName {
		if decision, ok := m.envCatalog().Classify(name); ok && decision.ValueClass == domain.EnvValueClassServiceCredential {
			serviceCredentialNames[name] = true
		}
	}
	target.OriginalContent = redactServiceCredentialAssignments(original, serviceCredentialNames)
	for _, item := range vars {
		confidence := item.Confidence
		if confidence == 0 {
			confidence = 0.5
		}
		value := domain.EnvDraftValue{
			Name:         item.Name,
			Value:        item.SuggestedValue,
			Secret:       item.Secret,
			Instructions: append([]string{}, item.Instructions...),
			Confidence:   confidence,
			Provenance:   domain.EnvValueProvenance{Source: domain.EnvValueSourceDraft},
		}
		if decision, ok := m.envCatalog().Classify(item.Name); ok {
			applyDecision := decision.Action != envcatalog.ActionFillDevDefault || shouldApplyCatalogDevDefault(item)
			if applyDecision {
				value.ValueClass = decision.ValueClass
				value.Instructions = append(value.Instructions, decision.Instructions...)
				value.Attention = append(value.Attention, decision.Attention...)
				if decision.Confidence > 0 {
					value.Confidence = decision.Confidence
				}
			}
			switch decision.Action {
			case envcatalog.ActionGenerateLocalSecret:
				generated, err := m.generatedLocalSecret(repoPath, targetPath, item.Name)
				if err != nil {
					return domain.EnvDraftTarget{}, fmt.Errorf("generate local secret for %s: %w", item.Name, err)
				}
				value.Value = generated
				value.Provenance.Source = domain.EnvValueSourceGeneratedSecret
			case envcatalog.ActionFillDevDefault:
				if applyDecision {
					value.Value = decision.DefaultValue
					value.Provenance.Source = domain.EnvValueSourceCatalog
				}
			case envcatalog.ActionLeaveBlank:
				value.Value = ""
				value.Provenance.Source = domain.EnvValueSourceCatalog
			}
		}
		if item.DefaultSource != "" && strings.TrimSpace(item.SuggestedValue) != "" {
			value.Value = item.SuggestedValue
			value.ValueClass = item.DefaultClass
			if value.ValueClass == "" {
				value.ValueClass = domain.EnvValueClassDevDefault
			}
			value.Provenance.Source = item.DefaultSource
		}
		applyCloudDatastoreSuggestion(item, &value)
		m.applyDevDefaultAllocation(repoPath, targetPath, item, &value)
		if existing, ok := valuesByName[item.Name]; ok && strings.TrimSpace(existing) != "" && !shouldReplaceExistingEnvValue(value, existing) {
			value.Value = existing
			value.Confidence = 1
			value.Provenance.Source = domain.EnvValueSourceExistingFile
			if value.ValueClass == domain.EnvValueClassGeneratedLocalSecret && isWeakCustomEnvSecret(existing) {
				value.Attention = append(value.Attention, "Existing local secret looks weak. Review it before running the app.")
			}
			if value.ValueClass == domain.EnvValueClassServiceCredential {
				value.HasExistingValue = true
				value.Value = ""
			}
		}
		target.Values = append(target.Values, value)
	}

	return target, nil
}

func applyCloudDatastoreSuggestion(item domain.EnvVarRequirement, value *domain.EnvDraftValue) {
	if value == nil {
		return
	}
	lower := strings.ToLower(item.SuggestedValue)
	name := strings.ToUpper(strings.TrimSpace(item.Name))
	if strings.Contains(lower, "mongodb+srv://") || strings.Contains(lower, "atlas") || (isMongoDatabaseURLEnv(name) && hasTopologyServiceSignal(item, "mongodb") && !hasLocalMongoDBEvidence(item)) {
		value.Value = ""
		value.ValueClass = domain.EnvValueClassProviderConfig
		value.Instructions = append(value.Instructions, "Cloud MongoDB hint detected. Leave blank unless you have the real Atlas connection string.")
		value.Instructions = append(value.Instructions, "Local suggestion: mongodb://localhost:27017/"+envDatabaseName(item.ProjectName, ""))
		value.Attention = append(value.Attention, "Cloud datastore hint found; InstantRepo left the value blank and added a local suggestion.")
		value.Provenance.Source = domain.EnvValueSourceAllocator
		return
	}
	if isExternalPostgresURL(item.SuggestedValue) {
		value.Value = ""
		value.ValueClass = domain.EnvValueClassProviderConfig
		value.Instructions = append(value.Instructions, "External PostgreSQL hint detected. Leave blank unless you have the real connection string.")
		value.Instructions = append(value.Instructions, "Local suggestion: postgres://postgres:postgres@localhost:5432/"+envDatabaseName(item.ProjectName, ""))
		value.Attention = append(value.Attention, "External datastore hint found; InstantRepo left the value blank and added a local suggestion.")
		value.Provenance.Source = domain.EnvValueSourceAllocator
		return
	}
	if strings.Contains(name, "SUPABASE") || strings.Contains(name, "FIREBASE") {
		value.Value = ""
		value.Instructions = append(value.Instructions, "Local datastore suggestion: postgres://postgres:postgres@localhost:5432/"+envDatabaseName(item.ProjectName, ""))
		value.Attention = append(value.Attention, "Cloud provider datastore hint found; InstantRepo left the value blank.")
	}
}

func isExternalPostgresURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(lower, "postgres://") && !strings.HasPrefix(lower, "postgresql://") {
		return false
	}
	return !strings.Contains(lower, "@localhost:") && !strings.Contains(lower, "@127.0.0.1:")
}

func shouldApplyCatalogDevDefault(item domain.EnvVarRequirement) bool {
	name := strings.ToUpper(strings.TrimSpace(item.Name))
	if isDatabaseURLEnv(name) {
		return hasLocalDatastoreEvidence(item, "postgres")
	}
	return true
}

func isWeakCustomEnvSecret(value string) bool {
	normalized := strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'`))
	if isKnownWeakEnvPlaceholder(normalized) {
		return false
	}
	switch normalized {
	case "secret", "password", "default", "devsecret", "dev-secret", "123456", "123456789":
		return true
	default:
		return false
	}
}

func shouldReplaceExistingEnvValue(value domain.EnvDraftValue, existing string) bool {
	if value.ValueClass != domain.EnvValueClassGeneratedLocalSecret {
		return false
	}
	return isKnownWeakEnvPlaceholder(existing)
}

func isKnownWeakEnvPlaceholder(value string) bool {
	normalized := strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'`))
	switch normalized {
	case "changeme", "change_me", "change-me", "your_secret_here", "your-secret-here", "your_value_here", "your-value-here":
		return true
	default:
		return false
	}
}

func (m *EnvDraftManager) generatedLocalSecret(repoPath, targetPath, name string) (string, error) {
	if m.generatedSecrets == nil {
		m.generatedSecrets = map[string]string{}
	}
	key := repoPath + "\x00" + targetPath + "\x00" + name
	if value, ok := m.generatedSecrets[key]; ok {
		return value, nil
	}
	value, err := m.envSecretGenerator()()
	if err != nil {
		return "", err
	}
	m.generatedSecrets[key] = value
	return value, nil
}

func (m *EnvDraftManager) envCatalog() envcatalog.Catalog {
	if strings.TrimSpace(m.catalog.Version) == "" && len(m.catalog.Rules) == 0 {
		return envcatalog.DefaultCatalog()
	}
	return m.catalog
}

func (m *EnvDraftManager) envSecretGenerator() func() (string, error) {
	if m.generateSecret != nil {
		return m.generateSecret
	}
	return generateEnvLocalSecret
}

func generateEnvLocalSecret() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func renderEnvDraft(draft domain.EnvDraft) string {
	if len(draft.Targets) == 1 {
		return renderEnvDraftTarget(draft.Targets[0])
	}

	var builder strings.Builder
	for i, target := range draft.Targets {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("# Target ")
		builder.WriteString(target.RelativePath)
		builder.WriteString("\n")
		builder.WriteString(renderEnvDraftTarget(target))
	}
	return builder.String()
}

func renderEnvDraftTarget(target domain.EnvDraftTarget) string {
	valuesByName := map[string]domain.EnvDraftValue{}
	for _, value := range target.Values {
		valuesByName[value.Name] = value
	}

	seen := map[string]bool{}
	var builder strings.Builder
	lines := strings.Split(strings.ReplaceAll(target.OriginalContent, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		matches := envLinePattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if len(matches) != 3 {
			builder.WriteString(line)
			builder.WriteString("\n")
			continue
		}
		name := matches[1]
		value, ok := valuesByName[name]
		if !ok {
			builder.WriteString(line)
			builder.WriteString("\n")
			continue
		}
		seen[name] = true
		builder.WriteString(formatEnvAssignmentLike(line, name, value.Value))
		builder.WriteString("\n")
	}

	var appended []domain.EnvDraftValue
	for _, value := range target.Values {
		if !seen[value.Name] {
			appended = append(appended, value)
		}
	}
	if len(appended) > 0 {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("# Added by InstantRepo\n")
		for _, value := range appended {
			builder.WriteString(formatEnvAssignment(value.Name, value.Value))
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func envSaveFailure(relativePath, kind string) domain.EnvSaveTargetResult {
	return domain.EnvSaveTargetResult{
		RelativePath: relativePath,
		ErrorKind:    kind,
	}
}

func envSaveError(relativePath, kind string) error {
	return fmt.Errorf("save env target %s failed: %s", relativePath, kind)
}

func envDraftTargets(analysis domain.RepositoryAnalysis) ([]string, map[string][]domain.EnvVarRequirement) {
	paths := []string{}
	varsByTarget := map[string][]domain.EnvVarRequirement{}
	indexByTarget := map[string]map[string]int{}
	for _, item := range analysis.Env.Variables {
		item.TopologySignals = append(item.TopologySignals, analysis.Topology.Signals...)
		item.ProjectName = analysis.ProjectName
		targetPath := envDraftTargetPath(analysis.Env.TargetPath, item)
		if strings.TrimSpace(targetPath) == "" {
			continue
		}
		if _, ok := varsByTarget[targetPath]; !ok {
			paths = append(paths, targetPath)
			indexByTarget[targetPath] = map[string]int{}
		}
		if existingIndex, ok := indexByTarget[targetPath][item.Name]; ok {
			existing := varsByTarget[targetPath][existingIndex]
			if shouldReplaceEnvDraftRequirement(existing, item) {
				varsByTarget[targetPath][existingIndex] = item
			}
			continue
		}
		indexByTarget[targetPath][item.Name] = len(varsByTarget[targetPath])
		varsByTarget[targetPath] = append(varsByTarget[targetPath], item)
	}
	if len(paths) == 0 {
		targetPath := analysis.Env.TargetPath
		if strings.TrimSpace(targetPath) == "" {
			return paths, varsByTarget
		}
		paths = append(paths, targetPath)
		varsByTarget[targetPath] = nil
	}
	return paths, varsByTarget
}

func envDraftTargetPath(defaultTargetPath string, item domain.EnvVarRequirement) string {
	if strings.TrimSpace(item.TargetDir) != "" {
		if preservesLocalEnvFileName(item.Source) {
			return filepath.Join(item.TargetDir, item.Source)
		}
		return filepath.Join(item.TargetDir, ".env")
	}
	return defaultTargetPath
}

func shouldReplaceEnvDraftRequirement(existing, incoming domain.EnvVarRequirement) bool {
	return strings.TrimSpace(existing.TargetDir) == "" && strings.TrimSpace(incoming.TargetDir) != ""
}

func preservesLocalEnvFileName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case ".env.local", ".env.development", ".env.dev":
		return true
	default:
		return false
	}
}
