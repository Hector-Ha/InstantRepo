package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"instantrepo/internal/domain"
)

func TestVersionCommandJSONReturnsContractMetadata(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:   []string{"version", "--json"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Version: VersionInfo{
			AppVersion:         "1.2.3",
			GitCommit:          "abc123",
			CLIContractVersion: "2026-05-issue-35",
		},
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("version must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	var payload struct {
		OK       bool        `json:"ok"`
		Data     VersionInfo `json:"data"`
		Metadata VersionInfo `json:"metadata"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("ok = false")
	}
	if payload.Data.CLIContractVersion != "2026-05-issue-35" {
		t.Fatalf("data contract = %q", payload.Data.CLIContractVersion)
	}
	if payload.Metadata.AppVersion != "1.2.3" || payload.Metadata.GitCommit != "abc123" {
		t.Fatalf("metadata = %+v", payload.Metadata)
	}
}

func TestVersionCommandHumanOutputIsDefault(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:   []string{"version"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Version: VersionInfo{
			AppVersion:         "1.2.3",
			GitCommit:          "abc123",
			CLIContractVersion: "2026-05-issue-35",
		},
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("version must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	output := stdout.String()
	if !strings.Contains(output, "InstantRepo 1.2.3") || !strings.Contains(output, "CLI contract 2026-05-issue-35") {
		t.Fatalf("output = %q", output)
	}
	if strings.Contains(output, `"ok"`) {
		t.Fatalf("human output should not be JSON: %s", output)
	}
}

func TestUnknownSubcommandJSONReturnsStructuredError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:    []string{"nope", "--json"},
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("unknown command must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Metadata VersionInfo `json:"metadata"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.OK {
		t.Fatalf("ok = true")
	}
	if payload.Error.Code != "unknown_command" || !strings.Contains(payload.Error.Message, "nope") {
		t.Fatalf("error = %+v", payload.Error)
	}
	if payload.Metadata.CLIContractVersion == "" {
		t.Fatalf("missing metadata")
	}
}

func TestMissingLegacyTargetJSONReturnsStructuredError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:    []string{"--json"},
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("missing target must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.OK {
		t.Fatalf("ok = true")
	}
	if payload.Error.Code != "missing_target" {
		t.Fatalf("error = %+v", payload.Error)
	}
}

func TestMissingLegacyTargetHumanOutputKeepsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:    nil,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("missing target must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: instantrepo") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if strings.Contains(stderr.String(), `"ok"`) || strings.Contains(stderr.String(), `"error"`) {
		t.Fatalf("human missing target should not be JSON: %s", stderr.String())
	}
}

func TestLegacyAnalyzeFlagsStillCallAnalyzeAndWriteRawJSON(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stdout bytes.Buffer
	fake := &fakeApp{
		analyzeResp: domain.AnalyzeResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"-path", repoPath},
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			return fake, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if fake.analyzeReq.LocalPath != repoPath {
		t.Fatalf("local path = %q", fake.analyzeReq.LocalPath)
	}
	var raw domain.AnalyzeResponse
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw analyze json: %v\n%s", err, stdout.String())
	}
	if raw.Source.Path != repoPath {
		t.Fatalf("source path = %q", raw.Source.Path)
	}
	if strings.Contains(stdout.String(), `"ok"`) {
		t.Fatalf("legacy output must stay raw Wails/domain JSON: %s", stdout.String())
	}
}

func TestLegacyExecuteFlagsStillCallExecute(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stdout bytes.Buffer
	fake := &fakeApp{
		executeResp: domain.ExecuteResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
			Result: domain.ExecutionResult{StepID: "install"},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"-path", repoPath, "-step", "install", "-approve"},
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			return fake, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if fake.executeReq.LocalPath != repoPath || fake.executeReq.StepID != "install" || !fake.executeReq.ApproveRisky {
		t.Fatalf("execute req = %+v", fake.executeReq)
	}
	var raw domain.ExecuteResponse
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw execute json: %v\n%s", err, stdout.String())
	}
	if raw.Result.StepID != "install" {
		t.Fatalf("step = %q", raw.Result.StepID)
	}
}

