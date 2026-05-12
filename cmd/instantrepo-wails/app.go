package main

import (
	"context"

	"instantrepo/internal/domain"
	"instantrepo/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	service *service.AppService
}

func NewApp() *App {
	return &App{
		service: service.NewAppService(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) OpenDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose Workspace Folder",
	})
}

func (a *App) AnalyzeRepository(repoURL, localPath string) (domain.AnalyzeResponse, error) {
	return a.service.Analyze(a.appContext(), domain.AnalyzeRequest{
		RepoURL:   repoURL,
		LocalPath: localPath,
	})
}

func (a *App) ImportRepository(repoURL, destinationRoot string) (domain.AnalyzeResponse, error) {
	return a.service.ImportRepository(a.appContext(), repoURL, destinationRoot)
}

func (a *App) ClonePreflight(repoURL, destinationRoot string) (domain.ClonePreflightResponse, error) {
	return a.service.ClonePreflight(a.appContext(), domain.ClonePreflightRequest{
		RepoURL:         repoURL,
		DestinationRoot: destinationRoot,
	})
}

func (a *App) ListInstalledRepos() (domain.InstalledRepoManagerResponse, error) {
	return a.service.ListInstalledRepos(a.appContext())
}

func (a *App) InstalledRepoDetails(installedRepoID int64) (domain.InstalledRepoDetailsResponse, error) {
	return a.service.InstalledRepoDetails(a.appContext(), installedRepoID)
}

func (a *App) GenerateEnvDraft(localPath string) (domain.EnvDraft, error) {
	return a.service.GenerateEnvDraft(a.appContext(), localPath)
}

func (a *App) SaveEnvDraft(localPath string, draft domain.EnvDraft) (domain.ExecuteResponse, error) {
	return a.service.SaveStructuredEnvDraft(a.appContext(), localPath, draft)
}

func (a *App) SaveEnvVaultCredential(req domain.EnvVaultSaveRequest) (domain.EnvVaultSaveResponse, error) {
	return a.service.SaveEnvVaultCredential(a.appContext(), req)
}

func (a *App) ListEnvVaultEntries() (domain.EnvVaultManagerResponse, error) {
	return a.service.ListEnvVaultEntries(a.appContext())
}

func (a *App) RevealEnvVaultEntry(req domain.EnvVaultRevealRequest) (domain.EnvVaultRevealResponse, error) {
	return a.service.RevealEnvVaultEntry(a.appContext(), req)
}

func (a *App) UpdateEnvVaultEntry(req domain.EnvVaultUpdateRequest) (domain.EnvVaultSaveResponse, error) {
	return a.service.UpdateEnvVaultEntry(a.appContext(), req)
}

func (a *App) RemoveEnvVaultEntry(entryID int64) error {
	return a.service.RemoveEnvVaultEntry(a.appContext(), entryID)
}

func (a *App) EnvContributionSettings() (domain.EnvContributionSettingsResponse, error) {
	return a.service.EnvContributionSettings(a.appContext())
}

func (a *App) SaveEnvContributionSettings(settings domain.EnvContributionSettings) (domain.EnvContributionSettingsResponse, error) {
	return a.service.SaveEnvContributionSettings(a.appContext(), settings)
}

func (a *App) RecordEnvContributionConsent(publicEnabled bool) (domain.EnvContributionSettingsResponse, error) {
	return a.service.RecordEnvContributionConsent(a.appContext(), publicEnabled)
}

func (a *App) ClearEnvContributionQueue() (domain.EnvContributionSettingsResponse, error) {
	return a.service.ClearEnvContributionQueue(a.appContext())
}

func (a *App) AIEnvReviewSettings() (domain.AIEnvReviewSettings, error) {
	return a.service.AIEnvReviewSettings(a.appContext())
}

func (a *App) SaveAIEnvReviewSettings(settings domain.AIEnvReviewSettings) (domain.AIEnvReviewSettings, error) {
	return a.service.SaveAIEnvReviewSettings(a.appContext(), settings)
}

func (a *App) ApproveEnvVaultEntry(approval domain.EnvVaultApproval) error {
	return a.service.ApproveEnvVaultEntry(a.appContext(), approval)
}

func (a *App) MarkEnvVaultEntryStatus(entryID int64, status string) error {
	return a.service.MarkEnvVaultEntryStatus(a.appContext(), entryID, status)
}

func (a *App) RevokeEnvVaultApproval(approvalID int64) error {
	return a.service.RevokeEnvVaultApproval(a.appContext(), approvalID)
}

func (a *App) SuppressEnvVaultPrompt(suppression domain.EnvVaultPromptSuppression) error {
	return a.service.SuppressEnvVaultPrompt(a.appContext(), suppression)
}

func (a *App) SaveEnvFile(localPath, content string) (domain.ExecuteResponse, error) {
	return a.service.SaveRawEnv(a.appContext(), localPath, content)
}

func (a *App) ExportRepoDiagnostics(localPath string) (domain.RepoDiagnosticExport, error) {
	return a.service.ExportRepoDiagnostics(a.appContext(), domain.RepoDiagnosticExportRequest{
		LocalPath: localPath,
	})
}

func (a *App) ExecuteStep(repoURL, localPath, stepID string, approveRisky bool) (domain.ExecuteResponse, error) {
	return a.service.Execute(a.appContext(), domain.ExecuteRequest{
		RepoURL:      repoURL,
		LocalPath:    localPath,
		StepID:       stepID,
		ApproveRisky: approveRisky,
	})
}

func (a *App) ShellInfo() map[string]string {
	return map[string]string{
		"shell":    "wails",
		"frontend": "react",
		"backend":  "go-service-layer",
		"adapter":  "pending",
	}
}

func (a *App) appContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
