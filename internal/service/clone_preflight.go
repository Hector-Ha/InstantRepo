package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"instantrepo/internal/domain"
)

const clonePreflightGiB uint64 = 1024 * 1024 * 1024

const (
	clonePreflightBlockFreeBytes = clonePreflightGiB
	clonePreflightWarnFreeBytes  = 2 * clonePreflightGiB
)

type DiskChecker interface {
	FreeBytes(path string) (uint64, error)
}

func (s *AppService) ClonePreflight(ctx context.Context, req domain.ClonePreflightRequest) (domain.ClonePreflightResponse, error) {
	repoURL := strings.TrimSpace(req.RepoURL)
	destinationRoot := strings.TrimSpace(req.DestinationRoot)
	if repoURL == "" {
		return domain.ClonePreflightResponse{}, fmt.Errorf("repo URL is required")
	}
	if destinationRoot == "" {
		return domain.ClonePreflightResponse{}, fmt.Errorf("destination folder is required")
	}

	targetPath, err := CloneTargetPath(repoURL, destinationRoot)
	if err != nil {
		return domain.ClonePreflightResponse{}, fmt.Errorf("resolve destination: %w", err)
	}
	absDestination := filepath.Dir(targetPath)

	targetExists, targetEmpty := inspectTarget(targetPath)
	destinationWritable := canWriteDirectory(absDestination)
	disk := s.cloneDiskStatus(diskCheckPath(absDestination))
	normalizedURL := normalizeRepoURL(repoURL)

	duplicateRepos, err := s.clonePreflightDuplicateRepos(ctx, normalizedURL, targetPath)
	if err != nil {
		return domain.ClonePreflightResponse{}, err
	}
	pathConflictRepos, err := s.clonePreflightPathConflictRepos(ctx, targetPath)
	if err != nil {
		return domain.ClonePreflightResponse{}, err
	}

	resp := domain.ClonePreflightResponse{
		RepoURL:             repoURL,
		NormalizedURL:       normalizedURL,
		DestinationRoot:     absDestination,
		DestinationWritable: destinationWritable,
		TargetPath:          targetPath,
		TargetExists:        targetExists,
		TargetEmpty:         targetEmpty,
		DuplicateRepos:      duplicateRepos,
		PathConflict:        !targetEmpty || len(pathConflictRepos) > 0,
		PathConflictRepos:   pathConflictRepos,
		Disk:                disk,
		RecommendedAction:   domain.CloneActionClone,
	}
	if resp.Disk.Status == domain.CloneDiskStatusWarn {
		resp.RecommendedAction = domain.CloneActionCloneWithAttention
		resp.Messages = append(resp.Messages, domain.ClonePreflightMessage{
			Severity: "attention",
			Text:     resp.Disk.Reason,
		})
	}
	if resp.Disk.Status == domain.CloneDiskStatusBlock {
		resp.RecommendedAction = domain.CloneActionFreeDiskSpace
		resp.Messages = append(resp.Messages, domain.ClonePreflightMessage{
			Severity: "risk",
			Text:     resp.Disk.Reason,
		})
	}
	if len(resp.DuplicateRepos) > 0 {
		resp.RecommendedAction = domain.CloneActionOpenExisting
		resp.Messages = append(resp.Messages, domain.ClonePreflightMessage{
			Severity: "attention",
			Text:     "Repo already exists in Clone History.",
		})
	}
	if !resp.DestinationWritable {
		resp.RecommendedAction = domain.CloneActionChooseDifferentFolder
		resp.Messages = append(resp.Messages, domain.ClonePreflightMessage{
			Severity: "attention",
			Text:     "Destination folder is not writable.",
		})
	}
	if resp.PathConflict {
		resp.RecommendedAction = domain.CloneActionChooseDifferentFolder
		if !resp.TargetEmpty {
			resp.Messages = append(resp.Messages, domain.ClonePreflightMessage{
				Severity: "attention",
				Text:     "Target folder is not empty.",
			})
		}
		if len(resp.PathConflictRepos) > 0 {
			resp.Messages = append(resp.Messages, domain.ClonePreflightMessage{
				Severity: "attention",
				Text:     "Target path is already tracked in Clone History.",
			})
		}
	}
	if len(resp.Messages) == 0 {
		resp.Messages = append(resp.Messages, domain.ClonePreflightMessage{
			Severity: "info",
			Text:     "Ready to clone repository.",
		})
	}

	return resp, nil
}

