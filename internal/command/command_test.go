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
	"time"

	"instantrepo/internal/domain"
)

func TestVersionCommandJSONReturnsContractMetadata(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:   []string{"version", "--json"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Version: VersionInfo{
			AppVersion:            "1.2.3",
			GitCommit:             "abc123",
			CLIContractVersion:    "2026-05-issue-40",
			BridgeContractVersion: "2026-05-bridge-1",
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
	if payload.Data.CLIContractVersion != "2026-05-issue-40" {
		t.Fatalf("data contract = %q", payload.Data.CLIContractVersion)
	}
	if payload.Data.BridgeContractVersion != "2026-05-bridge-1" || payload.Metadata.BridgeContractVersion != "2026-05-bridge-1" {
		t.Fatalf("bridge metadata = data:%q metadata:%q", payload.Data.BridgeContractVersion, payload.Metadata.BridgeContractVersion)
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
			AppVersion:            "1.2.3",
			GitCommit:             "abc123",
			CLIContractVersion:    "2026-05-issue-40",
			BridgeContractVersion: "2026-05-bridge-1",
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
	if !strings.Contains(output, "InstantRepo 1.2.3") || !strings.Contains(output, "CLI contract 2026-05-issue-40") || !strings.Contains(output, "Bridge contract 2026-05-bridge-1") {
		t.Fatalf("output = %q", output)
	}
	if strings.Contains(output, `"ok"`) {
		t.Fatalf("human output should not be JSON: %s", output)
	}
}

func TestSettingsContributionGetJSONReturnsDomainResponse(t *testing.T) {
	var stdout bytes.Buffer
	fake := &fakeApp{
		envContributionSettingsResp: domain.EnvContributionSettingsResponse{
			Settings: domain.EnvContributionSettings{
				PublicEnvPatternsEnabled:       true,
				PrivateLocalEnvPatternsEnabled: false,
				ConsentShown:                   true,
			},
			Queue: domain.EnvContributionQueueStatus{Count: 2},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"settings", "contribution", "get", "--json"},
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
	if !fake.envContributionSettingsCalled {
		t.Fatalf("EnvContributionSettings was not called")
	}
	var payload struct {
		OK   bool                                   `json:"ok"`
		Data domain.EnvContributionSettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || !payload.Data.Settings.PublicEnvPatternsEnabled || payload.Data.Queue.Count != 2 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestSettingsContributionSaveFileAndStdinJSONReturnDomainResponse(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		in   string
	}{
		{name: "file", args: []string{"settings", "contribution", "save", "--file", "", "--json"}},
		{name: "stdin", args: []string{"settings", "contribution", "save", "--stdin", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := domain.EnvContributionSettings{
				PublicEnvPatternsEnabled:       false,
				PrivateLocalEnvPatternsEnabled: true,
				ConsentShown:                   true,
			}
			raw, err := json.Marshal(settings)
			if err != nil {
				t.Fatalf("marshal settings: %v", err)
			}
			args := append([]string(nil), tc.args...)
			stdin := strings.NewReader("")
			if tc.name == "file" {
				inputPath := filepath.Join(t.TempDir(), "settings.json")
				if err := os.WriteFile(inputPath, raw, 0o600); err != nil {
					t.Fatalf("write settings: %v", err)
				}
				args[4] = inputPath
			} else {
				stdin = strings.NewReader(string(raw))
			}
			var stdout bytes.Buffer
			fake := &fakeApp{
				saveEnvContributionSettingsResp: domain.EnvContributionSettingsResponse{
					Settings: settings,
					Queue:    domain.EnvContributionQueueStatus{Count: 0},
				},
			}

			exitCode := Run(context.Background(), Options{
				Args:    args,
				Stdin:   stdin,
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
			if !fake.saveEnvContributionSettingsCalled || !fake.saveEnvContributionSettingsArg.PrivateLocalEnvPatternsEnabled {
				t.Fatalf("save arg = %+v called=%t", fake.saveEnvContributionSettingsArg, fake.saveEnvContributionSettingsCalled)
			}
			var payload struct {
				OK   bool                                   `json:"ok"`
				Data domain.EnvContributionSettingsResponse `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
			}
			if !payload.OK || !payload.Data.Settings.PrivateLocalEnvPatternsEnabled {
				t.Fatalf("payload = %+v", payload)
			}
		})
	}
}

func TestSettingsContributionConsentAndClearQueueJSONReturnDomainResponse(t *testing.T) {
	t.Run("consent", func(t *testing.T) {
		var stdout bytes.Buffer
		fake := &fakeApp{
			recordEnvContributionConsentResp: domain.EnvContributionSettingsResponse{
				Settings: domain.EnvContributionSettings{
					PublicEnvPatternsEnabled: true,
					ConsentShown:             true,
				},
				Queue: domain.EnvContributionQueueStatus{Count: 0},
			},
		}

		exitCode := Run(context.Background(), Options{
			Args:    []string{"settings", "contribution", "consent", "--public-enabled", "true", "--json"},
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
		if !fake.recordEnvContributionConsentCalled || !fake.recordEnvContributionConsentArg {
			t.Fatalf("consent arg = %t called=%t", fake.recordEnvContributionConsentArg, fake.recordEnvContributionConsentCalled)
		}
		var payload struct {
			Data domain.EnvContributionSettingsResponse `json:"data"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
		}
		if !payload.Data.Settings.ConsentShown {
			t.Fatalf("payload = %+v", payload)
		}
	})

	t.Run("clear queue", func(t *testing.T) {
		var stdout bytes.Buffer
		fake := &fakeApp{
			clearEnvContributionQueueResp: domain.EnvContributionSettingsResponse{
				Settings: domain.EnvContributionSettings{ConsentShown: true},
				Queue:    domain.EnvContributionQueueStatus{Count: 0},
			},
		}

		exitCode := Run(context.Background(), Options{
			Args:    []string{"settings", "contribution", "clear-queue", "--json"},
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
		if !fake.clearEnvContributionQueueCalled {
			t.Fatalf("ClearEnvContributionQueue was not called")
		}
		var payload struct {
			Data domain.EnvContributionSettingsResponse `json:"data"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
		}
		if payload.Data.Queue.Count != 0 {
			t.Fatalf("payload = %+v", payload)
		}
	})
}

func TestSettingsAIEnvReviewGetAndSaveJSONReturnDomainResponse(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		var stdout bytes.Buffer
		fake := &fakeApp{aiEnvReviewSettingsResp: domain.AIEnvReviewSettings{Enabled: true}}

		exitCode := Run(context.Background(), Options{
			Args:    []string{"settings", "ai-env-review", "get", "--json"},
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
		if !fake.aiEnvReviewSettingsCalled {
			t.Fatalf("AIEnvReviewSettings was not called")
		}
		var payload struct {
			Data domain.AIEnvReviewSettings `json:"data"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
		}
		if !payload.Data.Enabled {
			t.Fatalf("payload = %+v", payload)
		}
	})

	t.Run("save", func(t *testing.T) {
		settings := domain.AIEnvReviewSettings{Enabled: true}
		raw, err := json.Marshal(settings)
		if err != nil {
			t.Fatalf("marshal settings: %v", err)
		}
		var stdout bytes.Buffer
		fake := &fakeApp{saveAIEnvReviewSettingsResp: settings}

		exitCode := Run(context.Background(), Options{
			Args:    []string{"settings", "ai-env-review", "save", "--stdin", "--json"},
			Stdin:   strings.NewReader(string(raw)),
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
		if !fake.saveAIEnvReviewSettingsCalled || !fake.saveAIEnvReviewSettingsArg.Enabled {
			t.Fatalf("save arg = %+v called=%t", fake.saveAIEnvReviewSettingsArg, fake.saveAIEnvReviewSettingsCalled)
		}
		var payload struct {
			Data domain.AIEnvReviewSettings `json:"data"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
		}
		if !payload.Data.Enabled {
			t.Fatalf("payload = %+v", payload)
		}
	})
}

func TestShellInfoJSONReturnsBridgeMetadata(t *testing.T) {
	var stdout bytes.Buffer

	exitCode := Run(context.Background(), Options{
		Args:   []string{"shell", "info", "--json"},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Version: VersionInfo{
			AppVersion:            "1.2.3",
			GitCommit:             "abc123",
			CLIContractVersion:    "2026-05-issue-40",
			BridgeContractVersion: "2026-05-bridge-1",
		},
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("shell info must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	var payload struct {
		OK   bool              `json:"ok"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data["shell"] != "cli" || payload.Data["bridgeContractVersion"] != "2026-05-bridge-1" || payload.Data["cliContractVersion"] != "2026-05-issue-40" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestSettingsPassesIsolatedAppDataAndCreatesDirectory(t *testing.T) {
	appDataDir := filepath.Join(t.TempDir(), "app-data")
	var gotConfig AppConfig

	exitCode := Run(context.Background(), Options{
		Args:    []string{"settings", "contribution", "get", "--app-data-dir", appDataDir, "--json"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			gotConfig = config
			return &fakeApp{}, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if gotConfig.AppDataDir != filepath.Clean(appDataDir) {
		t.Fatalf("app data dir = %q, want %q", gotConfig.AppDataDir, filepath.Clean(appDataDir))
	}
	if _, err := os.Stat(appDataDir); err != nil {
		t.Fatalf("app data dir not created: %v", err)
	}
}

func TestSettingsCommandsPersistWithIsolatedAppData(t *testing.T) {
	appDataDir := filepath.Join(t.TempDir(), "app-data")
	runJSON := func(args []string, stdin string, out any) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exitCode := Run(context.Background(), Options{
			Args:    append(args, "--app-data-dir", appDataDir, "--json"),
			Stdin:   strings.NewReader(stdin),
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: defaultTestVersion(),
		})
		if exitCode != 0 {
			t.Fatalf("exit code = %d stderr=%s", exitCode, stderr.String())
		}
		if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
			t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
		}
	}

	settingsJSON := `{"publicEnvPatternsEnabled":false,"privateLocalEnvPatternsEnabled":true,"consentShown":true}`
	var savePayload struct {
		Data domain.EnvContributionSettingsResponse `json:"data"`
	}
	runJSON([]string{"settings", "contribution", "save", "--stdin"}, settingsJSON, &savePayload)
	if savePayload.Data.Settings.PublicEnvPatternsEnabled || !savePayload.Data.Settings.PrivateLocalEnvPatternsEnabled || !savePayload.Data.Settings.ConsentShown {
		t.Fatalf("saved contribution settings = %+v", savePayload.Data.Settings)
	}

	var getPayload struct {
		Data domain.EnvContributionSettingsResponse `json:"data"`
	}
	runJSON([]string{"settings", "contribution", "get"}, "", &getPayload)
	if getPayload.Data.Settings.PublicEnvPatternsEnabled || !getPayload.Data.Settings.PrivateLocalEnvPatternsEnabled || !getPayload.Data.Settings.ConsentShown {
		t.Fatalf("persisted contribution settings = %+v", getPayload.Data.Settings)
	}

	var aiSavePayload struct {
		Data domain.AIEnvReviewSettings `json:"data"`
	}
	runJSON([]string{"settings", "ai-env-review", "save", "--stdin"}, `{"enabled":true}`, &aiSavePayload)
	if !aiSavePayload.Data.Enabled {
		t.Fatalf("saved AI Env Review settings = %+v", aiSavePayload.Data)
	}

	var aiGetPayload struct {
		Data domain.AIEnvReviewSettings `json:"data"`
	}
	runJSON([]string{"settings", "ai-env-review", "get"}, "", &aiGetPayload)
	if !aiGetPayload.Data.Enabled {
		t.Fatalf("persisted AI Env Review settings = %+v", aiGetPayload.Data)
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

func TestRepoAnalyzePathJSONReturnsDomainResponse(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stdout bytes.Buffer
	fake := &fakeApp{
		analyzeResp: domain.AnalyzeResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
			Analysis: domain.RepositoryAnalysis{
				ProjectName: "demo",
				ProjectType: "node",
			},
			Plan: domain.SetupPlan{
				Steps: []domain.ExecutionStep{{ID: "install", Title: "Install deps"}},
			},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "analyze", "--path", repoPath, "--json"},
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
	var payload struct {
		OK   bool                   `json:"ok"`
		Data domain.AnalyzeResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK {
		t.Fatalf("ok = false")
	}
	if payload.Data.Source.Path != repoPath || payload.Data.Analysis.ProjectName != "demo" {
		t.Fatalf("data = %+v", payload.Data)
	}
}

func TestEnvDraftGenerateJSONReturnsDomainDraft(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stdout bytes.Buffer
	fake := &fakeApp{
		envDraftResp: domain.EnvDraft{
			RepoPath: repoPath,
			Targets: []domain.EnvDraftTarget{{
				RelativePath: ".env",
				AbsolutePath: filepath.Join(repoPath, ".env"),
				Values: []domain.EnvDraftValue{{
					Name:       "JWT_SECRET",
					Value:      "secret-value",
					Secret:     true,
					ValueClass: domain.EnvValueClassGeneratedLocalSecret,
					Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceGeneratedSecret},
				}},
			}},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "draft", "generate", "--path", repoPath, "--json"},
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
	if fake.generateEnvDraftPath != repoPath {
		t.Fatalf("generate path = %q", fake.generateEnvDraftPath)
	}
	var payload struct {
		OK   bool            `json:"ok"`
		Data domain.EnvDraft `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.RepoPath != repoPath || len(payload.Data.Targets) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Data.Targets[0].Values[0].Value != "secret-value" {
		t.Fatalf("draft JSON should preserve generated draft values for save-from-generate")
	}
}

func TestEnvDraftGenerateHumanOutputRedactsValues(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stdout bytes.Buffer
	fake := &fakeApp{
		envDraftResp: domain.EnvDraft{
			RepoPath: repoPath,
			Targets: []domain.EnvDraftTarget{{
				RelativePath: "api/.env",
				AbsolutePath: filepath.Join(repoPath, "api", ".env"),
				Values: []domain.EnvDraftValue{
					{
						Name:       "JWT_SECRET",
						Value:      "secret-value",
						Secret:     true,
						ValueClass: domain.EnvValueClassGeneratedLocalSecret,
						Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceGeneratedSecret},
					},
					{
						Name:       "STRIPE_SECRET_KEY",
						Value:      "",
						Secret:     true,
						ValueClass: domain.EnvValueClassServiceCredential,
						Attention:  []string{"Add Stripe key."},
						Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceCatalog},
					},
				},
			}},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "draft", "generate", "--path", repoPath},
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
	output := stdout.String()
	for _, want := range []string{
		"Repo: " + repoPath,
		"Env targets: 1",
		"api/.env: 2 values",
		"Service credentials: 1",
		"Action needed: 1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
	for _, secret := range []string{"secret-value", "sk-test"} {
		if strings.Contains(output, secret) {
			t.Fatalf("human output leaked %q: %s", secret, output)
		}
	}
}

func TestEnvDraftSaveFileJSONReturnsExecuteResponse(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	draft := domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{{
			RelativePath: ".env",
			AbsolutePath: filepath.Join(repoPath, ".env"),
			Values: []domain.EnvDraftValue{{
				Name:       "APP_URL",
				Value:      "http://localhost:3000",
				ValueClass: domain.EnvValueClassDevDefault,
				Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceCatalog},
			}},
		}},
	}
	draftPath := filepath.Join(t.TempDir(), "draft.json")
	draftBytes, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	if err := os.WriteFile(draftPath, draftBytes, 0o600); err != nil {
		t.Fatalf("write draft: %v", err)
	}
	var stdout bytes.Buffer
	fake := &fakeApp{
		saveStructuredResp: domain.ExecuteResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
			Result: domain.ExecutionResult{StepID: "create-env-file", Succeeded: true},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "draft", "save", "--path", repoPath, "--file", draftPath, "--json"},
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
	if fake.saveStructuredPath != repoPath {
		t.Fatalf("save path = %q", fake.saveStructuredPath)
	}
	if len(fake.saveStructuredDraft.Targets) != 1 || fake.saveStructuredDraft.Targets[0].Values[0].Name != "APP_URL" {
		t.Fatalf("saved draft = %+v", fake.saveStructuredDraft)
	}
	var payload struct {
		OK   bool                   `json:"ok"`
		Data domain.ExecuteResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.Result.StepID != "create-env-file" || !payload.Data.Result.Succeeded {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestEnvDraftSaveStdinJSONReturnsExecuteResponse(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	draft := domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{{
			RelativePath: ".env",
			AbsolutePath: filepath.Join(repoPath, ".env"),
			Values: []domain.EnvDraftValue{{
				Name:       "APP_URL",
				Value:      "http://localhost:5173",
				ValueClass: domain.EnvValueClassDevDefault,
				Provenance: domain.EnvValueProvenance{Source: domain.EnvValueSourceCatalog},
			}},
		}},
	}
	draftBytes, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	var stdout bytes.Buffer
	fake := &fakeApp{
		saveStructuredResp: domain.ExecuteResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
			Result: domain.ExecutionResult{StepID: "create-env-file", Succeeded: true},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "draft", "save", "--path", repoPath, "--stdin", "--json"},
		Stdin:   strings.NewReader(string(draftBytes)),
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
	if fake.saveStructuredPath != repoPath || fake.saveStructuredDraft.Targets[0].Values[0].Value != "http://localhost:5173" {
		t.Fatalf("save args = %q %+v", fake.saveStructuredPath, fake.saveStructuredDraft)
	}
	var payload struct {
		OK   bool                   `json:"ok"`
		Data domain.ExecuteResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || !payload.Data.Result.Succeeded {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestEnvDraftSaveInvalidJSONReturnsStructuredInputError(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "draft", "save", "--path", repoPath, "--stdin", "--json"},
		Stdin:   strings.NewReader(`{"targets":[{"values":[{"value":"secret-value"}]}`),
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("invalid draft JSON must not create app service")
			return nil, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "invalid_input" {
		t.Fatalf("error = %+v", payload.Error)
	}
	if strings.Contains(stderr.String(), "secret-value") {
		t.Fatalf("error leaked raw input: %s", stderr.String())
	}
}

func TestEnvDraftSavePolicyFailureReturnsStructuredError(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	draft := domain.EnvDraft{
		RepoPath: repoPath,
		Targets: []domain.EnvDraftTarget{{
			RelativePath: ".env",
			AbsolutePath: filepath.Join(repoPath, ".env"),
			Values: []domain.EnvDraftValue{{
				Name:       "API_SECRET",
				Value:      "secret-value",
				Secret:     true,
				ValueClass: domain.EnvValueClassGeneratedLocalSecret,
			}},
		}},
	}
	raw, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	var stderr bytes.Buffer
	fake := &fakeApp{saveStructuredErr: errors.New("save failed; rolled back first target")}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "draft", "save", "--path", repoPath, "--stdin", "--json"},
		Stdin:   strings.NewReader(string(raw)),
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			return fake, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "env_draft_save_failed" || !strings.Contains(payload.Error.Message, "rolled back") {
		t.Fatalf("error = %+v", payload.Error)
	}
	if strings.Contains(stderr.String(), "secret-value") {
		t.Fatalf("error leaked draft value: %s", stderr.String())
	}
}

func TestEnvDraftGeneratePassesIsolatedAppDataAndCreatesDirectory(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	appDataDir := filepath.Join(t.TempDir(), "app-data")
	var gotConfig AppConfig
	fake := &fakeApp{envDraftResp: domain.EnvDraft{RepoPath: repoPath}}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "draft", "generate", "--path", repoPath, "--app-data-dir", appDataDir, "--json"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			gotConfig = config
			return fake, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if gotConfig.AppDataDir != filepath.Clean(appDataDir) {
		t.Fatalf("app data dir = %q, want %q", gotConfig.AppDataDir, filepath.Clean(appDataDir))
	}
	if _, err := os.Stat(appDataDir); err != nil {
		t.Fatalf("app data dir not created: %v", err)
	}
	if fake.generateEnvDraftPath != repoPath {
		t.Fatalf("generate path = %q", fake.generateEnvDraftPath)
	}
}

func TestEnvRawSaveFileJSONReturnsExecuteResponseWithoutEchoingContent(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	inputPath := filepath.Join(t.TempDir(), ".env")
	rawEnv := "API_SECRET=secret-value\nSTRIPE_SECRET_KEY=sk-test-secret\n"
	if err := os.WriteFile(inputPath, []byte(rawEnv), 0o600); err != nil {
		t.Fatalf("write raw env: %v", err)
	}
	var stdout bytes.Buffer
	fake := &fakeApp{
		saveRawResp: domain.ExecuteResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
			Result: domain.ExecutionResult{StepID: "save-env-file", Succeeded: true},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "raw", "save", "--path", repoPath, "--file", inputPath, "--json"},
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
	if fake.saveRawPath != repoPath || fake.saveRawContent != rawEnv {
		t.Fatalf("raw save args = %q %q", fake.saveRawPath, fake.saveRawContent)
	}
	var payload struct {
		OK       bool                   `json:"ok"`
		Data     domain.ExecuteResponse `json:"data"`
		Metadata VersionInfo            `json:"metadata"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.Result.StepID != "save-env-file" {
		t.Fatalf("payload = %+v", payload)
	}
	for _, secret := range []string{"secret-value", "sk-test-secret"} {
		if strings.Contains(stdout.String(), secret) {
			t.Fatalf("JSON output leaked %q: %s", secret, stdout.String())
		}
	}
}

func TestEnvRawSaveStdinHumanOutputDoesNotEchoContent(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	rawEnv := "API_SECRET=secret-value\n"
	var stdout bytes.Buffer
	fake := &fakeApp{
		saveRawResp: domain.ExecuteResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
			Result: domain.ExecutionResult{
				StepID:    "save-env-file",
				Succeeded: true,
				Stdout:    "Saved .env\n",
			},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "raw", "save", "--path", repoPath, "--stdin"},
		Stdin:   strings.NewReader(rawEnv),
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
	if fake.saveRawContent != rawEnv {
		t.Fatalf("raw content = %q", fake.saveRawContent)
	}
	output := stdout.String()
	for _, want := range []string{"Step: save-env-file", "Status: succeeded", "Saved .env"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, "secret-value") {
		t.Fatalf("human output leaked raw env content: %s", output)
	}
}

func TestEnvRawSavePolicyFailureReturnsStructuredError(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stderr bytes.Buffer
	fake := &fakeApp{saveRawErr: errors.New("raw env save supports one env target; use structured env draft for multiple targets")}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "raw", "save", "--path", repoPath, "--stdin", "--json"},
		Stdin:   strings.NewReader("API_SECRET=secret-value\n"),
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			return fake, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "env_raw_save_failed" || !strings.Contains(payload.Error.Message, "structured env draft") {
		t.Fatalf("error = %+v", payload.Error)
	}
	if strings.Contains(stderr.String(), "secret-value") {
		t.Fatalf("error leaked raw env content: %s", stderr.String())
	}
}

func TestRepoAnalyzeHumanOutputIsConciseSummary(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stdout bytes.Buffer
	fake := &fakeApp{
		analyzeResp: domain.AnalyzeResponse{
			Source:   domain.RepoSource{Type: "local", Path: repoPath},
			Analysis: domain.RepositoryAnalysis{ProjectName: "demo", ProjectType: "go"},
			Plan: domain.SetupPlan{
				Steps:  []domain.ExecutionStep{{ID: "download"}},
				Safety: domain.SafetyReport{Findings: []domain.SafetyFinding{{Severity: "attention", Summary: "script"}}},
			},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "analyze", "--path", repoPath},
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
	output := stdout.String()
	for _, want := range []string{"Repo:", repoPath, "Project: demo (go)", "Setup steps: 1", "Attention: 1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, `"ok"`) {
		t.Fatalf("human output should not be JSON: %s", output)
	}
}

func TestRepoPreflightJSONReturnsDomainResponse(t *testing.T) {
	destination := filepath.Clean(t.TempDir())
	target := filepath.Join(destination, "demo")
	var stdout bytes.Buffer
	fake := &fakeApp{
		preflightResp: domain.ClonePreflightResponse{
			RepoURL:           "https://github.com/acme/demo",
			DestinationRoot:   destination,
			TargetPath:        target,
			RecommendedAction: domain.CloneActionChooseDifferentFolder,
			Messages:          []domain.ClonePreflightMessage{{Severity: "attention", Text: "Target folder is not empty."}},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "preflight", "--repo", "https://github.com/acme/demo", "--destination", destination, "--json"},
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
	if fake.preflightReq.RepoURL != "https://github.com/acme/demo" || fake.preflightReq.DestinationRoot != destination {
		t.Fatalf("preflight req = %+v", fake.preflightReq)
	}
	var payload struct {
		OK   bool                          `json:"ok"`
		Data domain.ClonePreflightResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.TargetPath != target {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestRepoPreflightHumanOutputIncludesActionAndMessages(t *testing.T) {
	destination := filepath.Clean(t.TempDir())
	var stdout bytes.Buffer
	fake := &fakeApp{
		preflightResp: domain.ClonePreflightResponse{
			RepoURL:           "https://github.com/acme/demo",
			DestinationRoot:   destination,
			TargetPath:        filepath.Join(destination, "demo"),
			RecommendedAction: domain.CloneActionFreeDiskSpace,
			Messages: []domain.ClonePreflightMessage{
				{Severity: "risk", Text: "Destination disk has less than 1 GiB free."},
			},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "preflight", "--repo", "https://github.com/acme/demo", "--destination", destination},
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
	output := stdout.String()
	for _, want := range []string{"Action: free-disk-space", "Target:", "risk: Destination disk has less than 1 GiB free."} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
}

func TestRepoPreflightRejectsAppDataInsideFutureTargetBeforeCreatingIt(t *testing.T) {
	destination := filepath.Clean(t.TempDir())
	appDataDir := filepath.Join(destination, "demo", "app-data")
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "preflight", "--repo", "https://github.com/acme/demo", "--destination", destination, "--app-data-dir", appDataDir, "--json"},
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
	if _, err := os.Stat(appDataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("app data child should not be created, stat err = %v", err)
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "invalid_app_data_dir" {
		t.Fatalf("error = %+v", payload.Error)
	}
}

func TestRepoImportJSONReturnsAnalyzeResponse(t *testing.T) {
	destination := filepath.Clean(t.TempDir())
	clonedPath := filepath.Join(destination, "demo")
	var stdout bytes.Buffer
	fake := &fakeApp{
		importResp: domain.AnalyzeResponse{
			Source: domain.RepoSource{Type: "github", RepoURL: "https://github.com/acme/demo", Path: clonedPath},
			Plan:   domain.SetupPlan{Steps: []domain.ExecutionStep{{ID: "install"}}},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "import", "--repo", "https://github.com/acme/demo", "--destination", destination, "--json"},
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
	if fake.importRepoURL != "https://github.com/acme/demo" || fake.importDestination != destination {
		t.Fatalf("import args = %q %q", fake.importRepoURL, fake.importDestination)
	}
	var payload struct {
		OK   bool                   `json:"ok"`
		Data domain.AnalyzeResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.Source.Path != clonedPath {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestRepoImportRejectsAppDataInsideFutureTargetBeforeCreatingIt(t *testing.T) {
	destination := filepath.Clean(t.TempDir())
	appDataDir := filepath.Join(destination, "demo", "app-data")
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "import", "--repo", "https://github.com/acme/demo", "--destination", destination, "--app-data-dir", appDataDir, "--json"},
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
	if _, err := os.Stat(appDataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("app data child should not be created, stat err = %v", err)
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "invalid_app_data_dir" {
		t.Fatalf("error = %+v", payload.Error)
	}
}

func TestRepoImportMissingDestinationJSONReturnsStructuredError(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "import", "--repo", "https://github.com/acme/demo", "--json"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("invalid import must not create app")
			return nil, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "missing_destination" {
		t.Fatalf("error = %+v", payload.Error)
	}
}

func TestRepoImportPathConflictJSONReturnsStructuredError(t *testing.T) {
	destination := filepath.Clean(t.TempDir())
	var stderr bytes.Buffer
	fake := &fakeApp{importErr: errors.New("target folder is not empty")}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "import", "--repo", "https://github.com/acme/demo", "--destination", destination, "--json"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			return fake, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "import_failed" || !strings.Contains(payload.Error.Message, "not empty") {
		t.Fatalf("error = %+v", payload.Error)
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

func TestRepoExecuteJSONReturnsStructuredApprovalFailure(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	var stderr bytes.Buffer
	fake := &fakeApp{executeErr: errors.New(`step "install" requires approval; retry with approveRisky=true`)}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "execute", "--path", repoPath, "--step", "install", "--json"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			return fake, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	if fake.executeReq.LocalPath != repoPath || fake.executeReq.StepID != "install" || fake.executeReq.ApproveRisky {
		t.Fatalf("execute req = %+v", fake.executeReq)
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "execute_failed" || !strings.Contains(payload.Error.Message, "requires approval") {
		t.Fatalf("error = %+v", payload.Error)
	}
}

func TestRepoExecuteApprovePassesIsolatedAppDataAndReturnsDomainResponse(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	appDataDir := filepath.Join(t.TempDir(), "app-data")
	var stdout bytes.Buffer
	var gotConfig AppConfig
	fake := &fakeApp{
		executeResp: domain.ExecuteResponse{
			Source: domain.RepoSource{Type: "local", Path: repoPath},
			Result: domain.ExecutionResult{StepID: "install", Succeeded: true, Stdout: "done"},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "execute", "--path", repoPath, "--step", "install", "--approve", "--app-data-dir", appDataDir, "--json"},
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			gotConfig = config
			return fake, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if gotConfig.AppDataDir != filepath.Clean(appDataDir) {
		t.Fatalf("app data dir = %q", gotConfig.AppDataDir)
	}
	if fake.executeReq.LocalPath != repoPath || fake.executeReq.StepID != "install" || !fake.executeReq.ApproveRisky {
		t.Fatalf("execute req = %+v", fake.executeReq)
	}
	var payload struct {
		OK   bool                   `json:"ok"`
		Data domain.ExecuteResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || payload.Data.Result.StepID != "install" || !payload.Data.Result.Succeeded {
		t.Fatalf("payload = %+v", payload)
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

func TestMetadataCommandsDoNotCreateAppDataDir(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "version", args: []string{"version", "--json"}},
		{name: "shell info", args: []string{"shell", "info", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appDataDir := filepath.Join(t.TempDir(), "metadata-app-data")
			args := append(append([]string{}, tc.args...), "--app-data-dir", appDataDir)

			exitCode := Run(context.Background(), Options{
				Args:    args,
				Stdout:  &bytes.Buffer{},
				Stderr:  &bytes.Buffer{},
				Version: defaultTestVersion(),
				NewApp: func(AppConfig) (App, func() error, error) {
					t.Fatal("metadata command must not create app service")
					return nil, nil, nil
				},
			})

			if exitCode != 0 {
				t.Fatalf("exit code = %d", exitCode)
			}
			if _, err := os.Stat(appDataDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("metadata command should not create app data dir, stat err = %v", err)
			}
		})
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

func TestRepoExecuteRejectsAppDataChildBeforeCreatingIt(t *testing.T) {
	repoPath := t.TempDir()
	appDataDir := filepath.Join(repoPath, "app-data")
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), Options{
		Args:    []string{"--app-data-dir", appDataDir, "repo", "execute", "--path", repoPath, "--step", "install", "--json"},
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
	if _, err := os.Stat(appDataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("app data child should not be created, stat err = %v", err)
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "invalid_app_data_dir" {
		t.Fatalf("error = %+v", payload.Error)
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

func TestRepoListJSONReturnsDomainResponseAndPassesIsolatedAppData(t *testing.T) {
	appDataDir := filepath.Join(t.TempDir(), "app-data")
	var stdout bytes.Buffer
	var gotConfig AppConfig
	fake := &fakeApp{
		listInstalledReposResp: domain.InstalledRepoManagerResponse{
			Repos: []domain.InstalledRepoSummary{{
				ID:             42,
				ProjectName:    "demo",
				LocalPath:      filepath.Clean(`C:\work\demo`),
				Status:         domain.InstalledRepoStatusAnalyzed,
				LastActivityAt: time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC),
			}},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "list", "--app-data-dir", appDataDir, "--json"},
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			gotConfig = config
			return fake, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if gotConfig.AppDataDir != filepath.Clean(appDataDir) {
		t.Fatalf("app data dir = %q", gotConfig.AppDataDir)
	}
	if _, err := os.Stat(appDataDir); err != nil {
		t.Fatalf("app data dir not created: %v", err)
	}
	if !fake.listInstalledReposCalled {
		t.Fatalf("ListInstalledRepos was not called")
	}
	var payload struct {
		OK   bool                                `json:"ok"`
		Data domain.InstalledRepoManagerResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if !payload.OK || len(payload.Data.Repos) != 1 || payload.Data.Repos[0].ID != 42 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestRepoDetailsHumanOutputIncludesSetupSessions(t *testing.T) {
	var stdout bytes.Buffer
	fake := &fakeApp{
		installedRepoDetailsResp: domain.InstalledRepoDetailsResponse{
			Repo: domain.InstalledRepoSummary{
				ID:          7,
				ProjectName: "demo",
				LocalPath:   filepath.Clean(`C:\work\demo`),
				Status:      domain.InstalledRepoStatusAnalyzed,
			},
			SetupSessions: []domain.SetupSessionSummary{{
				ID:              9,
				InstalledRepoID: 7,
				RepoPath:        filepath.Clean(`C:\work\demo`),
				Status:          domain.SetupSessionStatusSucceeded,
			}},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "details", "--id", "7"},
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
	if fake.installedRepoDetailsID != 7 {
		t.Fatalf("details id = %d", fake.installedRepoDetailsID)
	}
	output := stdout.String()
	for _, want := range []string{"Repo #7", "demo", "Setup sessions: 1", "succeeded"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, `"ok"`) {
		t.Fatalf("human output should not be JSON: %s", output)
	}
}

func TestRepoDiagnosticsJSONAcceptsPathOrIDAndReturnsDomainExport(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	for _, tc := range []struct {
		name string
		args []string
		want domain.RepoDiagnosticExportRequest
	}{
		{
			name: "path",
			args: []string{"repo", "diagnostics", "--path", repoPath, "--json"},
			want: domain.RepoDiagnosticExportRequest{LocalPath: repoPath},
		},
		{
			name: "id",
			args: []string{"repo", "diagnostics", "--id", "11", "--json"},
			want: domain.RepoDiagnosticExportRequest{InstalledRepoID: 11},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			fake := &fakeApp{
				exportRepoDiagnosticsResp: domain.RepoDiagnosticExport{
					SchemaVersion: "repo-diagnostic-export/v1",
					Repo: domain.RepoDiagnosticRepoIdentity{
						ID:        11,
						LocalPath: repoPath,
						Status:    domain.InstalledRepoStatusAnalyzed,
					},
					SetupSessions: []domain.RepoDiagnosticSetupSession{{
						ID: 3,
						Steps: []domain.RepoDiagnosticStep{{
							StepID: "install",
							Log:    "[REDACTED]",
						}},
					}},
				},
			}

			exitCode := Run(context.Background(), Options{
				Args:    tc.args,
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
			if fake.exportRepoDiagnosticsReq != tc.want {
				t.Fatalf("diagnostic req = %+v, want %+v", fake.exportRepoDiagnosticsReq, tc.want)
			}
			var payload struct {
				OK   bool                        `json:"ok"`
				Data domain.RepoDiagnosticExport `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
			}
			if !payload.OK || payload.Data.SchemaVersion == "" || payload.Data.Repo.ID != 11 {
				t.Fatalf("payload = %+v", payload)
			}
		})
	}
}

func TestRepoDiagnosticsMissingTargetJSONReturnsStructuredError(t *testing.T) {
	var stderr bytes.Buffer
	exitCode := Run(context.Background(), Options{
		Args:    []string{"repo", "diagnostics", "--json"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(AppConfig) (App, func() error, error) {
			t.Fatal("missing diagnostics target must not create app")
			return nil, nil, nil
		},
	})

	if exitCode == 0 {
		t.Fatalf("exit code = 0")
	}
	var payload struct {
		Error commandError `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
	}
	if payload.Error.Code != "missing_target" {
		t.Fatalf("error = %+v", payload.Error)
	}
}

func TestEnvVaultSaveUsesStdinAndDoesNotPrintSecretByDefault(t *testing.T) {
	const secretValue = "sk-command-secret-value"
	var stdout, stderr bytes.Buffer
	fake := &fakeApp{
		saveEnvVaultCredentialResp: domain.EnvVaultSaveResponse{
			Entry: domain.EnvVaultEntry{
				ID:                  5,
				Provider:            "openai",
				VariableName:        "OPENAI_API_KEY",
				DisplayName:         "OpenAI dev key",
				FingerprintFragment: "abc123def456",
				Status:              domain.EnvVaultStatusReady,
			},
		},
	}

	exitCode := Run(context.Background(), Options{
		Args:    []string{"env", "vault", "save", "--provider", "openai", "--variable", "OPENAI_API_KEY", "--display-name", "OpenAI dev key", "--stdin", "--json"},
		Stdin:   strings.NewReader(secretValue),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Version: defaultTestVersion(),
		NewApp: func(config AppConfig) (App, func() error, error) {
			return fake, nil, nil
		},
	})

	if exitCode != 0 {
		t.Fatalf("exit code = %d stderr=%s", exitCode, stderr.String())
	}
	if fake.saveEnvVaultCredentialReq.Value != secretValue || fake.saveEnvVaultCredentialReq.Provider != "openai" || fake.saveEnvVaultCredentialReq.VariableName != "OPENAI_API_KEY" {
		t.Fatalf("save req = %+v", fake.saveEnvVaultCredentialReq)
	}
	if strings.Contains(stdout.String(), secretValue) || strings.Contains(stderr.String(), secretValue) {
		t.Fatalf("secret leaked stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var payload struct {
		Data domain.EnvVaultSaveResponse `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
	}
	if payload.Data.Entry.ID != 5 || payload.Data.Entry.FingerprintFragment != "abc123def456" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestEnvVaultMetadataCommandsCallAppAndReturnValueFreeJSON(t *testing.T) {
	repoPath := filepath.Clean(t.TempDir())
	cases := []struct {
		name   string
		args   []string
		stdin  string
		assert func(t *testing.T, fake *fakeApp, output string)
	}{
		{
			name: "list",
			args: []string{"env", "vault", "list", "--json"},
			assert: func(t *testing.T, fake *fakeApp, output string) {
				t.Helper()
				if !fake.listEnvVaultEntriesCalled {
					t.Fatalf("ListEnvVaultEntries was not called")
				}
			},
		},
		{
			name:  "update",
			args:  []string{"env", "vault", "update", "--id", "5", "--display-name", "Renamed", "--stdin", "--json"},
			stdin: "sk-updated-secret-value",
			assert: func(t *testing.T, fake *fakeApp, output string) {
				t.Helper()
				if fake.updateEnvVaultEntryReq.EntryID != 5 || fake.updateEnvVaultEntryReq.DisplayName != "Renamed" || !fake.updateEnvVaultEntryReq.UpdateValue {
					t.Fatalf("update req = %+v", fake.updateEnvVaultEntryReq)
				}
				if strings.Contains(output, "sk-updated-secret-value") {
					t.Fatalf("updated secret leaked: %s", output)
				}
			},
		},
		{
			name: "remove",
			args: []string{"env", "vault", "remove", "--id", "5", "--json"},
			assert: func(t *testing.T, fake *fakeApp, output string) {
				t.Helper()
				if fake.removeEnvVaultEntryID != 5 {
					t.Fatalf("remove id = %d", fake.removeEnvVaultEntryID)
				}
			},
		},
		{
			name: "approve",
			args: []string{"env", "vault", "approve", "--id", "5", "--repo-path", repoPath, "--target", ".env", "--variable", "OPENAI_API_KEY", "--json"},
			assert: func(t *testing.T, fake *fakeApp, output string) {
				t.Helper()
				got := fake.approveEnvVaultEntryArg
				if got.EntryID != 5 || got.RepoPath != repoPath || got.TargetRelativePath != ".env" || got.VariableName != "OPENAI_API_KEY" {
					t.Fatalf("approval = %+v", got)
				}
			},
		},
		{
			name: "revoke",
			args: []string{"env", "vault", "revoke", "--approval-id", "8", "--json"},
			assert: func(t *testing.T, fake *fakeApp, output string) {
				t.Helper()
				if fake.revokeEnvVaultApprovalID != 8 {
					t.Fatalf("approval id = %d", fake.revokeEnvVaultApprovalID)
				}
			},
		},
		{
			name: "status",
			args: []string{"env", "vault", "status", "--id", "5", "--status", domain.EnvVaultStatusActionNeeded, "--json"},
			assert: func(t *testing.T, fake *fakeApp, output string) {
				t.Helper()
				if fake.markEnvVaultEntryStatusID != 5 || fake.markEnvVaultEntryStatusValue != domain.EnvVaultStatusActionNeeded {
					t.Fatalf("status args = %d %q", fake.markEnvVaultEntryStatusID, fake.markEnvVaultEntryStatusValue)
				}
			},
		},
		{
			name: "suppress",
			args: []string{"env", "vault", "suppress", "--repo-path", repoPath, "--target", ".env", "--variable", "OPENAI_API_KEY", "--json"},
			assert: func(t *testing.T, fake *fakeApp, output string) {
				t.Helper()
				got := fake.suppressEnvVaultPromptArg
				if got.RepoPath != repoPath || got.TargetRelativePath != ".env" || got.VariableName != "OPENAI_API_KEY" {
					t.Fatalf("suppression = %+v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			fake := &fakeApp{
				listEnvVaultEntriesResp: domain.EnvVaultManagerResponse{
					Entries: []domain.EnvVaultManagerEntry{{
						EnvVaultEntry: domain.EnvVaultEntry{
							ID:                  5,
							Provider:            "openai",
							VariableName:        "OPENAI_API_KEY",
							DisplayName:         "OpenAI dev key",
							FingerprintFragment: "abc123def456",
							Status:              domain.EnvVaultStatusReady,
						},
					}},
				},
				updateEnvVaultEntryResp: domain.EnvVaultSaveResponse{
					Entry: domain.EnvVaultEntry{ID: 5, DisplayName: "Renamed", FingerprintFragment: "def456abc123"},
				},
			}

			exitCode := Run(context.Background(), Options{
				Args:    tc.args,
				Stdin:   strings.NewReader(tc.stdin),
				Stdout:  &stdout,
				Stderr:  &stderr,
				Version: defaultTestVersion(),
				NewApp: func(config AppConfig) (App, func() error, error) {
					return fake, nil, nil
				},
			})

			if exitCode != 0 {
				t.Fatalf("exit code = %d stderr=%s", exitCode, stderr.String())
			}
			output := stdout.String() + stderr.String()
			if strings.Contains(output, "sk-updated-secret-value") {
				t.Fatalf("secret leaked: %s", output)
			}
			var payload struct {
				OK bool `json:"ok"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
			}
			if !payload.OK {
				t.Fatalf("ok = false")
			}
			tc.assert(t, fake, output)
		})
	}
}

func TestEnvVaultRevealRequiresConfirmationAndOnlyRevealPrintsValue(t *testing.T) {
	const secretValue = "sk-reveal-secret-value"
	t.Run("missing confirmation", func(t *testing.T) {
		var stderr bytes.Buffer
		fake := &fakeApp{revealEnvVaultEntryResp: domain.EnvVaultRevealResponse{EntryID: 5, Value: secretValue}}

		exitCode := Run(context.Background(), Options{
			Args:    []string{"env", "vault", "reveal", "--id", "5", "--json"},
			Stdout:  &bytes.Buffer{},
			Stderr:  &stderr,
			Version: defaultTestVersion(),
			NewApp: func(config AppConfig) (App, func() error, error) {
				return fake, nil, nil
			},
		})

		if exitCode == 0 {
			t.Fatalf("exit code = 0")
		}
		if fake.revealEnvVaultEntryReq.EntryID != 0 {
			t.Fatalf("reveal should not be called without confirmation")
		}
		if strings.Contains(stderr.String(), secretValue) {
			t.Fatalf("secret leaked: %s", stderr.String())
		}
		var payload struct {
			Error commandError `json:"error"`
		}
		if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
			t.Fatalf("decode stderr: %v\n%s", err, stderr.String())
		}
		if payload.Error.Code != "reveal_not_confirmed" {
			t.Fatalf("error = %+v", payload.Error)
		}
	})

	t.Run("confirmed JSON reveal", func(t *testing.T) {
		var stdout bytes.Buffer
		fake := &fakeApp{
			revealEnvVaultEntryResp: domain.EnvVaultRevealResponse{
				EntryID:     5,
				Value:       secretValue,
				RevealUntil: time.Date(2026, 5, 18, 12, 0, 30, 0, time.UTC),
			},
		}

		exitCode := Run(context.Background(), Options{
			Args:    []string{"env", "vault", "reveal", "--id", "5", "--confirm-reveal", "--json"},
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
		if fake.revealEnvVaultEntryReq.EntryID != 5 || !fake.revealEnvVaultEntryReq.Confirmed {
			t.Fatalf("reveal req = %+v", fake.revealEnvVaultEntryReq)
		}
		var payload struct {
			Data domain.EnvVaultRevealResponse `json:"data"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
		}
		if payload.Data.Value != secretValue {
			t.Fatalf("payload = %+v", payload)
		}
	})
}

func TestRepoListPersistsAnalyzeWithIsolatedAppData(t *testing.T) {
	appDataDir := filepath.Join(t.TempDir(), "app-data")
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte("module example.com/cli\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	run := func(args []string, out any) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exitCode := Run(context.Background(), Options{
			Args:    append(args, "--app-data-dir", appDataDir, "--json"),
			Stdout:  &stdout,
			Stderr:  &stderr,
			Version: defaultTestVersion(),
		})
		if exitCode != 0 {
			t.Fatalf("exit code = %d stderr=%s", exitCode, stderr.String())
		}
		if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
			t.Fatalf("decode stdout: %v\n%s", err, stdout.String())
		}
	}

	var analyzePayload struct {
		Data domain.AnalyzeResponse `json:"data"`
	}
	run([]string{"repo", "analyze", "--path", repoPath}, &analyzePayload)
	if analyzePayload.Data.Source.Path != repoPath {
		t.Fatalf("analyze source = %+v", analyzePayload.Data.Source)
	}

	var listPayload struct {
		Data domain.InstalledRepoManagerResponse `json:"data"`
	}
	run([]string{"repo", "list"}, &listPayload)
	if len(listPayload.Data.Repos) != 1 || listPayload.Data.Repos[0].LocalPath != repoPath {
		t.Fatalf("installed repos = %+v", listPayload.Data.Repos)
	}
}

type fakeApp struct {
	analyzeReq                         domain.AnalyzeRequest
	analyzeResp                        domain.AnalyzeResponse
	preflightReq                       domain.ClonePreflightRequest
	preflightResp                      domain.ClonePreflightResponse
	importRepoURL                      string
	importDestination                  string
	importResp                         domain.AnalyzeResponse
	importErr                          error
	executeReq                         domain.ExecuteRequest
	executeResp                        domain.ExecuteResponse
	executeErr                         error
	generateEnvDraftPath               string
	envDraftResp                       domain.EnvDraft
	saveStructuredPath                 string
	saveStructuredDraft                domain.EnvDraft
	saveStructuredResp                 domain.ExecuteResponse
	saveStructuredErr                  error
	saveRawPath                        string
	saveRawContent                     string
	saveRawResp                        domain.ExecuteResponse
	saveRawErr                         error
	envContributionSettingsCalled      bool
	envContributionSettingsResp        domain.EnvContributionSettingsResponse
	saveEnvContributionSettingsCalled  bool
	saveEnvContributionSettingsArg     domain.EnvContributionSettings
	saveEnvContributionSettingsResp    domain.EnvContributionSettingsResponse
	recordEnvContributionConsentCalled bool
	recordEnvContributionConsentArg    bool
	recordEnvContributionConsentResp   domain.EnvContributionSettingsResponse
	clearEnvContributionQueueCalled    bool
	clearEnvContributionQueueResp      domain.EnvContributionSettingsResponse
	aiEnvReviewSettingsCalled          bool
	aiEnvReviewSettingsResp            domain.AIEnvReviewSettings
	saveAIEnvReviewSettingsCalled      bool
	saveAIEnvReviewSettingsArg         domain.AIEnvReviewSettings
	saveAIEnvReviewSettingsResp        domain.AIEnvReviewSettings
	listInstalledReposCalled           bool
	listInstalledReposResp             domain.InstalledRepoManagerResponse
	installedRepoDetailsID             int64
	installedRepoDetailsResp           domain.InstalledRepoDetailsResponse
	exportRepoDiagnosticsReq           domain.RepoDiagnosticExportRequest
	exportRepoDiagnosticsResp          domain.RepoDiagnosticExport
	saveEnvVaultCredentialReq          domain.EnvVaultSaveRequest
	saveEnvVaultCredentialResp         domain.EnvVaultSaveResponse
	listEnvVaultEntriesCalled          bool
	listEnvVaultEntriesResp            domain.EnvVaultManagerResponse
	revealEnvVaultEntryReq             domain.EnvVaultRevealRequest
	revealEnvVaultEntryResp            domain.EnvVaultRevealResponse
	updateEnvVaultEntryReq             domain.EnvVaultUpdateRequest
	updateEnvVaultEntryResp            domain.EnvVaultSaveResponse
	removeEnvVaultEntryID              int64
	approveEnvVaultEntryArg            domain.EnvVaultApproval
	markEnvVaultEntryStatusID          int64
	markEnvVaultEntryStatusValue       string
	revokeEnvVaultApprovalID           int64
	suppressEnvVaultPromptArg          domain.EnvVaultPromptSuppression
}

func (f *fakeApp) Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.AnalyzeResponse, error) {
	f.analyzeReq = req
	return f.analyzeResp, nil
}

func (f *fakeApp) ClonePreflight(ctx context.Context, req domain.ClonePreflightRequest) (domain.ClonePreflightResponse, error) {
	f.preflightReq = req
	return f.preflightResp, nil
}

func (f *fakeApp) ImportRepository(ctx context.Context, repoURL, destinationRoot string) (domain.AnalyzeResponse, error) {
	f.importRepoURL = repoURL
	f.importDestination = destinationRoot
	if f.importErr != nil {
		return domain.AnalyzeResponse{}, f.importErr
	}
	return f.importResp, nil
}

func (f *fakeApp) Execute(ctx context.Context, req domain.ExecuteRequest) (domain.ExecuteResponse, error) {
	f.executeReq = req
	if f.executeErr != nil {
		return domain.ExecuteResponse{}, f.executeErr
	}
	return f.executeResp, nil
}

func (f *fakeApp) GenerateEnvDraft(ctx context.Context, localPath string) (domain.EnvDraft, error) {
	f.generateEnvDraftPath = localPath
	return f.envDraftResp, nil
}

func (f *fakeApp) SaveStructuredEnvDraft(ctx context.Context, localPath string, draft domain.EnvDraft) (domain.ExecuteResponse, error) {
	f.saveStructuredPath = localPath
	f.saveStructuredDraft = draft
	if f.saveStructuredErr != nil {
		return domain.ExecuteResponse{}, f.saveStructuredErr
	}
	return f.saveStructuredResp, nil
}

func (f *fakeApp) SaveRawEnv(ctx context.Context, localPath, content string) (domain.ExecuteResponse, error) {
	f.saveRawPath = localPath
	f.saveRawContent = content
	if f.saveRawErr != nil {
		return domain.ExecuteResponse{}, f.saveRawErr
	}
	return f.saveRawResp, nil
}

func (f *fakeApp) EnvContributionSettings(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	f.envContributionSettingsCalled = true
	return f.envContributionSettingsResp, nil
}

func (f *fakeApp) SaveEnvContributionSettings(ctx context.Context, settings domain.EnvContributionSettings) (domain.EnvContributionSettingsResponse, error) {
	f.saveEnvContributionSettingsCalled = true
	f.saveEnvContributionSettingsArg = settings
	return f.saveEnvContributionSettingsResp, nil
}

func (f *fakeApp) RecordEnvContributionConsent(ctx context.Context, publicEnabled bool) (domain.EnvContributionSettingsResponse, error) {
	f.recordEnvContributionConsentCalled = true
	f.recordEnvContributionConsentArg = publicEnabled
	return f.recordEnvContributionConsentResp, nil
}

func (f *fakeApp) ClearEnvContributionQueue(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	f.clearEnvContributionQueueCalled = true
	return f.clearEnvContributionQueueResp, nil
}

func (f *fakeApp) AIEnvReviewSettings(ctx context.Context) (domain.AIEnvReviewSettings, error) {
	f.aiEnvReviewSettingsCalled = true
	return f.aiEnvReviewSettingsResp, nil
}

func (f *fakeApp) SaveAIEnvReviewSettings(ctx context.Context, settings domain.AIEnvReviewSettings) (domain.AIEnvReviewSettings, error) {
	f.saveAIEnvReviewSettingsCalled = true
	f.saveAIEnvReviewSettingsArg = settings
	return f.saveAIEnvReviewSettingsResp, nil
}

func (f *fakeApp) ListInstalledRepos(ctx context.Context) (domain.InstalledRepoManagerResponse, error) {
	f.listInstalledReposCalled = true
	return f.listInstalledReposResp, nil
}

func (f *fakeApp) InstalledRepoDetails(ctx context.Context, installedRepoID int64) (domain.InstalledRepoDetailsResponse, error) {
	f.installedRepoDetailsID = installedRepoID
	return f.installedRepoDetailsResp, nil
}

func (f *fakeApp) ExportRepoDiagnostics(ctx context.Context, req domain.RepoDiagnosticExportRequest) (domain.RepoDiagnosticExport, error) {
	f.exportRepoDiagnosticsReq = req
	return f.exportRepoDiagnosticsResp, nil
}

func (f *fakeApp) SaveEnvVaultCredential(ctx context.Context, req domain.EnvVaultSaveRequest) (domain.EnvVaultSaveResponse, error) {
	f.saveEnvVaultCredentialReq = req
	return f.saveEnvVaultCredentialResp, nil
}

func (f *fakeApp) ListEnvVaultEntries(ctx context.Context) (domain.EnvVaultManagerResponse, error) {
	f.listEnvVaultEntriesCalled = true
	return f.listEnvVaultEntriesResp, nil
}

func (f *fakeApp) RevealEnvVaultEntry(ctx context.Context, req domain.EnvVaultRevealRequest) (domain.EnvVaultRevealResponse, error) {
	f.revealEnvVaultEntryReq = req
	return f.revealEnvVaultEntryResp, nil
}

func (f *fakeApp) UpdateEnvVaultEntry(ctx context.Context, req domain.EnvVaultUpdateRequest) (domain.EnvVaultSaveResponse, error) {
	f.updateEnvVaultEntryReq = req
	return f.updateEnvVaultEntryResp, nil
}

func (f *fakeApp) RemoveEnvVaultEntry(ctx context.Context, entryID int64) error {
	f.removeEnvVaultEntryID = entryID
	return nil
}

func (f *fakeApp) ApproveEnvVaultEntry(ctx context.Context, approval domain.EnvVaultApproval) error {
	f.approveEnvVaultEntryArg = approval
	return nil
}

func (f *fakeApp) MarkEnvVaultEntryStatus(ctx context.Context, entryID int64, status string) error {
	f.markEnvVaultEntryStatusID = entryID
	f.markEnvVaultEntryStatusValue = status
	return nil
}

func (f *fakeApp) RevokeEnvVaultApproval(ctx context.Context, approvalID int64) error {
	f.revokeEnvVaultApprovalID = approvalID
	return nil
}

func (f *fakeApp) SuppressEnvVaultPrompt(ctx context.Context, suppression domain.EnvVaultPromptSuppression) error {
	f.suppressEnvVaultPromptArg = suppression
	return nil
}

func defaultTestVersion() VersionInfo {
	return VersionInfo{
		AppVersion:            "dev",
		GitCommit:             "",
		CLIContractVersion:    CLIContractVersion,
		BridgeContractVersion: BridgeContractVersion,
	}
}
