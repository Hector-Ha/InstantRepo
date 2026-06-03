package service

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"instantrepo/internal/domain"
)

type installedRepoManagerStore interface {
	InstalledRepos(ctx context.Context) ([]domain.InstalledRepo, error)
	InstalledRepoByID(ctx context.Context, id int64) (domain.InstalledRepo, error)
	SetupSessionsByInstalledRepoID(ctx context.Context, installedRepoID int64) ([]domain.SetupSession, error)
}

func (s *AppService) ListInstalledRepos(ctx context.Context) (domain.InstalledRepoManagerResponse, error) {
	managerStore, ok := s.installedRepos.(installedRepoManagerStore)
	if !ok {
		return domain.InstalledRepoManagerResponse{}, fmt.Errorf("installed repo manager store is not configured")
	}

	repos, err := managerStore.InstalledRepos(ctx)
	if err != nil {
		return domain.InstalledRepoManagerResponse{}, fmt.Errorf("list installed repos: %w", err)
	}

	summaries := make([]domain.InstalledRepoSummary, 0, len(repos))
	for _, repo := range repos {
		sessions, err := managerStore.SetupSessionsByInstalledRepoID(ctx, repo.ID)
		if err != nil {
			return domain.InstalledRepoManagerResponse{}, fmt.Errorf("list setup sessions for installed repo %d: %w", repo.ID, err)
		}
		summaries = append(summaries, installedRepoSummary(repo, sessions))
	}

	return domain.InstalledRepoManagerResponse{Repos: summaries}, nil
}

func (s *AppService) InstalledRepoDetails(ctx context.Context, installedRepoID int64) (domain.InstalledRepoDetailsResponse, error) {
	managerStore, ok := s.installedRepos.(installedRepoManagerStore)
	if !ok {
		return domain.InstalledRepoDetailsResponse{}, fmt.Errorf("installed repo manager store is not configured")
	}

	repo, err := managerStore.InstalledRepoByID(ctx, installedRepoID)
	if err != nil {
		return domain.InstalledRepoDetailsResponse{}, fmt.Errorf("find installed repo details: %w", err)
	}
	sessions, err := managerStore.SetupSessionsByInstalledRepoID(ctx, repo.ID)
	if err != nil {
		return domain.InstalledRepoDetailsResponse{}, fmt.Errorf("list setup sessions for installed repo %d: %w", repo.ID, err)
	}

	return domain.InstalledRepoDetailsResponse{
		Repo:          installedRepoSummary(repo, sessions),
		SetupSessions: setupSessionSummaries(sessions),
	}, nil
}

func installedRepoSummary(repo domain.InstalledRepo, sessions []domain.SetupSession) domain.InstalledRepoSummary {
	lastSetupAt := latestSetupAt(sessions)
	lastActivityAt := repo.LastAnalyzedAt
	if lastSetupAt.After(lastActivityAt) {
		lastActivityAt = lastSetupAt
	}

	return domain.InstalledRepoSummary{
		ID:              repo.ID,
		ProjectName:     installedRepoProjectName(repo),
		LocalPath:       repo.LocalPath,
		LocalPathExists: installedRepoPathExists(repo),
		Status:          repo.Status,
		LastAnalyzedAt:  repo.LastAnalyzedAt,
		LastSetupAt:     lastSetupAt,
		LastActivityAt:  lastActivityAt,
	}
}

func setupSessionSummaries(sessions []domain.SetupSession) []domain.SetupSessionSummary {
	summaries := make([]domain.SetupSessionSummary, 0, len(sessions))
	for _, session := range sessions {
		summaries = append(summaries, domain.SetupSessionSummary{
			ID:              session.ID,
			InstalledRepoID: session.InstalledRepoID,
			RepoPath:        session.RepoPath,
			Status:          session.Status,
			CreatedAt:       session.CreatedAt,
			UpdatedAt:       session.UpdatedAt,
		})
	}
	return summaries
}

func latestSetupAt(sessions []domain.SetupSession) time.Time {
	var latest time.Time
	for _, session := range sessions {
		candidate := session.UpdatedAt
		if candidate.IsZero() {
			candidate = session.CreatedAt
		}
		if candidate.After(latest) {
			latest = candidate
		}
	}
	return latest
}

func installedRepoProjectName(repo domain.InstalledRepo) string {
	for _, raw := range []string{repo.NormalizedURL, repo.RawURL} {
		if name := projectNameFromRepoURL(raw); name != "" {
			return name
		}
	}
	if name := strings.TrimSpace(filepath.Base(repo.LocalPath)); name != "" && name != "." {
		return name
	}
	return "Installed Repo"
}

func projectNameFromRepoURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	name := strings.TrimSuffix(path.Base(parsed.Path), ".git")
	if name == "." || name == "/" {
		return ""
	}
	return name
}
