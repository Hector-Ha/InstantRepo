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

func (a *App) GenerateEnvDraft(localPath string) (string, error) {
	return a.service.PreviewEnv(a.appContext(), localPath)
}

func (a *App) SaveEnvFile(localPath, content string) (domain.ExecuteResponse, error) {
	return a.service.SaveRawEnv(a.appContext(), localPath, content)
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
