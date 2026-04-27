package analyzer

import (
	"os"
	"path/filepath"
	"testing"
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

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath)
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

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath)
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

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
