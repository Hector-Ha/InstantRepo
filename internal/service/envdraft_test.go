package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"instantrepo/internal/analyzer"
	"instantrepo/internal/domain"
	"instantrepo/internal/envcatalog"
)

var unpaddedBase64URLPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func TestBuildEnvDraftPreservesExistingFileContentAndProvenance(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	existing := "# keep this comment\nDATABASE_URL=postgres://local/app\nUNKNOWN_VAR=keep-me\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "DATABASE_URL", FillStrategy: "user_required"},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	if len(draft.Targets) != 1 {
		t.Fatalf("expected one target, got %d", len(draft.Targets))
	}
	target := draft.Targets[0]
	if target.RelativePath != ".env" {
		t.Fatalf("expected relative path .env, got %q", target.RelativePath)
	}
	if target.OriginalContent != existing {
		t.Fatalf("expected original content to be preserved, got:\n%s", target.OriginalContent)
	}

	value := envDraftValue(t, target, "DATABASE_URL")
	if value.Value != "postgres://local/app" {
		t.Fatalf("expected existing assignment value, got %q", value.Value)
	}
	if value.Provenance.Source != domain.EnvValueSourceExistingFile {
		t.Fatalf("expected existing_file provenance, got %q", value.Provenance.Source)
	}
	if value.Confidence != 1 {
		t.Fatalf("expected confidence 1, got %v", value.Confidence)
	}
}

func TestBuildEnvDraftRedactsUntrackedServiceCredentialFromOriginalContent(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	existing := "SENDGRID_API_KEY=sg-existing-sensitive-value\nCUSTOM_FLAG=keep-me\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "OPENAI_API_KEY", Secret: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	rawJSON, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	if strings.Contains(string(rawJSON), "sg-existing-sensitive-value") {
		t.Fatalf("expected untracked service credential to be redacted from draft JSON, got:\n%s", string(rawJSON))
	}
	target := envDraftTarget(t, draft, ".env")
	if !strings.Contains(target.OriginalContent, "SENDGRID_API_KEY=") {
		t.Fatalf("expected credential assignment shape preserved, got %q", target.OriginalContent)
	}
	if !strings.Contains(target.OriginalContent, "CUSTOM_FLAG=keep-me") {
		t.Fatalf("expected non-credential assignment preserved, got %q", target.OriginalContent)
	}
}

func TestSaveAllPreservesUntrackedExistingServiceCredential(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	existing := "SENDGRID_API_KEY=sg-existing-sensitive-value\nCUSTOM_FLAG=keep-me\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "OPENAI_API_KEY", Secret: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}
	if _, err := manager.SaveAll(draft); err != nil {
		t.Fatalf("SaveAll returned error: %v", err)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target env: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "SENDGRID_API_KEY=sg-existing-sensitive-value") {
		t.Fatalf("expected untracked service credential to be preserved on disk save, got:\n%s", content)
	}
	if !strings.Contains(content, "CUSTOM_FLAG=keep-me") {
		t.Fatalf("expected unknown non-credential value to be preserved, got:\n%s", content)
	}
}

