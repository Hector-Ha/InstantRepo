package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/envcatalog"
)

var (
	ErrCredentialStoreUnavailable = errors.New("credential store unavailable")
	ErrCredentialUnavailable      = errors.New("credential unavailable")
)

type CredentialStore interface {
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

type EnvVaultStore interface {
	SaveEnvVaultEntry(ctx context.Context, entry domain.EnvVaultEntryMetadata) (domain.EnvVaultEntryMetadata, error)
	UpdateEnvVaultEntryCredentialKey(ctx context.Context, entryID int64, credentialKey string) error
	DeleteEnvVaultEntry(ctx context.Context, entryID int64) error
	EnvVaultEntryByID(ctx context.Context, entryID int64) (domain.EnvVaultEntryMetadata, error)
	EnvVaultEntryByProviderFingerprint(ctx context.Context, provider, fingerprint string) (domain.EnvVaultEntryMetadata, error)
	SetEnvVaultEntryStatus(ctx context.Context, entryID int64, status string) error
	SaveEnvVaultApproval(ctx context.Context, approval domain.EnvVaultApproval) error
	ApprovedEnvVaultEntries(ctx context.Context, repoPath, targetRelativePath, variableName string) ([]domain.EnvVaultEntryMetadata, error)
	RecordEnvVaultUse(ctx context.Context, record domain.EnvVaultUseRecord) error
}

type EnvVaultService struct {
	store      EnvVaultStore
	credential CredentialStore
	catalog    envcatalog.Catalog
}

func NewEnvVaultService(store EnvVaultStore, credential CredentialStore) *EnvVaultService {
	return &EnvVaultService{
		store:      store,
		credential: credential,
		catalog:    envcatalog.DefaultCatalog(),
	}
}

func (s *AppService) SaveEnvVaultCredential(ctx context.Context, req domain.EnvVaultSaveRequest) (domain.EnvVaultSaveResponse, error) {
	if s.vault == nil {
		return domain.EnvVaultSaveResponse{}, ErrCredentialStoreUnavailable
	}
	return s.vault.SaveCredential(ctx, req)
}

func (s *AppService) ApproveEnvVaultEntry(ctx context.Context, approval domain.EnvVaultApproval) error {
	if s.vault == nil {
		return ErrCredentialStoreUnavailable
	}
	return s.vault.Approve(ctx, approval)
}

func (s *AppService) MarkEnvVaultEntryStatus(ctx context.Context, entryID int64, status string) error {
	if s.vault == nil {
		return ErrCredentialStoreUnavailable
	}
	return s.vault.MarkStatus(ctx, entryID, status)
}

func (v *EnvVaultService) SaveCredential(ctx context.Context, req domain.EnvVaultSaveRequest) (domain.EnvVaultSaveResponse, error) {
	if v == nil || v.store == nil || v.credential == nil {
		return domain.EnvVaultSaveResponse{}, ErrCredentialStoreUnavailable
	}
	variableName := strings.ToUpper(strings.TrimSpace(req.VariableName))
	value := strings.TrimSpace(req.Value)
	if variableName == "" {
		return domain.EnvVaultSaveResponse{}, fmt.Errorf("variable name is required")
	}
	if value == "" {
		return domain.EnvVaultSaveResponse{}, fmt.Errorf("credential value is required")
	}
	if !v.isServiceCredential(variableName) {
		return domain.EnvVaultSaveResponse{}, fmt.Errorf("vault stores service credentials only")
	}
	provider := normalizeVaultProvider(req.Provider, variableName)
	fingerprint := fingerprintCredential(value)
	existing, err := v.store.EnvVaultEntryByProviderFingerprint(ctx, provider, fingerprint)
	if err == nil && existing.ID != 0 {
		return domain.EnvVaultSaveResponse{
			Entry:         existing.EnvVaultEntry,
			NeedsReview:   true,
			ReviewMessage: domain.EnvVaultDuplicateReviewMessage,
		}, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.EnvVaultSaveResponse{}, err
	}

	fragment := fingerprint[:12]
	entry := domain.EnvVaultEntryMetadata{
		EnvVaultEntry: domain.EnvVaultEntry{
			Provider:            provider,
			VariableName:        variableName,
			DisplayName:         fallbackVaultDisplayName(req.DisplayName, provider, variableName, fragment),
			FingerprintFragment: fragment,
			Status:              domain.EnvVaultStatusReady,
		},
		Fingerprint: fingerprint,
	}
	saved, err := v.store.SaveEnvVaultEntry(ctx, entry)
	if err != nil {
		return domain.EnvVaultSaveResponse{}, err
	}
	key := credentialsKeyForEntry(saved.ID)
	if err := v.credential.Put(ctx, key, value); err != nil {
		_ = v.store.DeleteEnvVaultEntry(ctx, saved.ID)
		return domain.EnvVaultSaveResponse{}, fmt.Errorf("store credential: %w", err)
	}
	if err := v.store.UpdateEnvVaultEntryCredentialKey(ctx, saved.ID, key); err != nil {
		_ = v.credential.Delete(ctx, key)
		_ = v.store.DeleteEnvVaultEntry(ctx, saved.ID)
		return domain.EnvVaultSaveResponse{}, err
	}
	saved.CredentialKey = key
	return domain.EnvVaultSaveResponse{Entry: saved.EnvVaultEntry}, nil
}

func (v *EnvVaultService) Approve(ctx context.Context, approval domain.EnvVaultApproval) error {
	if v == nil || v.store == nil {
		return ErrCredentialStoreUnavailable
	}
	approval.RepoPath = strings.TrimSpace(approval.RepoPath)
	approval.TargetRelativePath = strings.TrimSpace(approval.TargetRelativePath)
	approval.VariableName = strings.ToUpper(strings.TrimSpace(approval.VariableName))
	approval.Status = "approved"
	if approval.EntryID == 0 || approval.RepoPath == "" || approval.TargetRelativePath == "" || approval.VariableName == "" {
		return fmt.Errorf("entry, repo, target, and variable are required")
	}
	return v.store.SaveEnvVaultApproval(ctx, approval)
}

func (v *EnvVaultService) MarkStatus(ctx context.Context, entryID int64, status string) error {
	if entryID == 0 || !validVaultStatus(status) {
		return fmt.Errorf("valid entry status is required")
	}
	return v.store.SetEnvVaultEntryStatus(ctx, entryID, status)
}

func (v *EnvVaultService) ApplyApprovedBindings(ctx context.Context, draft *domain.EnvDraft) error {
	if v == nil || v.store == nil || draft == nil {
		return nil
	}
	for targetIndex := range draft.Targets {
		target := &draft.Targets[targetIndex]
		for valueIndex := range target.Values {
			value := &target.Values[valueIndex]
			if strings.TrimSpace(value.Value) != "" || value.ValueClass != domain.EnvValueClassServiceCredential {
				continue
			}
			entries, err := v.store.ApprovedEnvVaultEntries(ctx, draft.RepoPath, target.RelativePath, value.Name)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				continue
			}
			entry := entries[0]
			value.VaultBinding = vaultBinding(entry)
			value.Provenance.Source = domain.EnvValueSourceVault
		}
	}
	return nil
}

