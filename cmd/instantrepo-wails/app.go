package main

import (
	"context"
	"fmt"

	"instantrepo/internal/domain"
	"instantrepo/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	service *service.AppService
	initErr error
}

func NewApp() *App {
	appService, err := service.NewAppServiceWithDefaultStore()
	if err != nil {
		return &App{
			service: service.NewAppServiceWithInstalledRepoStore(nil),
			initErr: fmt.Errorf("initialize app data: %w", err),
		}
	}
	return &App{
		service: appService,
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
	appService, err := a.readyService()
	if err != nil {
		return domain.AnalyzeResponse{}, err
	}
	return appService.Analyze(a.appContext(), domain.AnalyzeRequest{
		RepoURL:   repoURL,
		LocalPath: localPath,
	})
}

func (a *App) ImportRepository(repoURL, destinationRoot string) (domain.AnalyzeResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.AnalyzeResponse{}, err
	}
	return appService.ImportRepository(a.appContext(), repoURL, destinationRoot)
}

func (a *App) ClonePreflight(repoURL, destinationRoot string) (domain.ClonePreflightResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.ClonePreflightResponse{}, err
	}
	return appService.ClonePreflight(a.appContext(), domain.ClonePreflightRequest{
		RepoURL:         repoURL,
		DestinationRoot: destinationRoot,
	})
}

func (a *App) ListInstalledRepos() (domain.InstalledRepoManagerResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.InstalledRepoManagerResponse{}, err
	}
	return appService.ListInstalledRepos(a.appContext())
}

func (a *App) InstalledRepoDetails(installedRepoID int64) (domain.InstalledRepoDetailsResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.InstalledRepoDetailsResponse{}, err
	}
	return appService.InstalledRepoDetails(a.appContext(), installedRepoID)
}

func (a *App) GenerateEnvDraft(localPath string) (domain.EnvDraft, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvDraft{}, err
	}
	return appService.GenerateEnvDraft(a.appContext(), localPath)
}

func (a *App) SaveEnvDraft(localPath string, draft domain.EnvDraft) (domain.ExecuteResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.ExecuteResponse{}, err
	}
	return appService.SaveStructuredEnvDraft(a.appContext(), localPath, draft)
}

func (a *App) SaveEnvVaultCredential(req domain.EnvVaultSaveRequest) (domain.EnvVaultSaveResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvVaultSaveResponse{}, err
	}
	return appService.SaveEnvVaultCredential(a.appContext(), req)
}

func (a *App) ListEnvVaultEntries() (domain.EnvVaultManagerResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvVaultManagerResponse{}, err
	}
	return appService.ListEnvVaultEntries(a.appContext())
}

func (a *App) RevealEnvVaultEntry(req domain.EnvVaultRevealRequest) (domain.EnvVaultRevealResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvVaultRevealResponse{}, err
	}
	return appService.RevealEnvVaultEntry(a.appContext(), req)
}

func (a *App) UpdateEnvVaultEntry(req domain.EnvVaultUpdateRequest) (domain.EnvVaultSaveResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvVaultSaveResponse{}, err
	}
	return appService.UpdateEnvVaultEntry(a.appContext(), req)
}

func (a *App) RemoveEnvVaultEntry(entryID int64) error {
	appService, err := a.readyService()
	if err != nil {
		return err
	}
	return appService.RemoveEnvVaultEntry(a.appContext(), entryID)
}

func (a *App) EnvContributionSettings() (domain.EnvContributionSettingsResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return appService.EnvContributionSettings(a.appContext())
}

func (a *App) SaveEnvContributionSettings(settings domain.EnvContributionSettings) (domain.EnvContributionSettingsResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return appService.SaveEnvContributionSettings(a.appContext(), settings)
}

func (a *App) RecordEnvContributionConsent(publicEnabled bool) (domain.EnvContributionSettingsResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return appService.RecordEnvContributionConsent(a.appContext(), publicEnabled)
}

func (a *App) ClearEnvContributionQueue() (domain.EnvContributionSettingsResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.EnvContributionSettingsResponse{}, err
	}
	return appService.ClearEnvContributionQueue(a.appContext())
}

func (a *App) AIEnvReviewSettings() (domain.AIEnvReviewSettings, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.AIEnvReviewSettings{}, err
	}
	return appService.AIEnvReviewSettings(a.appContext())
}

func (a *App) SaveAIEnvReviewSettings(settings domain.AIEnvReviewSettings) (domain.AIEnvReviewSettings, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.AIEnvReviewSettings{}, err
	}
	return appService.SaveAIEnvReviewSettings(a.appContext(), settings)
}

func (a *App) ApproveEnvVaultEntry(approval domain.EnvVaultApproval) error {
	appService, err := a.readyService()
	if err != nil {
		return err
	}
	return appService.ApproveEnvVaultEntry(a.appContext(), approval)
}

func (a *App) MarkEnvVaultEntryStatus(entryID int64, status string) error {
	appService, err := a.readyService()
	if err != nil {
		return err
	}
	return appService.MarkEnvVaultEntryStatus(a.appContext(), entryID, status)
}

func (a *App) RevokeEnvVaultApproval(approvalID int64) error {
	appService, err := a.readyService()
	if err != nil {
		return err
	}
	return appService.RevokeEnvVaultApproval(a.appContext(), approvalID)
}

func (a *App) SuppressEnvVaultPrompt(suppression domain.EnvVaultPromptSuppression) error {
	appService, err := a.readyService()
	if err != nil {
		return err
	}
	return appService.SuppressEnvVaultPrompt(a.appContext(), suppression)
}

func (a *App) SaveEnvFile(localPath, content string) (domain.ExecuteResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.ExecuteResponse{}, err
	}
	return appService.SaveRawEnv(a.appContext(), localPath, content)
}

func (a *App) ExportRepoDiagnostics(localPath string) (domain.RepoDiagnosticExport, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.RepoDiagnosticExport{}, err
	}
	return appService.ExportRepoDiagnostics(a.appContext(), domain.RepoDiagnosticExportRequest{
		LocalPath: localPath,
	})
}

func (a *App) ExecuteStep(repoURL, localPath, stepID string, approveRisky bool) (domain.ExecuteResponse, error) {
	appService, err := a.readyService()
	if err != nil {
		return domain.ExecuteResponse{}, err
	}
	return appService.Execute(a.appContext(), domain.ExecuteRequest{
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

func (a *App) readyService() (*service.AppService, error) {
	if a.initErr != nil {
		return nil, a.initErr
	}
	if a.service == nil {
		return nil, fmt.Errorf("app service is not initialized")
	}
	return a.service, nil
}

func (a *App) appContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
