package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

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
	UpdateEnvVaultEntryDisplayName(ctx context.Context, entryID int64, displayName string) error
	UpdateEnvVaultEntryCredentialMetadata(ctx context.Context, entryID int64, fingerprint, fingerprintFragment, status string) error
	UpdateEnvVaultEntryCredentialKey(ctx context.Context, entryID int64, credentialKey string) error
	DeleteEnvVaultEntry(ctx context.Context, entryID int64) error
	EnvVaultEntries(ctx context.Context) ([]domain.EnvVaultEntryMetadata, error)
	EnvVaultEntryByID(ctx context.Context, entryID int64) (domain.EnvVaultEntryMetadata, error)
	EnvVaultEntryByProviderFingerprint(ctx context.Context, provider, fingerprint string) (domain.EnvVaultEntryMetadata, error)
	SetEnvVaultEntryStatus(ctx context.Context, entryID int64, status string) error
	SaveEnvVaultApproval(ctx context.Context, approval domain.EnvVaultApproval) error
	EnvVaultApprovals(ctx context.Context, entryID int64) ([]domain.EnvVaultApproval, error)
	RevokeEnvVaultApproval(ctx context.Context, approvalID int64) error
	ApprovedEnvVaultEntries(ctx context.Context, repoPath, targetRelativePath, variableName string) ([]domain.EnvVaultEntryMetadata, error)
	RecordEnvVaultUse(ctx context.Context, record domain.EnvVaultUseRecord) error
	EnvVaultUseRecords(ctx context.Context, entryID int64) ([]domain.EnvVaultUseRecord, error)
	SuppressEnvVaultPrompt(ctx context.Context, suppression domain.EnvVaultPromptSuppression) error
	IsEnvVaultPromptSuppressed(ctx context.Context, repoPath, targetRelativePath, variableName string) (bool, error)
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

func (s *AppService) ListEnvVaultEntries(ctx context.Context) (domain.EnvVaultManagerResponse, error) {
	if s.vault == nil {
		return domain.EnvVaultManagerResponse{}, ErrCredentialStoreUnavailable
	}
	return s.vault.List(ctx)
}

func (s *AppService) RevealEnvVaultEntry(ctx context.Context, req domain.EnvVaultRevealRequest) (domain.EnvVaultRevealResponse, error) {
	if s.vault == nil {
		return domain.EnvVaultRevealResponse{}, ErrCredentialStoreUnavailable
	}
	return s.vault.Reveal(ctx, req)
}

func (s *AppService) UpdateEnvVaultEntry(ctx context.Context, req domain.EnvVaultUpdateRequest) (domain.EnvVaultSaveResponse, error) {
	if s.vault == nil {
		return domain.EnvVaultSaveResponse{}, ErrCredentialStoreUnavailable
	}
	return s.vault.Update(ctx, req)
}

