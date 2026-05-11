package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"instantrepo/internal/domain"
	"instantrepo/internal/store"
)

func TestEnvVaultStoresServiceCredentialWithoutPlaintextMetadata(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "instantrepo.db")
	sqliteStore, err := store.OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	credentials := newFakeCredentialStore()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, credentials)

	resp, err := app.SaveEnvVaultCredential(ctx, domain.EnvVaultSaveRequest{
		Provider:     "openai",
		VariableName: "OPENAI_API_KEY",
		DisplayName:  "work",
		Value:        "sk-test-secret-value",
	})
	if err != nil {
		t.Fatalf("SaveEnvVaultCredential returned error: %v", err)
	}
	if resp.Entry.ID == 0 || resp.Entry.Status != domain.EnvVaultStatusReady {
		t.Fatalf("expected ready entry with ID, got %+v", resp.Entry)
	}
	if got := credentials.values[credentialsKeyForEntry(resp.Entry.ID)]; got != "sk-test-secret-value" {
		t.Fatalf("expected credential store to receive value, got %q", got)
	}

	rawDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read sqlite file: %v", err)
	}
	if strings.Contains(string(rawDB), "sk-test-secret-value") {
		t.Fatalf("expected sqlite file to omit raw credential value")
	}
}

func TestEnvVaultRejectsNonServiceCredential(t *testing.T) {
	app := newEnvVaultTestApp(&recordingInstalledRepoStore{}, nil, newFakeCredentialStore())

	_, err := app.SaveEnvVaultCredential(context.Background(), domain.EnvVaultSaveRequest{
		Provider:     "local",
		VariableName: "JWT_SECRET",
		Value:        "not-for-vault",
	})
	if err == nil {
		t.Fatalf("expected generated local secret to be rejected")
	}
}

func TestEnvVaultDuplicateFingerprintUsesNeutralReview(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, newFakeCredentialStore())

	first, err := app.SaveEnvVaultCredential(ctx, domain.EnvVaultSaveRequest{
		Provider:     "openai",
		VariableName: "OPENAI_API_KEY",
		DisplayName:  "first",
		Value:        "sk-test-secret-value",
	})
	if err != nil {
		t.Fatalf("first SaveEnvVaultCredential returned error: %v", err)
	}
	second, err := app.SaveEnvVaultCredential(ctx, domain.EnvVaultSaveRequest{
		Provider:     "openai",
		VariableName: "OPENAI_API_KEY",
		DisplayName:  "second",
		Value:        "sk-test-secret-value",
	})
	if err != nil {
		t.Fatalf("second SaveEnvVaultCredential returned error: %v", err)
	}
	if !second.NeedsReview || second.ReviewMessage != domain.EnvVaultDuplicateReviewMessage {
		t.Fatalf("expected neutral review, got %+v", second)
	}
	if second.Entry.ID != first.Entry.ID {
		t.Fatalf("expected existing entry metadata, got first %+v second %+v", first.Entry, second.Entry)
	}
}

func TestEnvVaultSupportsMultipleNamedValuesAndFallbackName(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, newFakeCredentialStore())

	named, err := app.SaveEnvVaultCredential(ctx, domain.EnvVaultSaveRequest{
		Provider:     "openai",
		VariableName: "OPENAI_API_KEY",
		DisplayName:  "work",
		Value:        "sk-test-secret-one",
	})
	if err != nil {
		t.Fatalf("named SaveEnvVaultCredential returned error: %v", err)
	}
	unnamed, err := app.SaveEnvVaultCredential(ctx, domain.EnvVaultSaveRequest{
		Provider:     "openai",
		VariableName: "OPENAI_API_KEY",
		Value:        "sk-test-secret-two",
	})
	if err != nil {
		t.Fatalf("unnamed SaveEnvVaultCredential returned error: %v", err)
	}
	if named.Entry.ID == unnamed.Entry.ID {
		t.Fatalf("expected distinct entries")
	}
	if unnamed.Entry.DisplayName == "" || unnamed.Entry.DisplayName == "OPENAI_API_KEY" {
		t.Fatalf("expected generated fallback name, got %q", unnamed.Entry.DisplayName)
	}
}

