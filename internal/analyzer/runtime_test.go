package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"instantrepo/internal/domain"
)

func TestAnalyzeDetectsEnvTemplateAndLocalServices(t *testing.T) {
	repoPath := t.TempDir()

	writeFile(t, filepath.Join(repoPath, "package.json"), `{
  "name": "sample-app",
  "scripts": {
    "dev": "node server.js"
  }
}`)
	writeFile(t, filepath.Join(repoPath, ".env.example"), "DATABASE_URL=\nOPENAI_API_KEY=\n")
	writeFile(t, filepath.Join(repoPath, "docker-compose.yml"), "services:\n  postgres:\n    image: postgres:16\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if analysis.Env.TemplatePath == "" {
		t.Fatalf("expected env template path to be detected")
	}
	if len(analysis.Env.Variables) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(analysis.Env.Variables))
	}

	databaseVar := analysis.Env.Variables[0]
	if databaseVar.Name != "DATABASE_URL" {
		t.Fatalf("expected first env var to be DATABASE_URL, got %q", databaseVar.Name)
	}
	if databaseVar.FillStrategy != "auto_fillable" {
		t.Fatalf("expected DATABASE_URL to be auto_fillable, got %q", databaseVar.FillStrategy)
	}

	openAIVar := analysis.Env.Variables[1]
	if openAIVar.FillStrategy != "user_required" {
		t.Fatalf("expected OPENAI_API_KEY to be user_required, got %q", openAIVar.FillStrategy)
	}
	if len(openAIVar.Instructions) == 0 {
		t.Fatalf("expected OPENAI_API_KEY to include user instructions")
	}

	foundDocker := false
	for _, req := range analysis.Requirements {
		if req.Tool == "docker" {
			foundDocker = true
			break
		}
	}
	if !foundDocker {
		t.Fatalf("expected docker requirement to be added")
	}

	foundPostgres := false
	for _, service := range analysis.Services {
		if service.Name == "postgres" && service.Provisioning == "docker-compose" {
			foundPostgres = true
			break
		}
	}
	if !foundPostgres {
		t.Fatalf("expected postgres docker-compose service to be detected")
	}
}

func TestAnalyzeDetectsExternalMongoRequirement(t *testing.T) {
	repoPath := t.TempDir()

	writeFile(t, filepath.Join(repoPath, "requirements.txt"), "flask\n")
	writeFile(t, filepath.Join(repoPath, ".env.example"), "MONGODB_URI=mongodb+srv://cluster.example.mongodb.net/app\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(analysis.Services) == 0 {
		t.Fatalf("expected external service to be detected")
	}
	if analysis.Services[0].Name != "mongodb" || analysis.Services[0].Provisioning != "user-provided" {
		t.Fatalf("unexpected service detection: %+v", analysis.Services[0])
	}
	if len(analysis.Services[0].Instructions) == 0 {
		t.Fatalf("expected external service instructions to be present")
	}
	if len(analysis.Steps) == 0 || analysis.Steps[len(analysis.Steps)-1].ID != "review-env-values" {
		t.Fatalf("expected manual env review step to be added")
	}
}

func TestAnalyzeUsesNonLocalEnvFilesAsEvidenceOnly(t *testing.T) {
	repoPath := t.TempDir()

	writeFile(t, filepath.Join(repoPath, "package.json"), `{"name":"sample-app","scripts":{"dev":"node server.js"}}`)
	writeFile(t, filepath.Join(repoPath, ".env.production"), "DATABASE_URL=postgres://prod.example/app\nOPENAI_API_KEY=prod-key\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if analysis.Env.TargetPath != "" {
		t.Fatalf("expected no write target for production env file, got %q", analysis.Env.TargetPath)
	}
	if len(analysis.Env.Variables) != 2 {
		t.Fatalf("expected production env names to be evidence, got %+v", analysis.Env.Variables)
	}
	for _, envVar := range analysis.Env.Variables {
		if envVar.TargetDir != "" {
			t.Fatalf("expected %s to have no target dir, got %q", envVar.Name, envVar.TargetDir)
		}
	}
}

func TestAnalyzeTreatsWeirdEnvFileAsLowConfidenceLocalTarget(t *testing.T) {
	repoPath := t.TempDir()

	writeFile(t, filepath.Join(repoPath, "package.json"), `{"name":"sample-app","scripts":{"dev":"node server.js"}}`)
	writeFile(t, filepath.Join(repoPath, ".env.env"), "APP_URL=http://localhost:3000\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if analysis.Env.TargetPath != filepath.Join(repoPath, ".env") {
		t.Fatalf("expected root .env target, got %q", analysis.Env.TargetPath)
	}
	if len(analysis.Env.Variables) != 1 {
		t.Fatalf("expected one env var, got %+v", analysis.Env.Variables)
	}
	envVar := analysis.Env.Variables[0]
	if envVar.TargetDir != repoPath {
		t.Fatalf("expected weird env var to target repo root, got %q", envVar.TargetDir)
	}
	if envVar.Confidence <= 0 || envVar.Confidence >= 0.5 {
		t.Fatalf("expected weird env target confidence between 0 and 0.5, got %v", envVar.Confidence)
	}
}

