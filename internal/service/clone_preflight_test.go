package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"instantrepo/internal/domain"
)

type fakeDiskChecker struct {
	freeBytes uint64
	err       error
}

func (c fakeDiskChecker) FreeBytes(string) (uint64, error) {
	return c.freeBytes, c.err
}

type preflightInstalledRepoStore struct {
	byNormalizedURL map[string]domain.InstalledRepo
	byLocalPath     map[string]domain.InstalledRepo
}

func (s *preflightInstalledRepoStore) SaveInstalledRepo(_ context.Context, repo domain.InstalledRepo) (domain.InstalledRepo, error) {
	return repo, nil
}

func (s *preflightInstalledRepoStore) InstalledRepoByNormalizedURL(_ context.Context, normalizedURL string) (domain.InstalledRepo, error) {
	if repo, ok := s.byNormalizedURL[normalizedURL]; ok {
		return repo, nil
	}
	return domain.InstalledRepo{}, sql.ErrNoRows
}

func (s *preflightInstalledRepoStore) InstalledRepoByLocalPath(_ context.Context, localPath string) (domain.InstalledRepo, error) {
	if repo, ok := s.byLocalPath[localPath]; ok {
		return repo, nil
	}
	return domain.InstalledRepo{}, sql.ErrNoRows
}

func TestClonePreflightAllowsWritableEmptyDestination(t *testing.T) {
	destination := t.TempDir()
	app := NewAppServiceWithInstalledRepoStore(nil)
	app.disk = fakeDiskChecker{freeBytes: 5 * clonePreflightGiB}

	resp, err := app.ClonePreflight(context.Background(), domain.ClonePreflightRequest{
		RepoURL:         " https://github.com/Example/InstantRepo.git ",
		DestinationRoot: destination,
	})
	if err != nil {
		t.Fatalf("ClonePreflight returned error: %v", err)
	}

	wantTarget := filepath.Join(destination, "InstantRepo")
	if resp.NormalizedURL != "https://github.com/example/instantrepo" {
		t.Fatalf("expected normalized URL, got %q", resp.NormalizedURL)
	}
	if resp.TargetPath != wantTarget {
		t.Fatalf("expected target path %q, got %q", wantTarget, resp.TargetPath)
	}
	if !resp.DestinationWritable {
		t.Fatalf("expected destination to be writable")
	}
	if resp.TargetExists {
		t.Fatalf("expected target not to exist")
	}
	if !resp.TargetEmpty {
		t.Fatalf("expected missing target to be treated as empty")
	}
	if len(resp.DuplicateRepos) != 0 {
		t.Fatalf("expected no duplicates, got %d", len(resp.DuplicateRepos))
	}
	if resp.PathConflict {
		t.Fatalf("expected no path conflict")
	}
	if resp.Disk.Status != domain.CloneDiskStatusOK {
		t.Fatalf("expected disk status %q, got %q", domain.CloneDiskStatusOK, resp.Disk.Status)
	}
	if resp.RecommendedAction != domain.CloneActionClone {
		t.Fatalf("expected action %q, got %q", domain.CloneActionClone, resp.RecommendedAction)
	}
	if len(resp.Messages) == 0 {
		t.Fatalf("expected user-facing message")
	}
}

func TestClonePreflightRejectsUnwritableDestination(t *testing.T) {
	root := t.TempDir()
	destinationFile := filepath.Join(root, "not-a-folder")
	if err := os.WriteFile(destinationFile, []byte("not a folder\n"), 0o644); err != nil {
		t.Fatalf("write destination file: %v", err)
	}
	app := NewAppServiceWithInstalledRepoStore(nil)
	app.disk = fakeDiskChecker{freeBytes: 5 * clonePreflightGiB}

	resp, err := app.ClonePreflight(context.Background(), domain.ClonePreflightRequest{
		RepoURL:         "https://github.com/Example/InstantRepo.git",
		DestinationRoot: destinationFile,
	})
	if err != nil {
		t.Fatalf("ClonePreflight returned error: %v", err)
	}

	if resp.DestinationWritable {
		t.Fatalf("expected destination not to be writable")
	}
	if resp.RecommendedAction != domain.CloneActionChooseDifferentFolder {
		t.Fatalf("expected action %q, got %q", domain.CloneActionChooseDifferentFolder, resp.RecommendedAction)
	}
}