func TestAppDataDirFlagOverridesEnvironmentAndCreatesDirectory(t *testing.T) {
	envDir := filepath.Join(t.TempDir(), "env-app-data")
	flagDir := filepath.Join(t.TempDir(), "flag-app-data")
	repoPath := t.TempDir()
	var gotConfig AppConfig

	exitCode := Run(context.Background(), Options{
		Args:    []string{"--app-data-dir", flagDir, "-path", repoPath},
		Environ: []string{"INSTANTREPO_APP_DATA_DIR=" + envDir},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			gotConfig = config
			return &fakeApp{analyzeResp: domain.AnalyzeResponse{Source: domain.RepoSource{Path: repoPath}}}, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if gotConfig.AppDataDir != filepath.Clean(flagDir) {
		t.Fatalf("app data dir = %q, want %q", gotConfig.AppDataDir, filepath.Clean(flagDir))
	}
	if _, err := os.Stat(flagDir); err != nil {
		t.Fatalf("flag app data dir not created: %v", err)
	}
	if _, err := os.Stat(envDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("env app data dir should not be created, stat err = %v", err)
	}
}

func TestAppDataDirUsesEnvironmentWhenFlagMissing(t *testing.T) {
	envDir := filepath.Join(t.TempDir(), "env-app-data")
	repoPath := t.TempDir()
	var gotConfig AppConfig

	exitCode := Run(context.Background(), Options{
		Args:    []string{"-path", repoPath},
		Environ: []string{"INSTANTREPO_APP_DATA_DIR=" + envDir},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			gotConfig = config
			return &fakeApp{analyzeResp: domain.AnalyzeResponse{Source: domain.RepoSource{Path: repoPath}}}, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if gotConfig.AppDataDir != filepath.Clean(envDir) {
		t.Fatalf("app data dir = %q, want %q", gotConfig.AppDataDir, filepath.Clean(envDir))
	}
	if _, err := os.Stat(envDir); err != nil {
		t.Fatalf("env app data dir not created: %v", err)
	}
}

func TestAppDataDirEnvironmentNameIsCaseInsensitive(t *testing.T) {
	envDir := filepath.Join(t.TempDir(), "env-app-data")
	repoPath := t.TempDir()
	var gotConfig AppConfig

	exitCode := Run(context.Background(), Options{
		Args:    []string{"-path", repoPath},
		Environ: []string{"instantrepo_app_data_dir=" + envDir},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			gotConfig = config
			return &fakeApp{analyzeResp: domain.AnalyzeResponse{Source: domain.RepoSource{Path: repoPath}}}, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if gotConfig.AppDataDir != filepath.Clean(envDir) {
		t.Fatalf("app data dir = %q, want %q", gotConfig.AppDataDir, filepath.Clean(envDir))
	}
}

func TestAppDataDirRejectsTargetRepoAndChild(t *testing.T) {
	repoPath := t.TempDir()
	for _, appDataDir := range []string{
		repoPath,
		filepath.Join(repoPath, "child"),
	} {
		var stderr bytes.Buffer
		exitCode := Run(context.Background(), Options{
			Args:    []string{"--app-data-dir", appDataDir, "-path", repoPath},
			Stdout:  &bytes.Buffer{},
			Stderr:  &stderr,
			Version: defaultTestVersion(),
			NewApp: func(AppConfig) (App, func() error, error) {
				t.Fatal("invalid app data dir must not create app service")
				return nil, nil, nil
			},
		})
		if exitCode == 0 {
			t.Fatalf("exit code = 0 for %s", appDataDir)
		}
		if !strings.Contains(stderr.String(), "app data dir") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	}
}

func TestAppDataDirRejectsRelativeHomeAndWorkingRepoRoot(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("wd: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join(wd, "..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	for _, appDataDir := range []string{
		"relative-app-data",
		homeDir,
		repoRoot,
	} {
		var stderr bytes.Buffer
		exitCode := Run(context.Background(), Options{
			Args:    []string{"--app-data-dir", appDataDir, "version", "--json"},
			Stdout:  &bytes.Buffer{},
			Stderr:  &stderr,
			Version: defaultTestVersion(),
			NewApp: func(AppConfig) (App, func() error, error) {
				t.Fatal("invalid app data dir must not create app service")
				return nil, nil, nil
			},
		})
		if exitCode == 0 {
			t.Fatalf("exit code = 0 for %s", appDataDir)
		}
		var payload struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
			t.Fatalf("decode stderr for %s: %v\n%s", appDataDir, err, stderr.String())
		}
		if payload.Error.Code != "invalid_app_data_dir" {
			t.Fatalf("code = %q for %s", payload.Error.Code, appDataDir)
		}
	}
}

func TestAppDataDirRejectsRepoRootWhenLaunchedOutsideSourceTree(t *testing.T) {
	launcherDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "InstantRepo")
	if err := os.MkdirAll(repoRoot, 0o700); err != nil {
		t.Fatalf("create repo root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module instantrepo\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	t.Chdir(launcherDir)

	var stderr bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:    []string{"--app-data-dir", repoRoot, "version", "--json"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("invalid app data dir must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "invalid_app_data_dir" {
		t.Fatalf("code = %q", payload.Error.Code)
	}
	if !strings.Contains(payload.Error.Message, "repo root") {
		t.Fatalf("message = %q", payload.Error.Message)
	}
}

type fakeApp struct {
	analyzeReq  domain.AnalyzeRequest
	analyzeResp domain.AnalyzeResponse
	executeReq  domain.ExecuteRequest
	executeResp domain.ExecuteResponse
}

func (f *fakeApp) Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.AnalyzeResponse, error) {
	f.analyzeReq = req
	return f.analyzeResp, nil
}

func (f *fakeApp) Execute(ctx context.Context, req domain.ExecuteRequest) (domain.ExecuteResponse, error) {
	f.executeReq = req
	return f.executeResp, nil
}

func defaultTestVersion() VersionInfo {
	return VersionInfo{
		AppVersion:         "dev",
		GitCommit:          "",
		CLIContractVersion: CLIContractVersion,
	}
}