func TestAnalyzeKeepsTemplateVariablesGroupedByTargetFolder(t *testing.T) {
	repoPath := t.TempDir()
	apiDir := filepath.Join(repoPath, "api")
	webDir := filepath.Join(repoPath, "web")

	writeFile(t, filepath.Join(repoPath, "package.json"), `{"name":"sample-app","scripts":{"dev":"node server.js"}}`)
	writeFile(t, filepath.Join(apiDir, ".env.example"), "API_URL=http://localhost:8080\n")
	writeFile(t, filepath.Join(webDir, ".env.local.example"), "API_URL=http://localhost:5173\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	var apiFound, webFound bool
	for _, envVar := range analysis.Env.Variables {
		if envVar.Name != "API_URL" {
			continue
		}
		switch envVar.TargetDir {
		case apiDir:
			apiFound = true
		case webDir:
			webFound = true
		}
	}
	if !apiFound || !webFound {
		t.Fatalf("expected API_URL in api and web targets, got %+v", analysis.Env.Variables)
	}
}

func TestAnalyzeInfersComponentTargetsFromCodeUsage(t *testing.T) {
	repoPath := t.TempDir()
	serverDir := filepath.Join(repoPath, "server")
	clientDir := filepath.Join(repoPath, "client")

	writeFile(t, filepath.Join(repoPath, "package.json"), `{"name":"sample-app","scripts":{"dev":"node server/index.js"}}`)
	writeFile(t, filepath.Join(serverDir, "index.js"), "const secret = process.env.API_SECRET\n")
	writeFile(t, filepath.Join(clientDir, "main.ts"), "const baseUrl = process.env.VITE_API_URL\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if envVarTargetDir(analysis.Env.Variables, "API_SECRET") != serverDir {
		t.Fatalf("expected API_SECRET target %q, got %+v", serverDir, analysis.Env.Variables)
	}
	if envVarTargetDir(analysis.Env.Variables, "VITE_API_URL") != clientDir {
		t.Fatalf("expected VITE_API_URL target %q, got %+v", clientDir, analysis.Env.Variables)
	}
}

func TestAnalyzeUsesSafeDotenvLoaderPathAsTarget(t *testing.T) {
	repoPath := t.TempDir()
	configDir := filepath.Join(repoPath, "config")

	writeFile(t, filepath.Join(repoPath, "package.json"), `{"name":"sample-app","scripts":{"dev":"node server/index.js"}}`)
	writeFile(t, filepath.Join(repoPath, "server", "index.js"), "require('dotenv').config({ path: './config/.env' })\nconst token = process.env.API_TOKEN\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if envVarTargetDir(analysis.Env.Variables, "API_TOKEN") != configDir {
		t.Fatalf("expected API_TOKEN target %q from dotenv path, got %+v", configDir, analysis.Env.Variables)
	}
}

func TestAnalyzeReportsOutsideRepoDotenvLoaderWithoutOutsideTarget(t *testing.T) {
	repoPath := t.TempDir()
	outsidePath := filepath.Join(t.TempDir(), ".env")

	writeFile(t, filepath.Join(repoPath, "package.json"), `{"name":"sample-app","scripts":{"dev":"node server/index.js"}}`)
	writeFile(t, filepath.Join(repoPath, "server", "index.js"), "require('dotenv').config({ path: '"+filepath.ToSlash(outsidePath)+"' })\nconst token = process.env.API_TOKEN\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if envVarTargetDir(analysis.Env.Variables, "API_TOKEN") != "" {
		t.Fatalf("expected no target dir for outside loader, got %+v", analysis.Env.Variables)
	}
	if analysis.Env.TargetPath != "" {
		t.Fatalf("expected no fallback target path for outside loader, got %q", analysis.Env.TargetPath)
	}
	if len(analysis.Env.SourceFixSuggestions) != 1 {
		t.Fatalf("expected one source fix suggestion, got %+v", analysis.Env.SourceFixSuggestions)
	}
	if len(analysis.Unknowns) == 0 {
		t.Fatalf("expected outside loader attention in unknowns")
	}
}

func TestAnalyzeReportsAmbiguousFolderTopologyAsLowConfidence(t *testing.T) {
	repoPath := t.TempDir()
	webDir := filepath.Join(repoPath, "web")
	apiDir := filepath.Join(repoPath, "api")

	writeFile(t, filepath.Join(webDir, "package.json"), `{"scripts":{"dev":"node index.js"}}`)
	writeFile(t, filepath.Join(webDir, ".env.example"), "APP_URL=\n")
	writeFile(t, filepath.Join(apiDir, "package.json"), `{"scripts":{"dev":"node index.js"}}`)
	writeFile(t, filepath.Join(apiDir, ".env.example"), "PORT=\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath, domain.EnvironmentReport{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	frontend := topologySignal(t, analysis.Topology, "frontend", webDir)
	backend := topologySignal(t, analysis.Topology, "backend", apiDir)
	if frontend.Confidence >= 0.5 || backend.Confidence >= 0.5 {
		t.Fatalf("expected low-confidence folder topology, got frontend %+v backend %+v", frontend, backend)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func envVarTargetDir(vars []domain.EnvVarRequirement, name string) string {
	for _, envVar := range vars {
		if envVar.Name == name {
			return envVar.TargetDir
		}
	}
	return ""
}

func topologySignal(t *testing.T, topology domain.AppTopology, kind, targetDir string) domain.AppTopologySignal {
	t.Helper()
	for _, signal := range topology.Signals {
		if signal.Kind == kind && signal.TargetDir == targetDir {
			return signal
		}
	}
	t.Fatalf("expected topology signal %s %s in %+v", kind, targetDir, topology)
	return domain.AppTopologySignal{}
}