func (s *AppService) RemoveEnvVaultEntry(ctx context.Context, entryID int64) error {
	if s.vault == nil {
		return ErrCredentialStoreUnavailable
	}
	return s.vault.Remove(ctx, entryID)
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

func (s *AppService) RevokeEnvVaultApproval(ctx context.Context, approvalID int64) error {
	if s.vault == nil {
		return ErrCredentialStoreUnavailable
	}
	return s.vault.RevokeApproval(ctx, approvalID)
}

func (s *AppService) SuppressEnvVaultPrompt(ctx context.Context, suppression domain.EnvVaultPromptSuppression) error {
	if s.vault == nil {
		return ErrCredentialStoreUnavailable
	}
	return s.vault.SuppressPrompt(ctx, suppression)
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

func (v *EnvVaultService) List(ctx context.Context) (domain.EnvVaultManagerResponse, error) {
	if v == nil || v.store == nil {
		return domain.EnvVaultManagerResponse{}, ErrCredentialStoreUnavailable
	}
	entries, err := v.store.EnvVaultEntries(ctx)
	if err != nil {
		return domain.EnvVaultManagerResponse{}, err
	}
	resp := domain.EnvVaultManagerResponse{
		Entries: make([]domain.EnvVaultManagerEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		approvals, err := v.store.EnvVaultApprovals(ctx, entry.ID)
		if err != nil {
			return domain.EnvVaultManagerResponse{}, err
		}
		uses, err := v.store.EnvVaultUseRecords(ctx, entry.ID)
		if err != nil {
			return domain.EnvVaultManagerResponse{}, err
		}
		resp.Entries = append(resp.Entries, domain.EnvVaultManagerEntry{
			EnvVaultEntry: entry.EnvVaultEntry,
			Usage:         vaultUsageSummary(uses),
			Approvals:     approvals,
		})
	}
	return resp, nil
}

func (v *EnvVaultService) Update(ctx context.Context, req domain.EnvVaultUpdateRequest) (domain.EnvVaultSaveResponse, error) {
	if v == nil || v.store == nil || v.credential == nil {
		return domain.EnvVaultSaveResponse{}, ErrCredentialStoreUnavailable
	}
	if req.EntryID == 0 {
		return domain.EnvVaultSaveResponse{}, fmt.Errorf("entry is required")
	}
	entry, err := v.store.EnvVaultEntryByID(ctx, req.EntryID)
	if err != nil {
		return domain.EnvVaultSaveResponse{}, err
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName != "" && displayName != entry.DisplayName {
		if err := v.store.UpdateEnvVaultEntryDisplayName(ctx, entry.ID, displayName); err != nil {
			return domain.EnvVaultSaveResponse{}, err
		}
		entry.DisplayName = displayName
	}
	if req.UpdateValue {
		value := strings.TrimSpace(req.Value)
		if value == "" {
			return domain.EnvVaultSaveResponse{}, fmt.Errorf("credential value is required")
		}
		fingerprint := fingerprintCredential(value)
		existing, err := v.store.EnvVaultEntryByProviderFingerprint(ctx, entry.Provider, fingerprint)
		if err == nil && existing.ID != 0 && existing.ID != entry.ID {
			return domain.EnvVaultSaveResponse{
				Entry:         existing.EnvVaultEntry,
				NeedsReview:   true,
				ReviewMessage: domain.EnvVaultDuplicateReviewMessage,
			}, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return domain.EnvVaultSaveResponse{}, err
		}
		oldValue, oldErr := v.credential.Get(ctx, entry.CredentialKey)
		if oldErr != nil {
			return domain.EnvVaultSaveResponse{}, fmt.Errorf("read existing credential: %w", oldErr)
		}
		if err := v.credential.Put(ctx, entry.CredentialKey, value); err != nil {
			return domain.EnvVaultSaveResponse{}, fmt.Errorf("store credential: %w", err)
		}
		fragment := fingerprint[:12]
		if err := v.store.UpdateEnvVaultEntryCredentialMetadata(ctx, entry.ID, fingerprint, fragment, domain.EnvVaultStatusReady); err != nil {
			_ = v.credential.Put(ctx, entry.CredentialKey, oldValue)
			return domain.EnvVaultSaveResponse{}, err
		}
		entry.Fingerprint = fingerprint
		entry.FingerprintFragment = fragment
		entry.Status = domain.EnvVaultStatusReady
	}
	updated, err := v.store.EnvVaultEntryByID(ctx, entry.ID)
	if err != nil {
		return domain.EnvVaultSaveResponse{}, err
	}
	return domain.EnvVaultSaveResponse{Entry: updated.EnvVaultEntry}, nil
}

func (v *EnvVaultService) Remove(ctx context.Context, entryID int64) error {
	if v == nil || v.store == nil || v.credential == nil {
		return ErrCredentialStoreUnavailable
	}
	if entryID == 0 {
		return fmt.Errorf("entry is required")
	}
	entry, err := v.store.EnvVaultEntryByID(ctx, entryID)
	if err != nil {
		return err
	}
	if entry.CredentialKey != "" {
		if err := v.credential.Delete(ctx, entry.CredentialKey); err != nil && !errors.Is(err, ErrCredentialUnavailable) {
			return fmt.Errorf("delete credential: %w", err)
		}
	}
	return v.store.DeleteEnvVaultEntry(ctx, entryID)
}

func (v *EnvVaultService) Reveal(ctx context.Context, req domain.EnvVaultRevealRequest) (domain.EnvVaultRevealResponse, error) {
	if v == nil || v.store == nil || v.credential == nil {
		return domain.EnvVaultRevealResponse{}, ErrCredentialStoreUnavailable
	}
	if req.EntryID == 0 {
		return domain.EnvVaultRevealResponse{}, fmt.Errorf("entry is required")
	}
	if !req.Confirmed {
		return domain.EnvVaultRevealResponse{}, fmt.Errorf("credential reveal requires confirmation")
	}
	entry, err := v.store.EnvVaultEntryByID(ctx, req.EntryID)
	if err != nil {
		return domain.EnvVaultRevealResponse{}, err
	}
	value, err := v.credential.Get(ctx, entry.CredentialKey)
	if err != nil {
		return domain.EnvVaultRevealResponse{}, fmt.Errorf("read credential: %w", err)
	}
	return domain.EnvVaultRevealResponse{
		EntryID:     entry.ID,
		Value:       value,
		RevealUntil: time.Now().UTC().Add(30 * time.Second),
	}, nil
}

func (v *EnvVaultService) Approve(ctx context.Context, approval domain.EnvVaultApproval) error {
	if v == nil || v.store == nil {
		return ErrCredentialStoreUnavailable
	}
	approval.RepoPath = strings.TrimSpace(approval.RepoPath)
	approval.TargetRelativePath = strings.TrimSpace(approval.TargetRelativePath)
	approval.VariableName = strings.ToUpper(strings.TrimSpace(approval.VariableName))
	approval.Status = domain.EnvVaultApprovalStatusApproved
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

func (v *EnvVaultService) RevokeApproval(ctx context.Context, approvalID int64) error {
	if v == nil || v.store == nil {
		return ErrCredentialStoreUnavailable
	}
	if approvalID == 0 {
		return fmt.Errorf("approval is required")
	}
	return v.store.RevokeEnvVaultApproval(ctx, approvalID)
}

func (v *EnvVaultService) ApplyApprovedBindings(ctx context.Context, draft *domain.EnvDraft) error {
	if v == nil || v.store == nil || draft == nil {
		return nil
	}
	for targetIndex := range draft.Targets {
		target := &draft.Targets[targetIndex]
		for valueIndex := range target.Values {
			value := &target.Values[valueIndex]
			if value.HasExistingValue {
				continue
			}
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

func (v *EnvVaultService) ResolveBindings(ctx context.Context, draft *domain.EnvDraft) ([]domain.EnvVaultUseRecord, error) {
	if v == nil || v.store == nil || v.credential == nil || draft == nil {
		return nil, nil
	}
	var pending []domain.EnvVaultUseRecord
	for targetIndex := range draft.Targets {
		target := &draft.Targets[targetIndex]
		for valueIndex := range target.Values {
			value := &target.Values[valueIndex]
			if value.VaultBinding == nil {
				continue
			}
			entry, err := v.store.EnvVaultEntryByID(ctx, value.VaultBinding.EntryID)
			if err != nil {
				return nil, err
			}
			if entry.Status != domain.EnvVaultStatusReady || !strings.EqualFold(entry.VariableName, value.Name) {
				return nil, fmt.Errorf("vault entry for %s is not ready", value.Name)
			}
			approved, err := v.store.ApprovedEnvVaultEntries(ctx, draft.RepoPath, target.RelativePath, value.Name)
			if err != nil {
				return nil, err
			}
			if !containsVaultEntry(approved, entry.ID) {
				return nil, fmt.Errorf("vault entry for %s is not approved", value.Name)
			}
			secret, err := v.credential.Get(ctx, entry.CredentialKey)
			if err != nil {
				return nil, fmt.Errorf("read credential: %w", err)
			}
			value.Value = secret
			value.VaultBinding = nil
			pending = append(pending, domain.EnvVaultUseRecord{
				EntryID:            entry.ID,
				RepoPath:           draft.RepoPath,
				TargetRelativePath: target.RelativePath,
				VariableName:       value.Name,
			})
		}
	}
	return pending, nil
}

func (v *EnvVaultService) RecordResolvedUses(ctx context.Context, records []domain.EnvVaultUseRecord) error {
	if v == nil || v.store == nil {
		return nil
	}
	for _, record := range records {
		if err := v.store.RecordEnvVaultUse(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (v *EnvVaultService) SuppressPrompt(ctx context.Context, suppression domain.EnvVaultPromptSuppression) error {
	if v == nil || v.store == nil {
		return nil
	}
	suppression.RepoPath = strings.TrimSpace(suppression.RepoPath)
	suppression.TargetRelativePath = strings.TrimSpace(suppression.TargetRelativePath)
	suppression.VariableName = strings.ToUpper(strings.TrimSpace(suppression.VariableName))
	if suppression.RepoPath == "" || suppression.TargetRelativePath == "" || suppression.VariableName == "" {
		return fmt.Errorf("repo, target, and variable are required")
	}
	return v.store.SuppressEnvVaultPrompt(ctx, suppression)
}

func (v *EnvVaultService) PromptCandidates(ctx context.Context, draft *domain.EnvDraft, fromVault map[string]bool) ([]domain.EnvVaultPromptCandidate, error) {
	if v == nil || v.store == nil || draft == nil {
		return nil, nil
	}
	var candidates []domain.EnvVaultPromptCandidate
	for targetIndex := range draft.Targets {
		target := &draft.Targets[targetIndex]
		for valueIndex := range target.Values {
			value := &target.Values[valueIndex]
			if value.ValueClass != domain.EnvValueClassServiceCredential {
				continue
			}
			if strings.TrimSpace(value.Value) == "" {
				continue
			}
			key := promptCandidateKey(draft.RepoPath, target.RelativePath, value.Name)
			if fromVault[key] {
				continue
			}
			suppressed, err := v.store.IsEnvVaultPromptSuppressed(ctx, draft.RepoPath, target.RelativePath, value.Name)
			if err != nil {
				return nil, err
			}
			if suppressed {
				continue
			}
			provider := normalizeVaultProvider("", value.Name)
			fingerprint := fingerprintCredential(value.Value)
			existing, err := v.store.EnvVaultEntryByProviderFingerprint(ctx, provider, fingerprint)
			if err == nil && existing.ID != 0 {
				continue
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("check env vault duplicate fingerprint: %w", err)
			}
			candidates = append(candidates, domain.EnvVaultPromptCandidate{
				RepoPath:            draft.RepoPath,
				TargetRelativePath:  target.RelativePath,
				VariableName:        value.Name,
				Provider:            provider,
				FingerprintFragment: fingerprint[:12],
			})
		}
	}
	return candidates, nil
}

func promptCandidateKey(repoPath, targetRelativePath, variableName string) string {
	return strings.Join([]string{repoPath, targetRelativePath, strings.ToUpper(variableName)}, "\x00")
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

func vaultUsageSummary(records []domain.EnvVaultUseRecord) domain.EnvVaultUsageSummary {
	summary := domain.EnvVaultUsageSummary{
		Locations: make([]domain.EnvVaultUsageLocation, 0, len(records)),
	}
	for _, record := range records {
		summary.TotalUseCount += record.UseCount
		summary.Locations = append(summary.Locations, domain.EnvVaultUsageLocation{
			RepoPath:           record.RepoPath,
			TargetRelativePath: record.TargetRelativePath,
			VariableName:       record.VariableName,
			LastUsedAt:         record.UsedAt,
			UseCount:           record.UseCount,
		})
	}
	return summary
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
