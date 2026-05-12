package service

import (
	"fmt"
	"os"
	"path/filepath"

	"instantrepo/internal/domain"
)

type envDraftSavePolicy struct {
	writeEnvTarget        func(path string, content []byte) error
	preserveUntrackedCred func(draft *domain.EnvDraft)
}

func (m *EnvDraftManager) savePolicy() envDraftSavePolicy {
	return envDraftSavePolicy{
		writeEnvTarget:        m.writeEnvTarget,
		preserveUntrackedCred: m.preserveUntrackedServiceCredentialValues,
	}
}

func (p envDraftSavePolicy) SaveAll(draft domain.EnvDraft) (domain.EnvSaveResult, error) {
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
	preserveExistingServiceCredentialValues(&draft)
	if p.preserveUntrackedCred != nil {
		p.preserveUntrackedCred(&draft)
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
		if err := p.writeEnvTargetWithRetry(target.AbsolutePath, []byte(rendered)); err != nil {
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

func (p envDraftSavePolicy) writeEnvTargetWithRetry(path string, content []byte) error {
	writer := p.writeEnvTarget
	if writer == nil {
		writer = atomicWriteEnvTarget
	}
	return writeEnvTargetWithRetry(writer, path, content)
}