func (v *EnvVaultService) ResolveBindings(ctx context.Context, draft *domain.EnvDraft) error {
	if v == nil || v.store == nil || v.credential == nil || draft == nil {
		return nil
	}
	for targetIndex := range draft.Targets {
		target := &draft.Targets[targetIndex]
		for valueIndex := range target.Values {
			value := &target.Values[valueIndex]
			if value.VaultBinding == nil {
				continue
			}
			entry, err := v.store.EnvVaultEntryByID(ctx, value.VaultBinding.EntryID)
			if err != nil {
				return err
			}
			if entry.Status != domain.EnvVaultStatusReady || !strings.EqualFold(entry.VariableName, value.Name) {
				return fmt.Errorf("vault entry for %s is not ready", value.Name)
			}
			approved, err := v.store.ApprovedEnvVaultEntries(ctx, draft.RepoPath, target.RelativePath, value.Name)
			if err != nil {
				return err
			}
			if !containsVaultEntry(approved, entry.ID) {
				return fmt.Errorf("vault entry for %s is not approved", value.Name)
			}
			secret, err := v.credential.Get(ctx, entry.CredentialKey)
			if err != nil {
				return fmt.Errorf("read credential: %w", err)
			}
			value.Value = secret
			value.VaultBinding = nil
			if err := v.store.RecordEnvVaultUse(ctx, domain.EnvVaultUseRecord{
				EntryID:            entry.ID,
				RepoPath:           draft.RepoPath,
				TargetRelativePath: target.RelativePath,
				VariableName:       value.Name,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *EnvVaultService) isServiceCredential(variableName string) bool {
	if decision, ok := v.catalog.Classify(variableName); ok {
		return decision.ValueClass == domain.EnvValueClassServiceCredential
	}
	return false
}

func containsVaultEntry(entries []domain.EnvVaultEntryMetadata, entryID int64) bool {
	for _, entry := range entries {
		if entry.ID == entryID {
			return true
		}
	}
	return false
}

func vaultBinding(entry domain.EnvVaultEntryMetadata) *domain.EnvVaultBinding {
	return &domain.EnvVaultBinding{
		EntryID:             entry.ID,
		Provider:            entry.Provider,
		VariableName:        entry.VariableName,
		DisplayName:         entry.DisplayName,
		FingerprintFragment: entry.FingerprintFragment,
		Status:              entry.Status,
	}
}

func normalizeVaultProvider(provider, variableName string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "" {
		return provider
	}
	name := strings.ToUpper(strings.TrimSpace(variableName))
	parts := strings.Split(name, "_")
	if len(parts) > 1 {
		return strings.ToLower(parts[0])
	}
	return "service"
}

func fallbackVaultDisplayName(displayName, provider, variableName, fragment string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName != "" {
		return displayName
	}
	label := strings.TrimSpace(provider)
	if label == "" {
		label = strings.ToLower(variableName)
	}
	return label + " " + fragment
}

func fingerprintCredential(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func credentialsKeyForEntry(entryID int64) string {
	return fmt.Sprintf("instantrepo-env-vault-entry-%d", entryID)
}

func validVaultStatus(status string) bool {
	switch status {
	case domain.EnvVaultStatusReady, domain.EnvVaultStatusNeedsReview, domain.EnvVaultStatusActionNeeded, domain.EnvVaultStatusInvalid:
		return true
	default:
		return false
	}
}