func TestClonePreflightAllowsCreatableDestination(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "new-workspace")
	app := NewAppServiceWithInstalledRepoStore(nil)
	app.disk = fakeDiskChecker{freeBytes: 5 * clonePreflightGiB}

	resp, err := app.ClonePreflight(context.Background(), domain.ClonePreflightRequest{
		RepoURL:         "https://github.com/Example/InstantRepo.git",
		DestinationRoot: destination,
	})
	if err != nil {
		t.Fatalf("ClonePreflight returned error: %v", err)
	}

	if !resp.DestinationWritable {
		t.Fatalf("expected creatable destination to be writable")
	}
	if resp.RecommendedAction != domain.CloneActionClone {
		t.Fatalf("expected action %q, got %q", domain.CloneActionClone, resp.RecommendedAction)
	}
	if resp.TargetPath != filepath.Join(destination, "InstantRepo") {
		t.Fatalf("expected target path inside creatable destination, got %q", resp.TargetPath)
	}
}

func TestClonePreflightDetectsDuplicateNormalizedURL(t *testing.T) {
	destination := t.TempDir()
	existingPath := filepath.Join(t.TempDir(), "InstantRepo")
	store := &preflightInstalledRepoStore{
		byNormalizedURL: map[string]domain.InstalledRepo{
			"https://github.com/example/instantrepo": {
				ID:            7,
				RawURL:        "https://github.com/example/InstantRepo.git",
				NormalizedURL: "https://github.com/example/instantrepo",
				LocalPath:     existingPath,
				Status:        domain.InstalledRepoStatusAnalyzed,
			},
		},
	}
	app := NewAppServiceWithInstalledRepoStore(store)
	app.disk = fakeDiskChecker{freeBytes: 5 * clonePreflightGiB}

	resp, err := app.ClonePreflight(context.Background(), domain.ClonePreflightRequest{
		RepoURL:         "https://github.com/Example/InstantRepo.git",
		DestinationRoot: destination,
	})
	if err != nil {
		t.Fatalf("ClonePreflight returned error: %v", err)
	}

	if len(resp.DuplicateRepos) != 1 {
		t.Fatalf("expected one duplicate repo, got %d", len(resp.DuplicateRepos))
	}
	if resp.DuplicateRepos[0].LocalPath != existingPath {
		t.Fatalf("expected duplicate path %q, got %q", existingPath, resp.DuplicateRepos[0].LocalPath)
	}
	if resp.PathConflict {
		t.Fatalf("expected duplicate at different path not to be target path conflict")
	}
	if resp.RecommendedAction != domain.CloneActionOpenExisting {
		t.Fatalf("expected action %q, got %q", domain.CloneActionOpenExisting, resp.RecommendedAction)
	}
}

