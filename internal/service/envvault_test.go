package service

import (
	"context"
	"encoding/json"
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
	if existing.Value != "existing" || existing.VaultBinding != nil {
		t.Fatalf("expected existing value to stay unbound, got %+v", existing)
	}
	unready := envDraftTargetValue(t, target, "STRIPE_SECRET_KEY")
	if unready.VaultBinding != nil {
		t.Fatalf("expected action-needed entry not to bind, got %+v", unready)
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
