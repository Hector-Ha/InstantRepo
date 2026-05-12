package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"instantrepo/internal/domain"
)

func TestBuildAIEnvReviewBundleIsBoundedAndValueFree(t *testing.T) {
	repoPath := t.TempDir()
	writeTestFile(t, filepath.Join(repoPath, "package.json"), `{"name":"secret-app","scripts":{"dev":"vite --host 0.0.0.0","seed":"node scripts/seed.js"}}`)
	writeTestFile(t, filepath.Join(repoPath, "README.md"), "# App\n\n## Setup\n\nCopy `.env.example` then run dev.\n\n```bash\nbun run dev\n```\n")
	writeTestFile(t, filepath.Join(repoPath, ".env"), "OPENAI_API_KEY=sk-live-secret\nAPP_URL=http://localhost:9999\nFULL_ENV_SECRET=do-not-send\n")
	writeTestFile(t, filepath.Join(repoPath, ".env.example"), "OPENAI_API_KEY=\nAPP_URL=http://localhost:3000\nJWT_SECRET=changeme\n")
	writeTestFile(t, filepath.Join(repoPath, "src", "server.ts"), strings.Repeat("const noise = 'full source must not fit';\n", 80)+"console.log(process.env.APP_URL)\n")

	draft := domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{{
			RelativePath:    ".env",
			AbsolutePath:    filepath.Join(repoPath, ".env"),
			OriginalContent: "OPENAI_API_KEY=\nAPP_URL=http://localhost:9999\nFULL_ENV_SECRET=\n",
			Values: []domain.EnvDraftValue{
				{Name: "OPENAI_API_KEY", Value: "", ValueClass: domain.EnvValueClassServiceCredential, Secret: true, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceCatalog}},
				{Name: "APP_URL", Value: "http://localhost:5173", ValueClass: domain.EnvValueClassDevDefault, Confidence: 0.41, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceAllocator}},
				{Name: "JWT_SECRET", Value: "generated-local-secret", ValueClass: domain.EnvValueClassGeneratedLocalSecret, Secret: true, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceGeneratedSecret}},
			},
		}},
	}
	resp := domain.AnalyzeResponse{
		Source: domain.RepoSource{RepoURL: "https://github.com/Owner/Repo.git", Path: repoPath},
		Analysis: domain.RepositoryAnalysis{
			ProjectName: "secret-app",
			ProjectType: "node-project",
			RepoPath:    repoPath,
			Topology: domain.AppTopology{Signals: []domain.AppTopologySignal{{
				Kind: "frontend", Port: 5173, Confidence: 0.42, Evidence: "package.json",
			}}},
			Env: domain.EnvironmentConfig{Variables: []domain.EnvVarRequirement{
				{Name: "OPENAI_API_KEY", Secret: true, Source: ".env.example"},
				{Name: "APP_URL", Source: "code scan", Confidence: 0.41},
				{Name: "JWT_SECRET", Secret: true, Source: ".env.example"},
			}},
		},
	}

	service := NewAIEnvReviewService(nil)
	service.publicChecker = alwaysPublicChecker{}
	bundle, ok, err := service.BuildBundle(context.Background(), resp, draft)
	if err != nil {
		t.Fatalf("BuildBundle returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected low-confidence non-secret candidate")
	}

	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	payload := string(raw)
	for _, forbidden := range []string{
		"sk-live-secret",
		"do-not-send",
		"generated-local-secret",
		"FULL_ENV_SECRET",
		repoPath,
		"full source must not fit",
	} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("bundle leaked %q:\n%s", forbidden, payload)
		}
	}
	if bundle.Repo.URL != "https://github.com/owner/repo" {
		t.Fatalf("expected normalized public repo URL, got %q", bundle.Repo.URL)
	}
	if len(bundle.Targets) != 1 || bundle.Targets[0].RelativePath != ".env" {
		t.Fatalf("expected relative env target only, got %+v", bundle.Targets)
	}
	if len(bundle.Candidates) != 1 || bundle.Candidates[0].VariableName != "APP_URL" {
		t.Fatalf("expected only APP_URL as non-secret candidate, got %+v", bundle.Candidates)
	}
	if bundle.Candidates[0].CurrentValue != "" {
		t.Fatalf("expected bundle to omit candidate value, got %q", bundle.Candidates[0].CurrentValue)
	}
	if len(bundle.EnvNames) != 3 {
		t.Fatalf("expected env names without values, got %+v", bundle.EnvNames)
	}
	if len(bundle.FileTree) == 0 || len(bundle.Manifests) == 0 || len(bundle.SetupExcerpts) == 0 || len(bundle.UsageSnippets) == 0 {
		t.Fatalf("expected bounded review context, got %+v", bundle)
	}
}

