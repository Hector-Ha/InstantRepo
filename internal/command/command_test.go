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
			CLIContractVersion: "2026-05-issue-37",
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
	if payload.Data.CLIContractVersion != "2026-05-issue-37" {
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
			CLIContractVersion: "2026-05-issue-37",
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
	if !strings.Contains(output, "InstantRepo 1.2.3") || !strings.Contains(output, "CLI contract 2026-05-issue-37") {
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

type fakeApp struct {
	analyzeReq           domain.AnalyzeRequest
	analyzeResp          domain.AnalyzeResponse
	preflightReq         domain.ClonePreflightRequest
	preflightResp        domain.ClonePreflightResponse
	importRepoURL        string
	importDestination    string
	importResp           domain.AnalyzeResponse
	importErr            error
	executeReq           domain.ExecuteRequest
	executeResp          domain.ExecuteResponse
	executeErr           error
	generateEnvDraftPath string
	envDraftResp         domain.EnvDraft
	saveStructuredPath   string
	saveStructuredDraft  domain.EnvDraft
	saveStructuredResp   domain.ExecuteResponse
	saveStructuredErr    error
	saveRawPath          string
	saveRawContent       string
	saveRawResp          domain.ExecuteResponse
	saveRawErr           error
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

func defaultTestVersion() VersionInfo {
	return VersionInfo{
		AppVersion:         "dev",
		GitCommit:          "",
		CLIContractVersion: CLIContractVersion,
	}
}
