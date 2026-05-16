package main

import (
	"strings"
	"testing"
)

func TestNewAppSurfacesInvalidAppDataOverride(t *testing.T) {
	t.Setenv("INSTANTREPO_APP_DATA_DIR", "relative-app-data")

	app := NewApp()
	if app.initErr == nil {
		t.Fatalf("expected init error")
	}

	_, err := app.AnalyzeRepository("", t.TempDir())
	if err == nil {
		t.Fatalf("expected analyze to return init error")
	}
	if !strings.Contains(err.Error(), "app data dir") {
		t.Fatalf("expected app data error, got %v", err)
	}
}

func TestShellInfoExposesBridgeContractVersion(t *testing.T) {
	app := &App{}

	info := app.ShellInfo()

	if info["shell"] != "wails" {
		t.Fatalf("shell = %q", info["shell"])
	}
	if info["bridgeContractVersion"] == "" {
		t.Fatalf("missing bridge contract version: %+v", info)
	}
}