func TestGenerateEnvDraftBindsApprovedReadyVaultEntryOnlyForBlankServiceCredential(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, newFakeCredentialStore())
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\nSTRIPE_SECRET_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	entry := saveReadyVaultEntry(t, app, "openai", "OPENAI_API_KEY", "sk-test-secret-value")
	if err := app.ApproveEnvVaultEntry(ctx, domain.EnvVaultApproval{
		EntryID:            entry.ID,
		RepoPath:           repoPath,
		TargetRelativePath: ".env",
		VariableName:       "OPENAI_API_KEY",
	}); err != nil {
		t.Fatalf("ApproveEnvVaultEntry returned error: %v", err)
	}

	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}
	value := envDraftTargetValue(t, envDraftTargetByRelativePath(t, draft, ".env"), "OPENAI_API_KEY")
	if value.Value != "" || value.VaultBinding == nil || value.VaultBinding.EntryID != entry.ID {
		t.Fatalf("expected masked vault binding and no raw value, got %+v", value)
	}
	rawJSON, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	if strings.Contains(string(rawJSON), "sk-test-secret-value") {
		t.Fatalf("expected draft JSON to omit credential value")
	}
}

func TestGenerateEnvDraftDoesNotBindVaultOverExistingValueOrUnreadyEntry(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, newFakeCredentialStore())
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\nSTRIPE_SECRET_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("OPENAI_API_KEY=existing\n"), 0o644); err != nil {
		t.Fatalf("write existing env: %v", err)
	}
	ready := saveReadyVaultEntry(t, app, "openai", "OPENAI_API_KEY", "sk-test-secret-value")
	actionNeeded := saveReadyVaultEntry(t, app, "stripe", "STRIPE_SECRET_KEY", "sk-test-stripe-value")
	if err := app.MarkEnvVaultEntryStatus(ctx, actionNeeded.ID, domain.EnvVaultStatusActionNeeded); err != nil {
		t.Fatalf("MarkEnvVaultEntryStatus returned error: %v", err)
	}
	for _, approval := range []domain.EnvVaultApproval{
		{EntryID: ready.ID, RepoPath: repoPath, TargetRelativePath: ".env", VariableName: "OPENAI_API_KEY"},
		{EntryID: actionNeeded.ID, RepoPath: repoPath, TargetRelativePath: ".env", VariableName: "STRIPE_SECRET_KEY"},
	} {
		if err := app.ApproveEnvVaultEntry(ctx, approval); err != nil {
			t.Fatalf("ApproveEnvVaultEntry returned error: %v", err)
		}
	}

	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}
	target := envDraftTargetByRelativePath(t, draft, ".env")
	existing := envDraftTargetValue(t, target, "OPENAI_API_KEY")
	if existing.VaultBinding != nil || !existing.HasExistingValue || existing.Value != "" {
		t.Fatalf("expected existing service credential to be masked and unbound, got %+v", existing)
	}
	unready := envDraftTargetValue(t, target, "STRIPE_SECRET_KEY")
	if unready.VaultBinding != nil {
		t.Fatalf("expected action-needed entry not to bind, got %+v", unready)
	}
}

func TestSaveStructuredEnvDraftRefusesVaultOverwriteOfExistingServiceCredential(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	credentials := newFakeCredentialStore()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, credentials)
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("OPENAI_API_KEY=sk-existing-real\n"), 0o644); err != nil {
		t.Fatalf("write existing env: %v", err)
	}
	entry := saveReadyVaultEntry(t, app, "openai", "OPENAI_API_KEY", "sk-vault-secret-value")
	if err := app.ApproveEnvVaultEntry(ctx, domain.EnvVaultApproval{
		EntryID:            entry.ID,
		RepoPath:           repoPath,
		TargetRelativePath: ".env",
		VariableName:       "OPENAI_API_KEY",
	}); err != nil {
		t.Fatalf("ApproveEnvVaultEntry returned error: %v", err)
	}
	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}
	for ti := range draft.Targets {
		for vi := range draft.Targets[ti].Values {
			if draft.Targets[ti].Values[vi].Name == "OPENAI_API_KEY" {
				draft.Targets[ti].Values[vi].VaultBinding = &domain.EnvVaultBinding{
					EntryID:      entry.ID,
					Provider:     "openai",
					VariableName: "OPENAI_API_KEY",
					Status:       domain.EnvVaultStatusReady,
				}
			}
		}
	}

	if _, err := app.SaveStructuredEnvDraft(ctx, repoPath, draft); err != nil {
		t.Fatalf("SaveStructuredEnvDraft returned error: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if strings.Contains(string(raw), "sk-vault-secret-value") {
		t.Fatalf("expected existing service credential to survive, got:\n%s", string(raw))
	}
	if !strings.Contains(string(raw), "OPENAI_API_KEY=sk-existing-real") {
		t.Fatalf("expected existing value preserved, got:\n%s", string(raw))
	}
	records, err := sqliteStore.EnvVaultUseRecords(ctx, entry.ID)
	if err != nil {
		t.Fatalf("EnvVaultUseRecords returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no vault use record when vault did not apply, got %+v", records)
	}
}

