package command

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateQALocalWorkspaceRemainsIgnored(t *testing.T) {
	repoRoot, err := currentSourceRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	raw, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !hasGitignorePatternLine(string(raw), ".qa-local/") {
		t.Fatalf(".gitignore must keep .qa-local/ ignored")
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available for check-ignore guard")
	}
	check := exec.Command(gitPath, "-C", repoRoot, "check-ignore", "-q", "--", ".qa-local/")
	if output, err := check.CombinedOutput(); err != nil {
		t.Fatalf("git check-ignore .qa-local/ failed: %v\n%s", err, output)
	}

	ls := exec.Command(gitPath, "-C", repoRoot, "ls-files", "--", ".qa-local")
	output, err := ls.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-files .qa-local failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Fatalf(".qa-local files must not be tracked:\n%s", output)
	}
}

func hasGitignorePatternLine(content, pattern string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == pattern {
			return true
		}
	}
	return false
}