func TestBuildEnvDraftRepresentsTwoTargetFiles(t *testing.T) {
	repoPath := t.TempDir()
	apiDir := filepath.Join(repoPath, "api")
	webDir := filepath.Join(repoPath, "web")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatalf("create api dir: %v", err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("create web dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, ".env"), []byte("API_TOKEN=api-existing\n"), 0o644); err != nil {
		t.Fatalf("write api env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, ".env"), []byte("VITE_API_URL=http://localhost:3000\n"), 0o644); err != nil {
		t.Fatalf("write web env: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "API_TOKEN", TargetDir: apiDir, Secret: true},
				{Name: "VITE_API_URL", TargetDir: webDir},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	if len(draft.Targets) != 2 {
		t.Fatalf("expected two targets, got %d", len(draft.Targets))
	}
	apiTarget := envDraftTarget(t, draft, filepath.Join("api", ".env"))
	webTarget := envDraftTarget(t, draft, filepath.Join("web", ".env"))
	if envDraftValue(t, apiTarget, "API_TOKEN").Value != "api-existing" {
		t.Fatalf("expected API target to keep API_TOKEN")
	}
	if envDraftValue(t, webTarget, "VITE_API_URL").Value != "http://localhost:3000" {
		t.Fatalf("expected web target to keep VITE_API_URL")
	}
}

func TestBuildEnvDraftInfersClientAndServerTargetsFromViteImportMetaUsage(t *testing.T) {
	repoPath := t.TempDir()
	clientDir := filepath.Join(repoPath, "client")
	serverDir := filepath.Join(repoPath, "server")
	writeServiceTestFile(t, filepath.Join(clientDir, "package.json"), `{
  "dependencies": {"@vitejs/plugin-react": "latest", "vite": "latest", "react": "latest"}
}`)
	writeServiceTestFile(t, filepath.Join(clientDir, "src", "main.ts"), "const serverUrl = import.meta.env.VITE_SERVER_URL\n")
	writeServiceTestFile(t, filepath.Join(serverDir, "package.json"), `{
  "dependencies": {"express": "latest"},
  "scripts": {"dev": "node index.js"}
}`)
	writeServiceTestFile(t, filepath.Join(serverDir, "index.js"), "const sessionSecret = process.env.SESSION_SECRET\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	draft, err := NewEnvDraftManager().BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	clientTarget := envDraftTarget(t, draft, filepath.Join("client", ".env"))
	serverTarget := envDraftTarget(t, draft, filepath.Join("server", ".env"))
	if envDraftValue(t, clientTarget, "VITE_SERVER_URL").Name == "" {
		t.Fatalf("expected VITE_SERVER_URL in client target")
	}
	if envDraftValue(t, serverTarget, "SESSION_SECRET").Name == "" {
		t.Fatalf("expected SESSION_SECRET in server target")
	}
}

func TestBuildEnvDraftRejectsEvidenceOnlyVarsWithoutWriteTarget(t *testing.T) {
	repoPath := t.TempDir()

	manager := NewEnvDraftManager()
	_, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "DATABASE_URL", Source: ".env.production"},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected BuildDraft to reject evidence-only vars without a write target")
	}
}

func TestBuildEnvDraftUsesEnvRequirementConfidence(t *testing.T) {
	repoPath := t.TempDir()

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "APP_URL", TargetDir: repoPath, Confidence: 0.45},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, draft.Targets[0], "APP_URL")
	if value.Confidence != 0.45 {
		t.Fatalf("expected confidence 0.45, got %v", value.Confidence)
	}
}

func TestBuildEnvDraftGeneratesLocalSecretFromCatalog(t *testing.T) {
	repoPath := t.TempDir()

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "JWT_SECRET", TargetDir: repoPath, Secret: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, draft.Targets[0], "JWT_SECRET")
	if !unpaddedBase64URLPattern.MatchString(value.Value) {
		t.Fatalf("expected 32-byte unpadded base64url secret, got %q", value.Value)
	}
	if value.ValueClass != domain.EnvValueClassGeneratedLocalSecret {
		t.Fatalf("expected generated local secret class, got %q", value.ValueClass)
	}
	if value.Provenance.Source != domain.EnvValueSourceGeneratedSecret {
		t.Fatalf("expected generated secret provenance, got %q", value.Provenance.Source)
	}
}

func TestBuildEnvDraftKeepsGeneratedLocalSecretStable(t *testing.T) {
	repoPath := t.TempDir()
	generated := 0
	manager := NewEnvDraftManager()
	manager.generateSecret = func() (string, error) {
		generated++
		return fmt.Sprintf("generated-secret-%d", generated), nil
	}
	analysis := domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "JWT_SECRET", TargetDir: repoPath, Secret: true},
			},
		},
	}

	first, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("first BuildDraft returned error: %v", err)
	}
	second, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("second BuildDraft returned error: %v", err)
	}

	firstValue := envDraftValue(t, first.Targets[0], "JWT_SECRET").Value
	secondValue := envDraftValue(t, second.Targets[0], "JWT_SECRET").Value
	if firstValue != "generated-secret-1" || secondValue != firstValue {
		t.Fatalf("expected stable generated secret, got first %q second %q", firstValue, secondValue)
	}
}