func TestSaveStructuredEnvDraftResolvesVaultRefAtSaveTimeAndRecordsUse(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	credentials := newFakeCredentialStore()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, credentials)
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	entry := saveReadyVaultEntry(t, app, "openai", "OPENAI_API_KEY", "sk-test-secret-value")
	if err := app.ApproveEnvVaultEntry(ctx, domain.EnvVaultApproval{
		EntryID:            entry.ID,
		RepoPath:           repoPath,
		TargetRelativePath: ".env",
		VariableName:       "OPENAI_API_KEY",
	}); err != nil {
		t.Fatalf("ApproveEnvVaultEntry returned error: %v", err)
	}
	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}

	resp, err := app.SaveStructuredEnvDraft(ctx, repoPath, draft)
	if err != nil {
		t.Fatalf("SaveStructuredEnvDraft returned error: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(raw), "OPENAI_API_KEY=sk-test-secret-value") {
		t.Fatalf("expected env file to receive credential at save time, got:\n%s", string(raw))
	}
	if strings.Contains(resp.Result.Stdout, "sk-test-secret-value") || strings.Contains(resp.Result.Stderr, "sk-test-secret-value") {
		t.Fatalf("expected save response to omit credential, got %+v", resp.Result)
	}
	records, err := sqliteStore.EnvVaultUseRecords(ctx, entry.ID)
	if err != nil {
		t.Fatalf("EnvVaultUseRecords returned error: %v", err)
	}
	if len(records) != 1 || records[0].RepoPath != repoPath || records[0].VariableName != "OPENAI_API_KEY" {
		t.Fatalf("expected one value-free use record, got %+v", records)
	}
}