func CloneTargetPath(repoURL, destinationRoot string) (string, error) {
	absDestination, err := filepath.Abs(strings.TrimSpace(destinationRoot))
	if err != nil {
		return "", err
	}
	return filepath.Join(absDestination, repoDirName(repoURL)), nil
}

func (s *AppService) clonePreflightPathConflictRepos(ctx context.Context, targetPath string) ([]domain.InstalledRepo, error) {
	lookup, ok := s.installedRepos.(InstalledRepoLookup)
	if !ok {
		return nil, nil
	}

	repo, err := lookup.InstalledRepoByLocalPath(ctx, targetPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find path conflict installed repo: %w", err)
	}
	return []domain.InstalledRepo{repo}, nil
}

func (s *AppService) clonePreflightDuplicateRepos(ctx context.Context, normalizedURL, targetPath string) ([]domain.InstalledRepo, error) {
	lookup, ok := s.installedRepos.(InstalledRepoLookup)
	if !ok || normalizedURL == "" {
		return nil, nil
	}

	repo, err := lookup.InstalledRepoByNormalizedURL(ctx, normalizedURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find duplicate installed repo: %w", err)
	}
	if sameLocalPath(repo.LocalPath, targetPath) {
		return nil, nil
	}
	return []domain.InstalledRepo{repo}, nil
}

func (s *AppService) cloneDiskStatus(destinationRoot string) domain.CloneDiskStatus {
	checker := s.disk
	if checker == nil {
		checker = osDiskChecker{}
	}

	freeBytes, err := checker.FreeBytes(destinationRoot)
	if err != nil {
		return domain.CloneDiskStatus{
			Status: domain.CloneDiskStatusWarn,
			Reason: "Disk free space could not be measured.",
		}
	}
	if freeBytes < clonePreflightBlockFreeBytes {
		return domain.CloneDiskStatus{
			Status:    domain.CloneDiskStatusBlock,
			FreeBytes: freeBytes,
			Reason:    "Destination disk has less than 1 GiB free.",
		}
	}
	if freeBytes < clonePreflightWarnFreeBytes {
		return domain.CloneDiskStatus{
			Status:    domain.CloneDiskStatusWarn,
			FreeBytes: freeBytes,
			Reason:    "Destination disk has less than 2 GiB free.",
		}
	}

	return domain.CloneDiskStatus{
		Status:    domain.CloneDiskStatusOK,
		FreeBytes: freeBytes,
	}
}

func inspectTarget(targetPath string) (exists bool, empty bool) {
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, true
		}
		return true, false
	}
	if !info.IsDir() {
		return true, false
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return true, false
	}
	return true, len(entries) == 0
}

func canWriteDirectory(path string) bool {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return false
		}
		return canWriteExistingDirectory(path)
	}
	if !os.IsNotExist(err) {
		return false
	}

	parent, ok := nearestExistingDirectory(filepath.Dir(path))
	if !ok {
		return false
	}
	return canWriteExistingDirectory(parent)
}

func canWriteExistingDirectory(path string) bool {
	file, err := os.CreateTemp(path, ".instantrepo-preflight-*")
	if err != nil {
		return false
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true
}

func diskCheckPath(path string) string {
	if existing, ok := nearestExistingDirectory(path); ok {
		return existing
	}
	return path
}

func nearestExistingDirectory(path string) (string, bool) {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil {
			return current, info.IsDir()
		}
		if !os.IsNotExist(err) {
			return "", false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func sameLocalPath(a, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanA, cleanB)
	}
	return cleanA == cleanB
}
