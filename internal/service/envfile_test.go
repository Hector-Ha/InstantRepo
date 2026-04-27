package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"instantrepo/internal/domain"
)

func TestPrepareEnvWritesAutoFilledAndPlaceholderValues(t *testing.T) {
	repoPath := t.TempDir()
	templatePath := filepath.Join(repoPath, ".env.example")
	targetPath := filepath.Join(repoPath, ".env")

	if err := os.WriteFile(templatePath, []byte("DATABASE_URL=\nOPENAI_API_KEY=\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	manager := NewEnvFileManager()
	result, err := manager.Prepare(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TemplatePath: templatePath,
			TargetPath:   targetPath,
			Variables: []domain.EnvVarRequirement{
				{
					Name:           "DATABASE_URL",
					FillStrategy:   "auto_fillable",
					SuggestedValue: "postgres://postgres:postgres@localhost:5432/postgres",
					Instructions:   []string{"Start the local postgres service before launching the app."},
				},
				{
					Name:         "OPENAI_API_KEY",
					FillStrategy: "user_required",
					Instructions: []string{"Create an OpenAI API key and paste it here."},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if !result.Succeeded {
		t.Fatalf("expected env preparation to succeed")
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target env: %v", err)
	}
	content := string(raw)

	if !strings.Contains(content, "DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres") {
		t.Fatalf("expected DATABASE_URL to be auto-filled, got:\n%s", content)
	}
	if !strings.Contains(content, "OPENAI_API_KEY=") {
		t.Fatalf("expected OPENAI_API_KEY placeholder to exist, got:\n%s", content)
	}
	if !strings.Contains(content, "Create an OpenAI API key and paste it here.") {
		t.Fatalf("expected instructions to be written for external secrets, got:\n%s", content)
	}
}

func TestPrepareEnvPreservesExistingValues(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")

	if err := os.WriteFile(targetPath, []byte("OPENAI_API_KEY=existing-key\n"), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}

	manager := NewEnvFileManager()
	_, err := manager.Prepare(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{
					Name:         "OPENAI_API_KEY",
					FillStrategy: "user_required",
					Instructions: []string{"Keep this secret."},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target env: %v", err)
	}
	if !strings.Contains(string(raw), "OPENAI_API_KEY=existing-key") {
		t.Fatalf("expected existing value to be preserved, got:\n%s", string(raw))
	}
}