func TestValidateEnvPatchRejectsUnsafeOperationsAndProtectedValues(t *testing.T) {
	repoPath := t.TempDir()
	draft := domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{{
			RelativePath: ".env",
			AbsolutePath: filepath.Join(repoPath, ".env"),
			Values: []domain.EnvDraftValue{
				{Name: "OPENAI_API_KEY", ValueClass: domain.EnvValueClassServiceCredential, Secret: true, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceCatalog}},
				{Name: "DATABASE_URL", Value: "postgres://existing-secret@localhost/app", ValueClass: domain.EnvValueClassDevDefault, HasExistingValue: true, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceExistingFile}},
				{Name: "APP_URL", Value: "http://localhost:5173", ValueClass: domain.EnvValueClassDevDefault, Confidence: 0.4, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceDraft}},
				{Name: "JWT_SECRET", Value: "custom-secret", ValueClass: domain.EnvValueClassGeneratedLocalSecret, Secret: true, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceExistingFile}},
				{Name: "VAULTED_TOKEN", ValueClass: domain.EnvValueClassServiceCredential, Secret: true, VaultBinding: &domain.EnvVaultBinding{EntryID: 7}, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceVault}},
			},
		}},
	}
	validator := NewAIEnvReviewService(nil)
	tests := []domain.EnvPatch{
		{Operations: []domain.EnvPatchOperation{{Op: "run_command", Command: "bun install"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "write_file", Path: ".env", Value: "APP_URL=http://evil"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "set_env", TargetRelativePath: "..\\outside.env", VariableName: "APP_URL", Value: "http://localhost:3000"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "set_env", TargetRelativePath: ".env", VariableName: "MISSING", Value: "x"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "set_env", TargetRelativePath: ".env", VariableName: "OPENAI_API_KEY", Value: "sk-new-secret"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "set_env", TargetRelativePath: ".env", VariableName: "DATABASE_URL", Value: "postgres://ai@localhost/app"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "set_env", TargetRelativePath: ".env", VariableName: "APP_URL", Value: "http://localhost:3000"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "set_env", TargetRelativePath: ".env", VariableName: "JWT_SECRET", Value: "ai-secret"}}},
		{Operations: []domain.EnvPatchOperation{{Op: "set_env", TargetRelativePath: ".env", VariableName: "VAULTED_TOKEN", Value: "vault-secret"}}},
	}
	for _, patch := range tests {
		err := validator.ValidatePatch(draft, patch)
		if err == nil {
			t.Fatalf("expected patch to be rejected: %+v", patch)
		}
		if strings.Contains(err.Error(), "postgres://existing-secret") || strings.Contains(err.Error(), "sk-new-secret") || strings.Contains(err.Error(), "vault-secret") {
			t.Fatalf("validation error leaked values: %v", err)
		}
	}
}

func TestApplyEnvPatchUpdatesDraftOnlyWithAIProvenance(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	writeTestFile(t, targetPath, "APP_URL=\nNEXT_PUBLIC_API_URL=changeme\n")
	draft := domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{{
			RelativePath: ".env",
			AbsolutePath: targetPath,
			Values: []domain.EnvDraftValue{
				{Name: "APP_URL", Value: "", ValueClass: domain.EnvValueClassProviderConfig, Confidence: 0.3, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceCatalog}},
				{Name: "NEXT_PUBLIC_API_URL", Value: "changeme", ValueClass: domain.EnvValueClassDevDefault, Confidence: 0.4, Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceAllocator}},
			},
		}},
	}
	patch := domain.EnvPatch{Operations: []domain.EnvPatchOperation{
		{Op: "set_env", TargetRelativePath: ".env", VariableName: "APP_URL", Value: "http://localhost:3000", Confidence: 0.91, Reason: "dev server evidence"},
		{Op: "set_env", TargetRelativePath: ".env", VariableName: "NEXT_PUBLIC_API_URL", Value: "http://localhost:8080", Confidence: 0.88, Reason: "backend evidence"},
	}}

	service := NewAIEnvReviewService(nil)
	if err := service.ApplyPatch(&draft, patch); err != nil {
		t.Fatalf("ApplyPatch returned error: %v", err)
	}

	target := envDraftTarget(t, draft, ".env")
	appURL := envDraftValue(t, target, "APP_URL")
	if appURL.Value != "http://localhost:3000" || appURL.Provenance.Source != domain.EnvValueSourceAIPatch || appURL.Confidence != 0.91 {
		t.Fatalf("expected AI-patched APP_URL, got %+v", appURL)
	}
	apiURL := envDraftValue(t, target, "NEXT_PUBLIC_API_URL")
	if apiURL.Value != "http://localhost:8080" || !hasString(apiURL.Attention, "AI reviewed") {
		t.Fatalf("expected AI-reviewed API URL, got %+v", apiURL)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read env target: %v", err)
	}
	if string(raw) != "APP_URL=\nNEXT_PUBLIC_API_URL=changeme\n" {
		t.Fatalf("expected disk to stay unchanged, got %q", string(raw))
	}
}