func TestBuildEnvDraftReplacesKnownWeakSecretPlaceholder(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	if err := os.WriteFile(targetPath, []byte("JWT_SECRET=changeme\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	manager := NewEnvDraftManager()
	manager.generateSecret = func() (string, error) {
		return "generated-secret", nil
	}

	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "JWT_SECRET", TargetDir: repoPath, Secret: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, draft.Targets[0], "JWT_SECRET")
	if value.Value != "generated-secret" {
		t.Fatalf("expected weak placeholder to be replaced, got %q", value.Value)
	}
	if value.Provenance.Source != domain.EnvValueSourceGeneratedSecret {
		t.Fatalf("expected generated secret provenance, got %q", value.Provenance.Source)
	}
}

func TestBuildEnvDraftPreservesWeakCustomSecretWithAttention(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	if err := os.WriteFile(targetPath, []byte("JWT_SECRET=secret\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "JWT_SECRET", TargetDir: repoPath, Secret: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, draft.Targets[0], "JWT_SECRET")
	if value.Value != "secret" {
		t.Fatalf("expected weak custom value to be preserved, got %q", value.Value)
	}
	if len(value.Attention) == 0 {
		t.Fatalf("expected weak custom secret attention, got %+v", value)
	}
}

func TestBuildEnvDraftLeavesServiceCredentialBlank(t *testing.T) {
	repoPath := t.TempDir()

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "OPENAI_API_KEY", TargetDir: repoPath, Secret: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, draft.Targets[0], "OPENAI_API_KEY")
	if value.Value != "" {
		t.Fatalf("expected service credential to stay blank, got %q", value.Value)
	}
	if value.ValueClass != domain.EnvValueClassServiceCredential {
		t.Fatalf("expected service credential class, got %q", value.ValueClass)
	}
	if len(value.Instructions) == 0 {
		t.Fatalf("expected service credential instructions")
	}
}

func TestBuildEnvDraftLeavesProviderConfigBlank(t *testing.T) {
	repoPath := t.TempDir()

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "SUPABASE_URL", TargetDir: repoPath},
				{Name: "FIREBASE_API_KEY", TargetDir: repoPath},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	for _, name := range []string{"SUPABASE_URL", "FIREBASE_API_KEY"} {
		value := envDraftValue(t, draft.Targets[0], name)
		if value.Value != "" {
			t.Fatalf("expected provider config %s to stay blank, got %q", name, value.Value)
		}
		if value.ValueClass != domain.EnvValueClassProviderConfig {
			t.Fatalf("expected provider config class for %s, got %q", name, value.ValueClass)
		}
	}
}

func TestBuildEnvDraftAppliesDevDefaultFromCatalog(t *testing.T) {
	repoPath := t.TempDir()

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "DATABASE_URL", TargetDir: repoPath, Secret: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, draft.Targets[0], "DATABASE_URL")
	if value.Value != "postgres://postgres:postgres@localhost:5432/postgres" {
		t.Fatalf("expected database dev default, got %q", value.Value)
	}
	if value.ValueClass != domain.EnvValueClassDevDefault {
		t.Fatalf("expected dev default class, got %q", value.ValueClass)
	}
	if value.Provenance.Source != domain.EnvValueSourceCatalog {
		t.Fatalf("expected catalog provenance, got %q", value.Provenance.Source)
	}
}