func TestClonePreflightDetectsNonEmptyTargetPathConflict(t *testing.T) {
	destination := t.TempDir()
	targetPath := filepath.Join(destination, "InstantRepo")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("create target path: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetPath, "README.md"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	app := NewAppServiceWithInstalledRepoStore(nil)
	app.disk = fakeDiskChecker{freeBytes: 5 * clonePreflightGiB}

	resp, err := app.ClonePreflight(context.Background(), domain.ClonePreflightRequest{
		RepoURL:         "https://github.com/Example/InstantRepo.git",
		DestinationRoot: destination,
	})
	if err != nil {
		t.Fatalf("ClonePreflight returned error: %v", err)
	}

	if !resp.TargetExists {
		t.Fatalf("expected target to exist")
	}
	if resp.TargetEmpty {
		t.Fatalf("expected target not to be empty")
	}
	if !resp.PathConflict {
		t.Fatalf("expected path conflict")
	}
	if resp.RecommendedAction != domain.CloneActionChooseDifferentFolder {
		t.Fatalf("expected action %q, got %q", domain.CloneActionChooseDifferentFolder, resp.RecommendedAction)
	}
}

func TestClonePreflightDetectsSavedTargetPathConflict(t *testing.T) {
	destination := t.TempDir()
	targetPath := filepath.Join(destination, "InstantRepo")
	store := &preflightInstalledRepoStore{
		byLocalPath: map[string]domain.InstalledRepo{
			targetPath: {
				ID:            11,
				RawURL:        "https://github.com/example/other.git",
				NormalizedURL: "https://github.com/example/other",
				LocalPath:     targetPath,
				Status:        domain.InstalledRepoStatusAnalyzed,
			},
		},
	}
	app := NewAppServiceWithInstalledRepoStore(store)
	app.disk = fakeDiskChecker{freeBytes: 5 * clonePreflightGiB}

	resp, err := app.ClonePreflight(context.Background(), domain.ClonePreflightRequest{
		RepoURL:         "https://github.com/Example/InstantRepo.git",
		DestinationRoot: destination,
	})
	if err != nil {
		t.Fatalf("ClonePreflight returned error: %v", err)
	}

	if !resp.PathConflict {
		t.Fatalf("expected path conflict")
	}
	if len(resp.PathConflictRepos) != 1 {
		t.Fatalf("expected one path conflict repo, got %d", len(resp.PathConflictRepos))
	}
	if resp.PathConflictRepos[0].ID != 11 {
		t.Fatalf("expected path conflict repo ID 11, got %d", resp.PathConflictRepos[0].ID)
	}
	if resp.RecommendedAction != domain.CloneActionChooseDifferentFolder {
		t.Fatalf("expected action %q, got %q", domain.CloneActionChooseDifferentFolder, resp.RecommendedAction)
	}
}

func TestClonePreflightReportsDiskBlockAndMeasureFallback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		disk       fakeDiskChecker
		wantStatus string
		wantAction string
	}{
		{
			name:       "low free space blocks clone",
			disk:       fakeDiskChecker{freeBytes: clonePreflightGiB / 2},
			wantStatus: domain.CloneDiskStatusBlock,
			wantAction: domain.CloneActionFreeDiskSpace,
		},
		{
			name:       "limited free space warns without blocking",
			disk:       fakeDiskChecker{freeBytes: clonePreflightGiB + 1},
			wantStatus: domain.CloneDiskStatusWarn,
			wantAction: domain.CloneActionCloneWithAttention,
		},
		{
			name:       "measure error warns without blocking",
			disk:       fakeDiskChecker{err: errors.New("statfs failed")},
			wantStatus: domain.CloneDiskStatusWarn,
			wantAction: domain.CloneActionCloneWithAttention,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := NewAppServiceWithInstalledRepoStore(nil)
			app.disk = tc.disk

			resp, err := app.ClonePreflight(context.Background(), domain.ClonePreflightRequest{
				RepoURL:         "https://github.com/Example/InstantRepo.git",
				DestinationRoot: t.TempDir(),
			})
			if err != nil {
				t.Fatalf("ClonePreflight returned error: %v", err)
			}

			if resp.Disk.Status != tc.wantStatus {
				t.Fatalf("expected disk status %q, got %q", tc.wantStatus, resp.Disk.Status)
			}
			if resp.RecommendedAction != tc.wantAction {
				t.Fatalf("expected action %q, got %q", tc.wantAction, resp.RecommendedAction)
			}
			if len(resp.Messages) == 0 {
				t.Fatalf("expected user-facing messages")
			}
		})
	}
}