func TestGenerateEnvDraftUsesAIReviewOnlyWhenEnabled(t *testing.T) {
	ctx := context.Background()
	repoPath := t.TempDir()
	writeTestFile(t, filepath.Join(repoPath, "package.json"), `{"name":"ai-app","scripts":{"dev":"vite --host 0.0.0.0"}}`)
	writeTestFile(t, filepath.Join(repoPath, "src", "main.ts"), "console.log(process.env.SUPABASE_URL)\n")

	disabledReviewer := &fakeAIEnvReviewer{patch: domain.EnvPatch{Operations: []domain.EnvPatchOperation{{
		Op: "set_env", TargetRelativePath: ".env", VariableName: "SUPABASE_URL", Value: "http://localhost:54321", Confidence: 0.9,
	}}}}
	disabledApp := NewAppServiceWithInstalledRepoStore(nil)
	disabledApp.aiEnvReview = NewAIEnvReviewService(disabledReviewer)
	disabledDraft, err := disabledApp.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft disabled returned error: %v", err)
	}
	if disabledReviewer.calls != 0 {
		t.Fatalf("expected disabled AI reviewer to not be called")
	}
	if got := envDraftValue(t, disabledDraft.Targets[0], "SUPABASE_URL"); got.Provenance.Source == domain.EnvValueSourceAIPatch {
		t.Fatalf("expected deterministic draft without AI provenance, got %+v", got)
	}

	enabledReviewer := &fakeAIEnvReviewer{patch: domain.EnvPatch{Operations: []domain.EnvPatchOperation{{
		Op: "set_env", TargetRelativePath: ".env", VariableName: "SUPABASE_URL", Value: "http://localhost:54321", Confidence: 0.9,
	}}}}
	enabledApp := NewAppServiceWithInstalledRepoStore(nil)
	enabledApp.aiEnvReview = NewAIEnvReviewService(enabledReviewer)
	enabledApp.aiEnvReviewEnabled = true
	enabledDraft, err := enabledApp.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft enabled returned error: %v", err)
	}
	if enabledReviewer.calls != 1 {
		t.Fatalf("expected enabled AI reviewer call, got %d", enabledReviewer.calls)
	}
	if strings.Contains(enabledReviewer.lastPayload, "http://localhost:54321") {
		t.Fatalf("expected reviewer bundle to omit values, got %s", enabledReviewer.lastPayload)
	}
	got := envDraftValue(t, enabledDraft.Targets[0], "SUPABASE_URL")
	if got.Value != "http://localhost:54321" || got.Provenance.Source != domain.EnvValueSourceAIPatch {
		t.Fatalf("expected valid AI patch applied, got %+v", got)
	}

	invalidReviewer := &fakeAIEnvReviewer{patch: domain.EnvPatch{Operations: []domain.EnvPatchOperation{{
		Op: "set_env", TargetRelativePath: ".env", VariableName: "SUPABASE_URL", Value: "sk-should-not-apply", Confidence: 0.9,
	}}}}
	invalidApp := NewAppServiceWithInstalledRepoStore(nil)
	invalidApp.aiEnvReview = NewAIEnvReviewService(invalidReviewer)
	invalidApp.aiEnvReviewEnabled = true
	invalidDraft, err := invalidApp.GenerateEnvDraft(ctx, repoPath)
	if err != nil {
		t.Fatalf("GenerateEnvDraft invalid returned error: %v", err)
	}
	invalidValue := envDraftValue(t, invalidDraft.Targets[0], "SUPABASE_URL")
	if invalidValue.Value == "sk-should-not-apply" || invalidValue.Provenance.Source == domain.EnvValueSourceAIPatch {
		t.Fatalf("expected invalid patch to fail closed, got %+v", invalidValue)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

type alwaysPublicChecker struct{}

func (alwaysPublicChecker) IsPublicRepo(context.Context, string) bool {
	return true
}

func hasString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type fakeAIEnvReviewer struct {
	patch       domain.EnvPatch
	calls       int
	lastPayload string
}

func (f *fakeAIEnvReviewer) ReviewEnv(_ context.Context, bundle domain.AIEnvReviewBundle) (domain.EnvPatch, error) {
	f.calls++
	raw, _ := json.Marshal(bundle)
	f.lastPayload = string(raw)
	return f.patch, nil
}
