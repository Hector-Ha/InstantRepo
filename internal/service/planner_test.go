package service

import (
	"os"
	"path/filepath"
	"testing"

	"instantrepo/internal/domain"
)

func TestBuildPlanSafetyScanIgnoresGeneratedFolders(t *testing.T) {
	repoPath := t.TempDir()
	writePlannerTestFile(t, filepath.Join(repoPath, "setup.sh"), "#!/bin/sh\n")
	writePlannerTestFile(t, filepath.Join(repoPath, "node_modules", "pkg", "postinstall.sh"), "#!/bin/sh\n")

	plan := NewPlanner().BuildPlan(domain.RepositoryAnalysis{
		ProjectName: "safety-app",
		ProjectType: "node-project",
		RepoPath:    repoPath,
	}, domain.EnvironmentReport{})

	if len(plan.Safety.Findings) != 1 {
		t.Fatalf("expected only authored risky file finding, got %+v", plan.Safety.Findings)
	}
	if filepath.Base(plan.Safety.Findings[0].FilePath) != "setup.sh" {
		t.Fatalf("expected authored setup.sh finding, got %+v", plan.Safety.Findings[0])
	}
}

func TestBuildPlanSkipsEnvSetupForEvidenceOnlyEnvVars(t *testing.T) {
	repoPath := t.TempDir()
	plan := NewPlanner().BuildPlan(domain.RepositoryAnalysis{
		ProjectName: "evidence-only-env",
		ProjectType: "node-project",
		RepoPath:    repoPath,
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{
				{Name: "OPENAI_API_KEY", Source: ".env.production", FillStrategy: "user_required"},
			},
		},
	}, domain.EnvironmentReport{})

	if hasPlannerStep(plan.Steps, "create-env-file") {
		t.Fatalf("expected no env setup step without a local env target, got %+v", plan.Steps)
	}
}

func hasPlannerStep(steps []domain.ExecutionStep, id string) bool {
	for _, step := range steps {
		if step.ID == id {
			return true
		}
	}
	return false
}

func writePlannerTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