func TestExecuteEnvSetupAppliesApprovedVaultRefAndRecordsUse(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, newFakeCredentialStore())
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "package.json"), []byte(`{"name":"vault-env-setup"}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	entry := saveReadyVaultEntry(t, app, "openai", "OPENAI_API_KEY", "sk-setup-secret-value")
	if err := app.ApproveEnvVaultEntry(ctx, domain.EnvVaultApproval{
		EntryID:            entry.ID,
		RepoPath:           repoPath,
		TargetRelativePath: ".env",
		VariableName:       "OPENAI_API_KEY",
	}); err != nil {
		t.Fatalf("ApproveEnvVaultEntry returned error: %v", err)
	}

	resp, err := app.Execute(ctx, domain.ExecuteRequest{
		LocalPath:    repoPath,
		StepID:       "create-env-file",
		ApproveRisky: true,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !resp.Result.Succeeded {
		t.Fatalf("expected env setup to succeed, got %+v", resp.Result)
	}
	raw, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(raw), "OPENAI_API_KEY=sk-setup-secret-value") {
		t.Fatalf("expected guarded env setup to apply vault credential, got:\n%s", string(raw))
	}
	if strings.Contains(resp.Result.Stdout, "sk-setup-secret-value") || strings.Contains(resp.Result.Stderr, "sk-setup-secret-value") {
		t.Fatalf("expected execute response to omit credential, got %+v", resp.Result)
	}
	records, err := sqliteStore.EnvVaultUseRecords(ctx, entry.ID)
	if err != nil {
		t.Fatalf("EnvVaultUseRecords returned error: %v", err)
	}
	if len(records) != 1 || records[0].RepoPath != repoPath || records[0].VariableName != "OPENAI_API_KEY" {
		t.Fatalf("expected one value-free use record, got %+v", records)
	}
}

func TestGenerateEnvDraftMasksExistingServiceCredentialValuesInDraftJSON(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, newFakeCredentialStore())
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\nPORT=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("OPENAI_API_KEY=sk-real-existing-value\nPORT=3000\n"), 0o644); err != nil {
		t.Fatalf("write existing env: %v", err)
	}

	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}
	target := envDraftTargetByRelativePath(t, draft, ".env")
	credValue := envDraftTargetValue(t, target, "OPENAI_API_KEY")
	if credValue.Value != "" {
		t.Fatalf("expected service-credential existing value masked from draft JSON, got %q", credValue.Value)
	}
	if !credValue.HasExistingValue {
		t.Fatalf("expected hasExistingValue flag for existing service credential")
	}
	portValue := envDraftTargetValue(t, target, "PORT")
	if portValue.Value != "3000" {
		t.Fatalf("expected non-credential value to round-trip, got %q", portValue.Value)
	}
	rawJSON, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	if strings.Contains(string(rawJSON), "sk-real-existing-value") {
		t.Fatalf("expected draft JSON to redact service credential, got:\n%s", string(rawJSON))
	}
}

func TestSaveStructuredEnvDraftSkipsVaultUseRecordWhenSaveFails(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	credentials := newFakeCredentialStore()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, credentials)
	app.envDrafts.writeEnvTarget = func(string, []byte) error {
		return fmt.Errorf("forced write failure")
	}
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	entry := saveReadyVaultEntry(t, app, "openai", "OPENAI_API_KEY", "sk-test-secret-value")
	if err := app.ApproveEnvVaultEntry(ctx, domain.EnvVaultApproval{
		EntryID:            entry.ID,
		RepoPath:           repoPath,
		TargetRelativePath: ".env",
		VariableName:       "OPENAI_API_KEY",
	}); err != nil {
		t.Fatalf("ApproveEnvVaultEntry returned error: %v", err)
	}
	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}

	if _, err := app.SaveStructuredEnvDraft(ctx, repoPath, draft); err == nil {
		t.Fatalf("expected SaveStructuredEnvDraft to fail when write fails")
	}
	records, err := sqliteStore.EnvVaultUseRecords(ctx, entry.ID)
	if err != nil {
		t.Fatalf("EnvVaultUseRecords returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no use record on save failure, got %+v", records)
	}
}

func TestSaveStructuredEnvDraftSucceedsWhenVaultUseRecordFailsAfterSave(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	credentials := newFakeCredentialStore()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, credentials)
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	entry := saveReadyVaultEntry(t, app, "openai", "OPENAI_API_KEY", "sk-record-fail-secret")
	if err := app.ApproveEnvVaultEntry(ctx, domain.EnvVaultApproval{
		EntryID:            entry.ID,
		RepoPath:           repoPath,
		TargetRelativePath: ".env",
		VariableName:       "OPENAI_API_KEY",
	}); err != nil {
		t.Fatalf("ApproveEnvVaultEntry returned error: %v", err)
	}
	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}
	app.vault = NewEnvVaultService(&recordFailingVaultStore{
		EnvVaultStore: sqliteStore,
		err:           fmt.Errorf("forced use-record failure"),
	}, credentials)

	resp, err := app.SaveStructuredEnvDraft(ctx, repoPath, draft)
	if err != nil {
		t.Fatalf("SaveStructuredEnvDraft should report saved env success after metadata failure, got error: %v", err)
	}
	if !resp.Result.Succeeded {
		t.Fatalf("expected successful save response, got %+v", resp.Result)
	}
	raw, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(raw), "OPENAI_API_KEY=sk-record-fail-secret") {
		t.Fatalf("expected env file to be saved, got:\n%s", string(raw))
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(respJSON), "sk-record-fail-secret") {
		t.Fatalf("expected response to omit credential, got:\n%s", string(respJSON))
	}
	records, err := sqliteStore.EnvVaultUseRecords(ctx, entry.ID)
	if err != nil {
		t.Fatalf("EnvVaultUseRecords returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no use record when metadata write fails, got %+v", records)
	}
}

func TestPromptCandidatesReturnsDuplicateLookupErrors(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	vault := NewEnvVaultService(&fingerprintFailingVaultStore{
		EnvVaultStore: sqliteStore,
		err:           fmt.Errorf("forced duplicate lookup failure"),
	}, newFakeCredentialStore())
	draft := domain.EnvDraft{
		RepoPath: t.TempDir(),
		Targets: []domain.EnvDraftTarget{{
			RelativePath: ".env",
			Values: []domain.EnvDraftValue{{
				Name:       "OPENAI_API_KEY",
				Value:      "sk-user-typed-secret",
				ValueClass: domain.EnvValueClassServiceCredential,
			}},
		}},
	}

	_, err := vault.PromptCandidates(ctx, &draft, nil)
	if err == nil {
		t.Fatalf("expected duplicate lookup error")
	}
	if !strings.Contains(err.Error(), "forced duplicate lookup failure") {
		t.Fatalf("expected wrapped duplicate lookup error, got %v", err)
	}
}

func TestSaveStructuredEnvDraftSucceedsWhenVaultPromptMetadataFailsAfterSave(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	credentials := newFakeCredentialStore()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, credentials)
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}
	for ti := range draft.Targets {
		for vi := range draft.Targets[ti].Values {
			if draft.Targets[ti].Values[vi].Name == "OPENAI_API_KEY" {
				draft.Targets[ti].Values[vi].Value = "sk-prompt-fail-secret"
				draft.Targets[ti].Values[vi].Provenance.Source = domain.EnvValueSourceDraft
			}
		}
	}
	app.vault = NewEnvVaultService(&fingerprintFailingVaultStore{
		EnvVaultStore: sqliteStore,
		err:           fmt.Errorf("forced prompt metadata failure"),
	}, credentials)

	resp, err := app.SaveStructuredEnvDraft(ctx, repoPath, draft)
	if err != nil {
		t.Fatalf("SaveStructuredEnvDraft should report saved env success after prompt metadata failure, got error: %v", err)
	}
	if !resp.Result.Succeeded {
		t.Fatalf("expected successful save response, got %+v", resp.Result)
	}
	if len(resp.VaultPromptCandidates) != 0 {
		t.Fatalf("expected no prompt candidates when prompt metadata fails, got %+v", resp.VaultPromptCandidates)
	}
	raw, err := os.ReadFile(filepath.Join(repoPath, ".env"))
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(raw), "OPENAI_API_KEY=sk-prompt-fail-secret") {
		t.Fatalf("expected env file to be saved, got:\n%s", string(raw))
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(respJSON), "sk-prompt-fail-secret") {
		t.Fatalf("expected response to omit credential, got:\n%s", string(respJSON))
	}
}

type recordFailingVaultStore struct {
	EnvVaultStore
	err error
}

func (s *recordFailingVaultStore) RecordEnvVaultUse(context.Context, domain.EnvVaultUseRecord) error {
	return s.err
}

type fingerprintFailingVaultStore struct {
	EnvVaultStore
	err error
}

func (s *fingerprintFailingVaultStore) EnvVaultEntryByProviderFingerprint(context.Context, string, string) (domain.EnvVaultEntryMetadata, error) {
	return domain.EnvVaultEntryMetadata{}, s.err
}

func TestSaveStructuredEnvDraftEmitsValueFreeVaultPromptForManualServiceCredentialAndRespectsSuppression(t *testing.T) {
	ctx := context.Background()
	sqliteStore := openServiceTestSQLiteStore(t)
	defer sqliteStore.Close()
	app := newEnvVaultTestApp(sqliteStore, sqliteStore, newFakeCredentialStore())
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, ".env.example"), []byte("OPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write env template: %v", err)
	}
	draft, err := app.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft returned error: %v", err)
	}
	for ti := range draft.Targets {
		for vi := range draft.Targets[ti].Values {
			if draft.Targets[ti].Values[vi].Name == "OPENAI_API_KEY" {
				draft.Targets[ti].Values[vi].Value = "sk-user-typed-secret"
				draft.Targets[ti].Values[vi].Provenance.Source = domain.EnvValueSourceDraft
			}
		}
	}

	resp, err := app.SaveStructuredEnvDraft(ctx, repoPath, draft)
	if err != nil {
		t.Fatalf("SaveStructuredEnvDraft returned error: %v", err)
	}
	if len(resp.VaultPromptCandidates) != 1 {
		t.Fatalf("expected one vault prompt candidate, got %+v", resp.VaultPromptCandidates)
	}
	candidate := resp.VaultPromptCandidates[0]
	if candidate.VariableName != "OPENAI_API_KEY" || candidate.TargetRelativePath != ".env" || candidate.Provider != "openai" {
		t.Fatalf("expected candidate metadata for typed credential, got %+v", candidate)
	}
	if candidate.FingerprintFragment == "" || len(candidate.FingerprintFragment) > 16 {
		t.Fatalf("expected short fingerprint fragment, got %q", candidate.FingerprintFragment)
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(respJSON), "sk-user-typed-secret") {
		t.Fatalf("expected save response to omit credential value, got:\n%s", string(respJSON))
	}

	if err := app.SuppressEnvVaultPrompt(ctx, domain.EnvVaultPromptSuppression{
		RepoPath:           repoPath,
		TargetRelativePath: ".env",
		VariableName:       "OPENAI_API_KEY",
	}); err != nil {
		t.Fatalf("SuppressEnvVaultPrompt returned error: %v", err)
	}
	resp2, err := app.SaveStructuredEnvDraft(ctx, repoPath, draft)
	if err != nil {
		t.Fatalf("second SaveStructuredEnvDraft returned error: %v", err)
	}
	if len(resp2.VaultPromptCandidates) != 0 {
		t.Fatalf("expected suppression to silence prompt, got %+v", resp2.VaultPromptCandidates)
	}
}

func TestEnvVaultUnavailableCredentialStoreFailsClosed(t *testing.T) {
	app := newEnvVaultTestApp(&recordingInstalledRepoStore{}, nil, unavailableCredentialStore{})

	_, err := app.SaveEnvVaultCredential(context.Background(), domain.EnvVaultSaveRequest{
		Provider:     "openai",
		VariableName: "OPENAI_API_KEY",
		Value:        "sk-test-secret-value",
	})
	if err == nil {
		t.Fatalf("expected unavailable credential store to fail closed")
	}
}

func newEnvVaultTestApp(installed InstalledRepoStore, vault EnvVaultStore, credentials CredentialStore) *AppService {
	app := NewAppServiceWithInstalledRepoStore(installed)
	app.detector = installedRepoTestDetector{}
	app.vault = NewEnvVaultService(vault, credentials)
	return app
}

func openServiceTestSQLiteStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	sqliteStore, err := store.OpenSQLiteStore(filepath.Join(t.TempDir(), "instantrepo.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	return sqliteStore
}

func saveReadyVaultEntry(t *testing.T, app *AppService, provider, variable, value string) domain.EnvVaultEntry {
	t.Helper()
	resp, err := app.SaveEnvVaultCredential(context.Background(), domain.EnvVaultSaveRequest{
		Provider:     provider,
		VariableName: variable,
		Value:        value,
	})
	if err != nil {
		t.Fatalf("SaveEnvVaultCredential returned error: %v", err)
	}
	return resp.Entry
}

type fakeCredentialStore struct {
	values map[string]string
}

func newFakeCredentialStore() *fakeCredentialStore {
	return &fakeCredentialStore{values: map[string]string{}}
}

func (s *fakeCredentialStore) Put(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *fakeCredentialStore) Get(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrCredentialUnavailable
	}
	return value, nil
}

func (s *fakeCredentialStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

type unavailableCredentialStore struct{}

func (unavailableCredentialStore) Put(context.Context, string, string) error {
	return ErrCredentialStoreUnavailable
}

func (unavailableCredentialStore) Get(context.Context, string) (string, error) {
	return "", ErrCredentialStoreUnavailable
}

func (unavailableCredentialStore) Delete(context.Context, string) error {
	return ErrCredentialStoreUnavailable
}
