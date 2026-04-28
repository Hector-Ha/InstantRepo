package main

import (
	"context"

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

func (a *App) ShellInfo() map[string]string {
	return map[string]string{
		"shell":    "wails",
		"frontend": "react",
		"backend":  "go-service-layer",
		"adapter":  "pending",
	}
}
