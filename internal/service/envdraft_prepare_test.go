package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"instantrepo/internal/domain"
)

func TestPrepareEnvWritesCatalogDraft(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")

	manager := NewEnvDraftManager()
	manager.generateSecret = func() (string, error) {
		return "generated-secret", nil
	}

	result, err := manager.Prepare(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "JWT_SECRET", Secret: true},
				{Name: "OPENAI_API_KEY", Secret: true},
				{
					Name:   "DATABASE_URL",
					Secret: true,
					TopologySignals: []domain.AppTopologySignal{
						{Kind: "data_store", Service: "postgres", Evidence: "docker-compose service postgres", Confidence: 0.9},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if !result.Succeeded || result.StepID != "create-env-file" {
		t.Fatalf("expected successful env preparation, got %+v", result)
	}
	if !strings.Contains(result.Stdout, "Saved .env") {
		t.Fatalf("expected saved target stdout, got %q", result.Stdout)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target env: %v", err)
	}
	content := string(raw)
	databaseURL := "postgres://postgres:postgres@localhost:5432/" + envDatabaseName("", repoPath)
	for _, want := range []string{
		"JWT_SECRET=generated-secret",
		"OPENAI_API_KEY=",
		"DATABASE_URL=" + databaseURL,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in generated env, got:\n%s", want, content)
		}
	}
}

func TestPrepareEnvPreservesExistingValues(t *testing.T) {
	repoPath := t.TempDir()
	targetPath := filepath.Join(repoPath, ".env")

	if err := os.WriteFile(targetPath, []byte("OPENAI_API_KEY=existing-key\n"), 0o644); err != nil {
		t.Fatalf("write target env: %v", err)
	}

	manager := NewEnvDraftManager()
	_, err := manager.Prepare(domain.RepositoryAnalysis{
		RepoPath: repoPath,
		Env: domain.EnvironmentConfig{
			TargetPath: targetPath,
			Variables: []domain.EnvVarRequirement{
				{Name: "OPENAI_API_KEY", Secret: true},
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
