package main

import "testing"

func TestVersionInfoUsesBuildVariables(t *testing.T) {
	oldAppVersion := appVersion
	oldGitCommit := gitCommit
	defer func() {
		appVersion = oldAppVersion
		gitCommit = oldGitCommit
	}()

	appVersion = "1.2.3"
	gitCommit = "abc123"

	info := versionInfo()
	if info.AppVersion != "1.2.3" {
		t.Fatalf("app version = %q", info.AppVersion)
	}
	if info.GitCommit != "abc123" {
		t.Fatalf("git commit = %q", info.GitCommit)
	}
	if info.CLIContractVersion == "" {
		t.Fatalf("missing CLI contract version")
	}
}
