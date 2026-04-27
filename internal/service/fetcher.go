package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type RepoFetcher struct{}

func NewRepoFetcher() *RepoFetcher {
	return &RepoFetcher{}
}

func (f *RepoFetcher) Clone(ctx context.Context, repoURL string) (string, func(), error) {
	baseDir, err := os.UserCacheDir()
	if err != nil {
		return "", nil, fmt.Errorf("resolve user cache dir: %w", err)
	}

	workRoot := filepath.Join(baseDir, "instantrepo", "workspaces")
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return "", nil, fmt.Errorf("create workspace root: %w", err)
	}

	target := filepath.Join(workRoot, workspaceDirName(repoURL))
	_ = os.RemoveAll(target)

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(target)
		return "", nil, fmt.Errorf("git clone failed: %s", string(output))
	}

	cleanup := func() {}
	return target, cleanup, nil
}

func (f *RepoFetcher) CloneTo(ctx context.Context, repoURL, destinationRoot string) (string, error) {
	if strings.TrimSpace(destinationRoot) == "" {
		return "", fmt.Errorf("destination folder is required")
	}

	if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
		return "", fmt.Errorf("create destination folder: %w", err)
	}

	target := filepath.Join(destinationRoot, repoDirName(repoURL))
	if entries, err := os.ReadDir(target); err == nil && len(entries) > 0 {
		return "", fmt.Errorf("destination already exists and is not empty: %s", target)
	}
	if err := os.RemoveAll(target); err != nil {
		return "", fmt.Errorf("clear destination: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", repoURL, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(target)
		return "", fmt.Errorf("git clone failed: %s", string(output))
	}

	return target, nil
}

func workspaceDirName(repoURL string) string {
	sum := sha1.Sum([]byte(repoURL))
	hash := hex.EncodeToString(sum[:])[:8]

	return repoDirName(repoURL) + "-" + hash
}

func repoDirName(repoURL string) string {
	name := repoURL
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimRight(name, "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if name == "" {
		name = "repo"
	}

	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, name)
}
