package analyzer

import (
	"path/filepath"
	"testing"
)

func TestAnalyzeMergesReadmeCommandsWithManifestSteps(t *testing.T) {
	repoPath := t.TempDir()

	writeFile(t, filepath.Join(repoPath, "package.json"), `{
  "name": "sample-app",
  "scripts": {
    "dev": "node server.js"
  }
}`)
	writeFile(t, filepath.Join(repoPath, "README.md"), "# Sample\n\n## Installation\n\n```bash\nnpm install\n```\n\n## Development\n\n```bash\nnpm run dev\n```\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(analysis.Steps) != 2 {
		t.Fatalf("expected 2 merged steps, got %d", len(analysis.Steps))
	}

	foundInstall := false
	foundRun := false
	for _, step := range analysis.Steps {
		switch step.Command {
		case "npm install":
			foundInstall = true
			if step.EvidenceSource != "manifest+readme" {
				t.Fatalf("expected npm install to be confirmed by manifest+readme, got %q", step.EvidenceSource)
			}
			if step.Importance != "required" {
				t.Fatalf("expected npm install to be required, got %q", step.Importance)
			}
		case "npm run dev":
			foundRun = true
			if step.EvidenceSource != "manifest+readme" {
				t.Fatalf("expected npm run dev to be confirmed by manifest+readme, got %q", step.EvidenceSource)
			}
			if step.Importance != "recommended" {
				t.Fatalf("expected npm run dev to be recommended, got %q", step.Importance)
			}
		}
	}

	if !foundInstall || !foundRun {
		t.Fatalf("expected both install and run commands to be present")
	}
}

func TestAnalyzeAddsReadmeOnlyCommandsAsSecondaryEvidence(t *testing.T) {
	repoPath := t.TempDir()

	writeFile(t, filepath.Join(repoPath, "README.md"), "# Tool\n\n## Installation\n\n```bash\ngo mod download\n```\n\n## Run\n\n```bash\ngo run .\n```\n")

	analysis, err := NewRepositoryAnalyzer().Analyze(repoPath)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if len(analysis.Steps) == 0 {
		t.Fatalf("expected README-derived steps to be present")
	}

	if analysis.Steps[0].EvidenceSource != "readme" {
		t.Fatalf("expected first README-only step to have readme evidence, got %q", analysis.Steps[0].EvidenceSource)
	}
	if analysis.Steps[0].Importance != "required" {
		t.Fatalf("expected installation step to be classified as required, got %q", analysis.Steps[0].Importance)
	}
}
