package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"instantrepo/internal/domain"
)

const envSaveAttempts = 3

type EnvDraftManager struct {
	writeEnvTarget func(path string, content []byte) error
}

func NewEnvDraftManager() *EnvDraftManager {
	return &EnvDraftManager{
		writeEnvTarget: atomicWriteEnvTarget,
	}
}

func (m *EnvDraftManager) BuildDraft(analysis domain.RepositoryAnalysis) (domain.EnvDraft, error) {
	if strings.TrimSpace(analysis.Env.TargetPath) == "" && len(analysis.Env.Variables) == 0 {
		return domain.EnvDraft{}, fmt.Errorf("env target path is not available")
	}

	targetPaths, varsByTarget := envDraftTargets(analysis)
	draft := domain.EnvDraft{RepoPath: analysis.RepoPath}
	for _, targetPath := range targetPaths {
		target, err := buildEnvDraftTarget(analysis.RepoPath, targetPath, varsByTarget[targetPath])
		if err != nil {
			return domain.EnvDraft{}, err
		}
		draft.Targets = append(draft.Targets, target)
	}
	return draft, nil
}

func (m *EnvDraftManager) SaveAll(draft domain.EnvDraft) (domain.EnvSaveResult, error) {
	result := domain.EnvSaveResult{}
	if err := validateEnvDraftTargets(draft); err != nil {
		for _, target := range draft.Targets {
			if validateErr := validateEnvDraftTarget(draft.RepoPath, target); validateErr != nil {
				result.Targets = append(result.Targets, envSaveFailure(target.RelativePath, "invalid_target"))
				return result, err
			}
		}
		return result, err
	}

	rollbacks := []envTargetRollback{}
	for _, target := range draft.Targets {
		rendered := renderEnvDraftTarget(target)
		rollback, err := readEnvTargetRollback(target.AbsolutePath)
		if err != nil {
			result.Targets = append(result.Targets, envSaveFailure(target.RelativePath, "read_failed"))
			rollbackEnvTargets(rollbacks)
			return result, fmt.Errorf("read env target %s before save: %w", target.RelativePath, err)
		}
		if err := os.MkdirAll(filepath.Dir(target.AbsolutePath), 0o755); err != nil {
			result.Targets = append(result.Targets, envSaveFailure(target.RelativePath, "permission"))
			rollbackEnvTargets(rollbacks)
			return result, fmt.Errorf("create env directory for %s: %w", target.RelativePath, err)
		}
		if err := m.writeEnvTargetWithRetry(target.AbsolutePath, []byte(rendered)); err != nil {
			result.Targets = append(result.Targets, envSaveFailure(target.RelativePath, "write_failed"))
			rollbackEnvTargets(append(rollbacks, rollback))
			return result, envSaveError(target.RelativePath, "write_failed")
		}
		rollbacks = append(rollbacks, rollback)
		result.Targets = append(result.Targets, domain.EnvSaveTargetResult{
			RelativePath: target.RelativePath,
			Succeeded:    true,
		})
	}
	return result, nil
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

func buildEnvDraftTarget(repoPath, targetPath string, vars []domain.EnvVarRequirement) (domain.EnvDraftTarget, error) {
	raw, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return domain.EnvDraftTarget{}, fmt.Errorf("read env target: %w", err)
	}
	original := string(raw)
	valuesByName := parseEnvAssignments(original)

	target := domain.EnvDraftTarget{
		RelativePath:    relativeEnvTargetPath(repoPath, targetPath),
		AbsolutePath:    targetPath,
		OriginalContent: original,
	}
	for _, item := range vars {
		value := domain.EnvDraftValue{
			Name:       item.Name,
			Value:      item.SuggestedValue,
			Secret:     item.Secret,
			Confidence: 0.5,
			Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceDraft},
		}
		if existing, ok := valuesByName[item.Name]; ok && strings.TrimSpace(existing) != "" {
			value.Value = existing
			value.Confidence = 1
			value.Provenance.Source = domain.EnvValueSourceExistingFile
		}
		target.Values = append(target.Values, value)
	}

	return target, nil
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
		builder.WriteString(formatEnvAssignment(name, value.Value))
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
	for _, item := range analysis.Env.Variables {
		targetPath := analysis.Env.TargetPath
		if strings.TrimSpace(item.TargetDir) != "" {
			targetPath = filepath.Join(item.TargetDir, ".env")
		}
		if strings.TrimSpace(targetPath) == "" {
			targetPath = filepath.Join(analysis.RepoPath, ".env")
		}
		if _, ok := varsByTarget[targetPath]; !ok {
			paths = append(paths, targetPath)
		}
		varsByTarget[targetPath] = append(varsByTarget[targetPath], item)
	}
	if len(paths) == 0 {
		targetPath := analysis.Env.TargetPath
		if strings.TrimSpace(targetPath) == "" {
			targetPath = filepath.Join(analysis.RepoPath, ".env")
		}
		paths = append(paths, targetPath)
		varsByTarget[targetPath] = nil
	}
	return paths, varsByTarget
}