func TestBuildEnvDraftAllocatesFrontendBackendDefaultsFromTopology(t *testing.T) {
	repoPath := t.TempDir()
	webDir := filepath.Join(repoPath, "web")
	apiDir := filepath.Join(repoPath, "api")
	writeServiceTestFile(t, filepath.Join(webDir, "package.json"), `{
  "dependencies": {"@vitejs/plugin-react": "latest", "vite": "latest", "react": "latest"}
}`)
	writeServiceTestFile(t, filepath.Join(webDir, ".env.example"), "VITE_API_URL=\n")
	writeServiceTestFile(t, filepath.Join(apiDir, "package.json"), `{
  "dependencies": {"express": "latest"},
  "scripts": {"dev": "node server.js"}
}`)
	writeServiceTestFile(t, filepath.Join(apiDir, ".env.example"), "PORT=\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if !hasTopologySignal(analysis.Topology, "frontend", webDir) || !hasTopologySignal(analysis.Topology, "backend", apiDir) {
		t.Fatalf("expected frontend and backend topology signals, got %+v", analysis.Topology)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	apiTarget := envDraftTarget(t, draft, filepath.Join("api", ".env"))
	webTarget := envDraftTarget(t, draft, filepath.Join("web", ".env"))
	if got := envDraftValue(t, apiTarget, "PORT").Value; got != "8080" {
		t.Fatalf("expected backend PORT 8080, got %q", got)
	}
	if got := envDraftValue(t, webTarget, "VITE_API_URL").Value; got != "http://localhost:8080" {
		t.Fatalf("expected frontend API URL to point at backend, got %q", got)
	}
}

func TestBuildEnvDraftFillsSafeClientServerDefaultsAndLeavesProviderKeysBlank(t *testing.T) {
	repoPath := t.TempDir()
	clientDir := filepath.Join(repoPath, "client")
	serverDir := filepath.Join(repoPath, "server")
	writeServiceTestFile(t, filepath.Join(repoPath, "package.json"), `{"name":"client-server-app","private":true}`)
	writeServiceTestFile(t, filepath.Join(clientDir, "package.json"), `{
  "dependencies": {"@vitejs/plugin-react": "latest", "vite": "latest", "react": "latest"}
}`)
	writeServiceTestFile(t, filepath.Join(clientDir, "src", "main.ts"), "const serverUrl = import.meta.env.VITE_SERVER_URL\n")
	writeServiceTestFile(t, filepath.Join(serverDir, "package.json"), `{
  "dependencies": {"express": "latest", "@sendgrid/mail": "latest"},
  "scripts": {"dev": "node index.js"}
}`)
	writeServiceTestFile(t, filepath.Join(serverDir, ".env.example"), "PORT=\nCLIENT_URL=\nMONGODB_URI=\nSENDGRID_API_KEY=\nGROQ_API_KEY=\n")
	writeServiceTestFile(t, filepath.Join(serverDir, "index.js"), strings.Join([]string{
		"const port = process.env.PORT",
		"const clientURL = process.env.CLIENT_URL",
		"const mongoURI = process.env.MONGODB_URI",
		"const sendgridKey = process.env.SENDGRID_API_KEY",
		"const groqKey = process.env.GROQ_API_KEY",
	}, "\n"))
	writeServiceTestFile(t, filepath.Join(repoPath, "docker-compose.yml"), "services:\n  mongodb:\n    image: mongo:7\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	draft, err := NewEnvDraftManager().BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	clientTarget := envDraftTarget(t, draft, filepath.Join("client", ".env"))
	serverTarget := envDraftTarget(t, draft, filepath.Join("server", ".env"))
	viteServerURL := envDraftValue(t, clientTarget, "VITE_SERVER_URL")
	if viteServerURL.Value != "http://localhost:8080" {
		t.Fatalf("expected VITE_SERVER_URL to point at backend, got %q", viteServerURL.Value)
	}
	if viteServerURL.Provenance.Source != domain.EnvValueSourceAllocator || viteServerURL.ValueClass != domain.EnvValueClassDevDefault {
		t.Fatalf("expected VITE_SERVER_URL allocator dev default, got %+v", viteServerURL)
	}

	clientURL := envDraftValue(t, serverTarget, "CLIENT_URL")
	if clientURL.Value != "http://localhost:5173" {
		t.Fatalf("expected CLIENT_URL to point at Vite client, got %q", clientURL.Value)
	}
	if clientURL.Provenance.Source != domain.EnvValueSourceAllocator || clientURL.ValueClass != domain.EnvValueClassDevDefault {
		t.Fatalf("expected CLIENT_URL allocator dev default, got %+v", clientURL)
	}

	mongoURI := envDraftValue(t, serverTarget, "MONGODB_URI")
	if mongoURI.Value != "mongodb://localhost:27017/client_server_app" {
		t.Fatalf("expected local MongoDB URI, got %q", mongoURI.Value)
	}
	if mongoURI.Provenance.Source != domain.EnvValueSourceAllocator || mongoURI.ValueClass != domain.EnvValueClassDevDefault {
		t.Fatalf("expected MONGODB_URI allocator dev default, got %+v", mongoURI)
	}

	for _, name := range []string{"SENDGRID_API_KEY", "GROQ_API_KEY"} {
		value := envDraftValue(t, serverTarget, name)
		if value.Value != "" {
			t.Fatalf("expected provider credential %s to stay blank, got %q", name, value.Value)
		}
		if value.ValueClass != domain.EnvValueClassServiceCredential {
			t.Fatalf("expected provider credential class for %s, got %+v", name, value)
		}
	}
}

func TestBuildEnvDraftAllocatesFullstackAppURLAndPort(t *testing.T) {
	repoPath := t.TempDir()
	writeServiceTestFile(t, filepath.Join(repoPath, "package.json"), `{
  "dependencies": {"next": "latest", "react": "latest"},
  "scripts": {"dev": "next dev"}
}`)
	writeServiceTestFile(t, filepath.Join(repoPath, ".env.example"), "APP_URL=\nPORT=\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	target := envDraftTarget(t, draft, ".env")
	if got := envDraftValue(t, target, "PORT").Value; got != "3000" {
		t.Fatalf("expected fullstack PORT 3000, got %q", got)
	}
	if got := envDraftValue(t, target, "APP_URL").Value; got != "http://localhost:3000" {
		t.Fatalf("expected fullstack app URL, got %q", got)
	}
}

func TestBuildEnvDraftAllocatesDatastoreAndCacheDefaultsFromTopology(t *testing.T) {
	repoPath := t.TempDir()
	writeServiceTestFile(t, filepath.Join(repoPath, "package.json"), `{"name":"stack-app","scripts":{"dev":"node server.js"}}`)
	writeServiceTestFile(t, filepath.Join(repoPath, ".env.example"), "DATABASE_URL=\nREDIS_URL=\n")
	writeServiceTestFile(t, filepath.Join(repoPath, "docker-compose.yml"), "services:\n  postgres:\n    image: postgres:16\n  redis:\n    image: redis:7\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	target := envDraftTarget(t, draft, ".env")
	if got := envDraftValue(t, target, "DATABASE_URL").Value; got != "postgres://postgres:postgres@localhost:5432/stack_app" {
		t.Fatalf("expected repo-scoped postgres URL, got %q", got)
	}
	if got := envDraftValue(t, target, "REDIS_URL").Value; got != "redis://localhost:6379" {
		t.Fatalf("expected redis URL, got %q", got)
	}
	if !hasTopologyService(analysis.Topology, "database", "postgres") || !hasTopologyService(analysis.Topology, "cache", "redis") {
		t.Fatalf("expected datastore/cache topology signals, got %+v", analysis.Topology)
	}
}

func TestBuildEnvDraftUsesNextFreePortForConventionalBusyPort(t *testing.T) {
	repoPath := t.TempDir()
	writeServiceTestFile(t, filepath.Join(repoPath, "package.json"), `{"dependencies":{"next":"latest","react":"latest"}}`)
	writeServiceTestFile(t, filepath.Join(repoPath, ".env.example"), "APP_URL=\nPORT=\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	manager.portAvailable = func(port int) bool { return port != 3000 }
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	target := envDraftTarget(t, draft, ".env")
	if got := envDraftValue(t, target, "PORT").Value; got != "3001" {
		t.Fatalf("expected next free fullstack port, got %q", got)
	}
	if got := envDraftValue(t, target, "APP_URL").Value; got != "http://localhost:3001" {
		t.Fatalf("expected app URL to use next free port, got %q", got)
	}
}

func TestBuildEnvDraftKeepsBusyExactPortEvidenceWithAttention(t *testing.T) {
	repoPath := t.TempDir()
	writeServiceTestFile(t, filepath.Join(repoPath, "package.json"), `{"dependencies":{"next":"latest","react":"latest"}}`)
	writeServiceTestFile(t, filepath.Join(repoPath, ".env.example"), "APP_URL=\nPORT=4000\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	manager.portAvailable = func(port int) bool { return port != 4000 }
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	target := envDraftTarget(t, draft, ".env")
	port := envDraftValue(t, target, "PORT")
	if port.Value != "4000" {
		t.Fatalf("expected exact port evidence to stay 4000, got %q", port.Value)
	}
	if len(port.Attention) == 0 {
		t.Fatalf("expected busy exact port attention, got %+v", port)
	}
	if got := envDraftValue(t, target, "APP_URL").Value; got != "http://localhost:4000" {
		t.Fatalf("expected app URL to use exact port evidence, got %q", got)
	}
}

func TestBuildEnvDraftKeepsAssignedPortStableForSameRepo(t *testing.T) {
	repoPath := t.TempDir()
	writeServiceTestFile(t, filepath.Join(repoPath, "package.json"), `{"dependencies":{"next":"latest","react":"latest"}}`)
	writeServiceTestFile(t, filepath.Join(repoPath, ".env.example"), "PORT=\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	manager.portAvailable = func(port int) bool { return port == 3002 }
	first, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("first BuildDraft returned error: %v", err)
	}
	manager.portAvailable = func(port int) bool { return port == 3005 }
	second, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("second BuildDraft returned error: %v", err)
	}

	firstPort := envDraftValue(t, envDraftTarget(t, first, ".env"), "PORT").Value
	secondPort := envDraftValue(t, envDraftTarget(t, second, ".env"), "PORT").Value
	if firstPort != "3002" || secondPort != firstPort {
		t.Fatalf("expected stable assigned port 3002, got first %q second %q", firstPort, secondPort)
	}
}

func TestBuildEnvDraftAvoidsSameDraftPortCollisions(t *testing.T) {
	repoPath := t.TempDir()
	apiDir := filepath.Join(repoPath, "api")
	adminDir := filepath.Join(repoPath, "admin-api")
	writeServiceTestFile(t, filepath.Join(apiDir, "package.json"), `{"dependencies":{"express":"latest"}}`)
	writeServiceTestFile(t, filepath.Join(apiDir, ".env.example"), "PORT=\n")
	writeServiceTestFile(t, filepath.Join(adminDir, "package.json"), `{"dependencies":{"express":"latest"}}`)
	writeServiceTestFile(t, filepath.Join(adminDir, ".env.example"), "PORT=\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	manager.portAvailable = func(port int) bool { return true }
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	apiPort := envDraftValue(t, envDraftTarget(t, draft, filepath.Join("api", ".env")), "PORT").Value
	adminPort := envDraftValue(t, envDraftTarget(t, draft, filepath.Join("admin-api", ".env")), "PORT").Value
	if apiPort == adminPort {
		t.Fatalf("expected distinct backend ports, got %q and %q", apiPort, adminPort)
	}
}

func TestBuildEnvDraftLeavesCloudDatastoreHintBlankWithLocalSuggestion(t *testing.T) {
	repoPath := t.TempDir()
	writeServiceTestFile(t, filepath.Join(repoPath, "requirements.txt"), "flask\n")
	writeServiceTestFile(t, filepath.Join(repoPath, ".env.example"), "MONGODB_URI=mongodb+srv://cluster.example.mongodb.net/app\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, envDraftTarget(t, draft, ".env"), "MONGODB_URI")
	if value.Value != "" {
		t.Fatalf("expected cloud datastore hint to stay blank, got %q", value.Value)
	}
	if !containsText(value.Attention, "local suggestion") && !containsText(value.Instructions, "mongodb://localhost:27017") {
		t.Fatalf("expected local suggestion metadata, got attention %+v instructions %+v", value.Attention, value.Instructions)
	}
}

func TestBuildEnvDraftLeavesSupabaseHintBlankWithLocalSuggestion(t *testing.T) {
	repoPath := t.TempDir()
	writeServiceTestFile(t, filepath.Join(repoPath, "package.json"), `{"dependencies":{"next":"latest"}}`)
	writeServiceTestFile(t, filepath.Join(repoPath, ".env.example"), "SUPABASE_URL=\n")

	analysis, err := analyzer.NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(analysis)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}

	value := envDraftValue(t, envDraftTarget(t, draft, ".env"), "SUPABASE_URL")
	if value.Value != "" {
		t.Fatalf("expected Supabase provider config to stay blank, got %q", value.Value)
	}
	if !containsText(value.Instructions, "postgres://postgres:postgres@localhost:5432") {
		t.Fatalf("expected local datastore suggestion, got %+v", value.Instructions)
	}
}

func TestBuildEnvDraftRejectsUnsupportedCatalogAction(t *testing.T) {
	repoPath := t.TempDir()
	manager := NewEnvDraftManager()
	manager.catalog = envcatalog.Catalog{
		Version:        "test",
		AllowedActions: []string{envcatalog.ActionLeaveBlank},
		Rules: []envcatalog.Rule{
			{
				Names:  []string{"API_KEY"},
				Action: "run_command",
			},
		},
	}

	_, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "API_KEY", TargetDir: repoPath},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected unsupported catalog action error")
	}
}

func TestSaveAllUpdatesExistingAssignmentsAndAppendsNewValues(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	existing := "# existing header\nAPI_KEY=old-value\nUNKNOWN_VAR=keep-me\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "API_KEY", FillStrategy: "user_required"},
				{Name: "APP_URL", FillStrategy: "auto_fillable", SuggestedValue: "http://localhost:3000"},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}
	draft.Targets[0].Values[0].Value = "new-value"
	draft.Targets[0].Values[1].Value = "http://localhost:3000"

	result, err := manager.SaveAll(draft)
	if err != nil {
		t.Fatalf("SaveAll returned error: %v result %+v", err, result)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read saved env: %v", err)
	}
	content := string(raw)
	expected := "# existing header\nAPI_KEY=new-value\nUNKNOWN_VAR=keep-me\n\n# Added by InstantRepo\nAPP_URL=http://localhost:3000\n"
	if content != expected {
		t.Fatalf("expected sparse rendered env:\n%s\ngot:\n%s", expected, content)
	}
}

func TestSaveAllPreservesExportPrefixOnUpdatedAssignments(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	existing := "export APP_URL=http://localhost:3000\n"
	if err := os.WriteFile(targetPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}

	manager := NewEnvDraftManager()
	draft, err := manager.BuildDraft(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "APP_URL", SuggestedValue: "http://localhost:4000"},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}
	draft.Targets[0].Values[0].Value = "http://localhost:4000"

	if _, err := manager.SaveAll(draft); err != nil {
		t.Fatalf("SaveAll returned error: %v", err)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read saved env: %v", err)
	}
	if got := string(raw); got != "export APP_URL=http://localhost:4000\n" {
		t.Fatalf("expected export prefix preserved, got:\n%s", got)
	}
}

func TestSaveAllValidatesAllTargetsBeforeWriting(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	original := "API_KEY=old-value\n"
	if err := os.WriteFile(targetPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}
	outsidePath := filepath.Join(t.TempDir(), ".env")

	manager := NewEnvDraftManager()
	_, err := manager.SaveAll(domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{
			{
				RelativePath:    ".env",
				AbsolutePath:    targetPath,
				OriginalContent: original,
				Values: []domain.EnvDraftValue{
					{Name: "API_KEY", Value: "new-value"},
				},
			},
			{
				RelativePath: "outside.env",
				AbsolutePath: outsidePath,
				Values: []domain.EnvDraftValue{
					{Name: "SHOULD_NOT_WRITE", Value: "blocked"},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target env: %v", err)
	}
	if string(raw) != original {
		t.Fatalf("expected first target to remain untouched, got:\n%s", string(raw))
	}
	if _, err := os.Stat(outsidePath); !os.IsNotExist(err) {
		t.Fatalf("expected invalid target not to be written, stat err %v", err)
	}
}

func TestSaveAllRollsBackEarlierTargetWhenLaterWriteFails(t *testing.T) {
	repoPath := t.TempDir()
	firstPath := filepath.Join(repoPath, ".env")
	firstOriginal := "API_KEY=old-value\n"
	if err := os.WriteFile(firstPath, []byte(firstOriginal), 0o644); err != nil {
		t.Fatalf("write first env: %v", err)
	}
	blockingDir := filepath.Join(repoPath, "api", ".env")
	if err := os.MkdirAll(blockingDir, 0o755); err != nil {
		t.Fatalf("create blocking target dir: %v", err)
	}

	manager := NewEnvDraftManager()
	_, err := manager.SaveAll(domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{
			{
				RelativePath:    ".env",
				AbsolutePath:    firstPath,
				OriginalContent: firstOriginal,
				Values: []domain.EnvDraftValue{
					{Name: "API_KEY", Value: "new-value"},
				},
			},
			{
				RelativePath: filepath.Join("api", ".env"),
				AbsolutePath: blockingDir,
				Values: []domain.EnvDraftValue{
					{Name: "API_TOKEN", Value: "new-token"},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected second target write to fail")
	}

	raw, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read rolled back env: %v", err)
	}
	if string(raw) != firstOriginal {
		t.Fatalf("expected first target to roll back, got:\n%s", string(raw))
	}
}

func TestSaveAllRetriesTransientWriteFailuresTwice(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	attempts := 0
	manager := &EnvDraftManager{
		writeEnvTarget: func(path string, content []byte) error {
			attempts++
			if attempts < 3 {
				return errors.New("transient write failure")
			}
			return os.WriteFile(path, content, 0o644)
		},
	}

	result, err := manager.SaveAll(domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{
			{
				RelativePath: ".env",
				AbsolutePath: targetPath,
				Values: []domain.EnvDraftValue{
					{Name: "API_KEY", Value: "saved-after-retry"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveAll returned error after transient failures: %v result %+v", err, result)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read saved env: %v", err)
	}
	if string(raw) != "# Added by InstantRepo\nAPI_KEY=saved-after-retry\n" {
		t.Fatalf("expected retried write content, got:\n%s", string(raw))
	}
}

func TestSaveAllFailureIsRedactionSafe(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")
	manager := &EnvDraftManager{
		writeEnvTarget: func(path string, content []byte) error {
			return errors.New("disk refused secret-value")
		},
	}

	result, err := manager.SaveAll(domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{
			{
				RelativePath: ".env",
				AbsolutePath: targetPath,
				Values: []domain.EnvDraftValue{
					{Name: "API_KEY", Value: "secret-value", Secret: true},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected save failure")
	}
	if len(result.Targets) != 1 {
		t.Fatalf("expected one target result, got %+v", result)
	}
	if result.Targets[0].RelativePath != ".env" || result.Targets[0].ErrorKind != "write_failed" {
		t.Fatalf("expected relative target and error kind, got %+v", result.Targets[0])
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("expected error to omit env value, got %q", err.Error())
	}
	if strings.Contains(fmt.Sprintf("%+v", result), "secret-value") {
		t.Fatalf("expected result to omit env value, got %+v", result)
	}
}

func envDraftTarget(t *testing.T, draft domain.EnvDraft, relativePath string) domain.EnvDraftTarget {
	t.Helper()
	for _, target := range draft.Targets {
		if target.RelativePath == relativePath {
			return target
		}
	}
	t.Fatalf("expected target %s in draft %+v", relativePath, draft)
	return domain.EnvDraftTarget{}
}

func envDraftValue(t *testing.T, target domain.EnvDraftTarget, name string) domain.EnvDraftValue {
	t.Helper()
	for _, value := range target.Values {
		if value.Name == name {
			return value
		}
	}
	t.Fatalf("expected value %s in target %+v", name, target)
	return domain.EnvDraftValue{}
}

func hasTopologySignal(topology domain.AppTopology, kind, targetDir string) bool {
	for _, signal := range topology.Signals {
		if signal.Kind == kind && signal.TargetDir == targetDir && signal.Confidence > 0 {
			return true
		}
	}
	return false
}

func hasTopologyService(topology domain.AppTopology, kind, service string) bool {
	for _, signal := range topology.Signals {
		if signal.Kind == kind && signal.Service == service && signal.Confidence > 0 {
			return true
		}
	}
	return false
}

func writeServiceTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func containsText(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
