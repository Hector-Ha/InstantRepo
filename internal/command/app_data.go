package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"instantrepo/internal/store"
)

func prepareAppDataDir(appDataDir, targetRepoPath string) error {
	cleanAppData, err := validateAppDataDirPath(appDataDir, targetRepoPath)
	if err != nil {
		return err
	}
	if cleanAppData == "" {
		return nil
	}
	if err := os.MkdirAll(cleanAppData, 0o700); err != nil {
		return fmt.Errorf("create app data dir: %w", err)
	}
	return nil
}

func validateAppDataDir(appDataDir, targetRepoPath string) error {
	_, err := validateAppDataDirPath(appDataDir, targetRepoPath)
	return err
}

func validateAppDataDirPath(appDataDir, targetRepoPath string) (string, error) {
	appDataDir = strings.TrimSpace(appDataDir)
	if appDataDir == "" {
		return "", nil
	}
	if !filepath.IsAbs(appDataDir) {
		return "", fmt.Errorf("app data dir must be absolute")
	}
	cleanAppData, err := filepath.Abs(appDataDir)
	if err != nil {
		return "", fmt.Errorf("resolve app data dir: %w", err)
	}
	cleanAppData = filepath.Clean(cleanAppData)
	if filepath.Dir(cleanAppData) == cleanAppData {
		return "", fmt.Errorf("app data dir must not be filesystem root")
	}
	if volume := filepath.VolumeName(cleanAppData); volume != "" && strings.EqualFold(cleanAppData, volume+string(os.PathSeparator)) {
		return "", fmt.Errorf("app data dir must not be drive root")
	}
	homeDir, err := os.UserHomeDir()
	if err == nil && samePath(cleanAppData, homeDir) {
		return "", fmt.Errorf("app data dir must not be home dir")
	}
	repoRoot, err := currentSourceRepoRoot()
	if err == nil && samePath(cleanAppData, repoRoot) {
		return "", fmt.Errorf("app data dir must not be repo root")
	}
	if strings.TrimSpace(targetRepoPath) != "" {
		repoAbs, err := filepath.Abs(targetRepoPath)
		if err != nil {
			return "", fmt.Errorf("resolve target repo path: %w", err)
		}
		repoAbs = filepath.Clean(repoAbs)
		if samePath(cleanAppData, repoAbs) || isPathInside(cleanAppData, repoAbs) {
			return "", fmt.Errorf("app data dir must not be target repo or inside target repo")
		}
	}
	if _, err := store.DatabasePathForAppDataDir(cleanAppData); err != nil {
		return "", err
	}
	return cleanAppData, nil
}

func cleanAppDataDir(appDataDir string) string {
	if strings.TrimSpace(appDataDir) == "" {
		return ""
	}
	abs, err := filepath.Abs(appDataDir)
	if err != nil {
		return filepath.Clean(appDataDir)
	}
	return filepath.Clean(abs)
}

func currentSourceRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Clean(wd)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA == nil {
		a = aa
	}
	if errB == nil {
		b = bb
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func isPathInside(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
