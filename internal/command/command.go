package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"instantrepo/internal/api"
	"instantrepo/internal/contract"
	"instantrepo/internal/domain"
	"instantrepo/internal/service"
	"instantrepo/internal/store"
)

const (
	CLIContractVersion    = contract.CLIContractVersion
	BridgeContractVersion = contract.BridgeContractVersion
)

type VersionInfo struct {
	AppVersion            string `json:"appVersion"`
	GitCommit             string `json:"gitCommit,omitempty"`
	CLIContractVersion    string `json:"cliContractVersion"`
	BridgeContractVersion string `json:"bridgeContractVersion"`
}

type Options struct {
	Args    []string
	Environ []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Version VersionInfo
	NewApp  func(AppConfig) (App, func() error, error)
}

type AppConfig struct {
	AppDataDir string
}

type App interface {
	Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.AnalyzeResponse, error)
	ClonePreflight(ctx context.Context, req domain.ClonePreflightRequest) (domain.ClonePreflightResponse, error)
	ImportRepository(ctx context.Context, repoURL, destinationRoot string) (domain.AnalyzeResponse, error)
	Execute(ctx context.Context, req domain.ExecuteRequest) (domain.ExecuteResponse, error)
	GenerateEnvDraft(ctx context.Context, localPath string) (domain.EnvDraft, error)
	SaveStructuredEnvDraft(ctx context.Context, localPath string, draft domain.EnvDraft) (domain.ExecuteResponse, error)
	SaveRawEnv(ctx context.Context, localPath, content string) (domain.ExecuteResponse, error)
	EnvContributionSettings(ctx context.Context) (domain.EnvContributionSettingsResponse, error)
	SaveEnvContributionSettings(ctx context.Context, settings domain.EnvContributionSettings) (domain.EnvContributionSettingsResponse, error)
	RecordEnvContributionConsent(ctx context.Context, publicEnabled bool) (domain.EnvContributionSettingsResponse, error)
	ClearEnvContributionQueue(ctx context.Context) (domain.EnvContributionSettingsResponse, error)
	AIEnvReviewSettings(ctx context.Context) (domain.AIEnvReviewSettings, error)
	SaveAIEnvReviewSettings(ctx context.Context, settings domain.AIEnvReviewSettings) (domain.AIEnvReviewSettings, error)
}

type appWithServer struct {
	app *service.AppService
}

func (a appWithServer) Analyze(ctx context.Context, req domain.AnalyzeRequest) (domain.AnalyzeResponse, error) {
	return a.app.Analyze(ctx, req)
}

func (a appWithServer) Execute(ctx context.Context, req domain.ExecuteRequest) (domain.ExecuteResponse, error) {
	return a.app.Execute(ctx, req)
}

func (a appWithServer) ClonePreflight(ctx context.Context, req domain.ClonePreflightRequest) (domain.ClonePreflightResponse, error) {
	return a.app.ClonePreflight(ctx, req)
}

func (a appWithServer) ImportRepository(ctx context.Context, repoURL, destinationRoot string) (domain.AnalyzeResponse, error) {
	return a.app.ImportRepository(ctx, repoURL, destinationRoot)
}

func (a appWithServer) GenerateEnvDraft(ctx context.Context, localPath string) (domain.EnvDraft, error) {
	return a.app.GenerateEnvDraft(ctx, localPath)
}

func (a appWithServer) SaveStructuredEnvDraft(ctx context.Context, localPath string, draft domain.EnvDraft) (domain.ExecuteResponse, error) {
	return a.app.SaveStructuredEnvDraft(ctx, localPath, draft)
}

func (a appWithServer) SaveRawEnv(ctx context.Context, localPath, content string) (domain.ExecuteResponse, error) {
	return a.app.SaveRawEnv(ctx, localPath, content)
}

func (a appWithServer) EnvContributionSettings(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	return a.app.EnvContributionSettings(ctx)
}

func (a appWithServer) SaveEnvContributionSettings(ctx context.Context, settings domain.EnvContributionSettings) (domain.EnvContributionSettingsResponse, error) {
	return a.app.SaveEnvContributionSettings(ctx, settings)
}

func (a appWithServer) RecordEnvContributionConsent(ctx context.Context, publicEnabled bool) (domain.EnvContributionSettingsResponse, error) {
	return a.app.RecordEnvContributionConsent(ctx, publicEnabled)
}

func (a appWithServer) ClearEnvContributionQueue(ctx context.Context) (domain.EnvContributionSettingsResponse, error) {
	return a.app.ClearEnvContributionQueue(ctx)
}

func (a appWithServer) AIEnvReviewSettings(ctx context.Context) (domain.AIEnvReviewSettings, error) {
	return a.app.AIEnvReviewSettings(ctx)
}

func (a appWithServer) SaveAIEnvReviewSettings(ctx context.Context, settings domain.AIEnvReviewSettings) (domain.AIEnvReviewSettings, error) {
	return a.app.SaveAIEnvReviewSettings(ctx, settings)
}

func Run(ctx context.Context, opts Options) int {
	opts = withDefaults(opts)

	global, remaining, err := parseGlobalFlags(opts.Args, opts.Environ)
	if err != nil {
		return writeCommandError(opts, err, global.JSON)
	}

	if len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
		return runSubcommand(ctx, opts, global, remaining)
	}
	return runLegacy(ctx, opts, global, remaining)
}

func withDefaults(opts Options) Options {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Environ == nil {
		opts.Environ = os.Environ()
	}
	if opts.Version.AppVersion == "" {
		opts.Version.AppVersion = "dev"
	}
	if opts.Version.CLIContractVersion == "" {
		opts.Version.CLIContractVersion = CLIContractVersion
	}
	if opts.Version.BridgeContractVersion == "" {
		opts.Version.BridgeContractVersion = BridgeContractVersion
	}
	if opts.NewApp == nil {
		opts.NewApp = newServiceApp
	}
	return opts
}

func runSubcommand(ctx context.Context, opts Options, global globalFlags, args []string) int {
	name := args[0]
	switch name {
	case "env":
		return runEnv(ctx, opts, global, args[1:])
	case "repo":
		return runRepo(ctx, opts, global, args[1:])
	case "settings":
		return runSettings(ctx, opts, global, args[1:])
	case "shell":
		return runShell(opts, global, args[1:])
	case "version":
		return runVersion(opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown command %q", name),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runSettings(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "settings command is required"}, global.JSON)
	}
	switch args[0] {
	case "contribution":
		return runSettingsContribution(ctx, opts, global, args[1:])
	case "ai-env-review":
		return runSettingsAIEnvReview(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown settings command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runSettingsContribution(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "settings contribution command is required"}, global.JSON)
	}
	switch args[0] {
	case "get":
		return runSettingsContributionGet(ctx, opts, global, args[1:])
	case "save":
		return runSettingsContributionSave(ctx, opts, global, args[1:])
	case "consent":
		return runSettingsContributionConsent(ctx, opts, global, args[1:])
	case "clear-queue":
		return runSettingsContributionClearQueue(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown settings contribution command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runSettingsContributionGet(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("settings contribution get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.EnvContributionSettings(ctx)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "settings_contribution_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeContributionSettingsSummary(opts.Stdout, resp)
}

func runSettingsContributionSave(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("settings contribution save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	inputPath := fs.String("file", "", "settings JSON file")
	readStdin := fs.Bool("stdin", false, "read settings JSON from stdin")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	raw, err := readCommandInput(opts, *inputPath, *readStdin, "contribution settings input")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	var settings domain.EnvContributionSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_input", Message: fmt.Sprintf("decode contribution settings JSON: %v", err)}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.SaveEnvContributionSettings(ctx, settings)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "settings_contribution_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeContributionSettingsSummary(opts.Stdout, resp)
}

func runSettingsContributionConsent(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("settings contribution consent", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	publicEnabled := fs.String("public-enabled", "", "enable public env pattern contribution")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*publicEnabled) == "" {
		return writeCommandError(opts, commandError{Code: "missing_public_enabled", Message: "--public-enabled is required"}, *jsonOut)
	}
	enabled, err := strconv.ParseBool(*publicEnabled)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: "--public-enabled must be true or false"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.RecordEnvContributionConsent(ctx, enabled)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "settings_contribution_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeContributionSettingsSummary(opts.Stdout, resp)
}

func runSettingsContributionClearQueue(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("settings contribution clear-queue", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.ClearEnvContributionQueue(ctx)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "settings_contribution_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeContributionSettingsSummary(opts.Stdout, resp)
}

func runSettingsAIEnvReview(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "settings ai-env-review command is required"}, global.JSON)
	}
	switch args[0] {
	case "get":
		return runSettingsAIEnvReviewGet(ctx, opts, global, args[1:])
	case "save":
		return runSettingsAIEnvReviewSave(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown settings ai-env-review command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runSettingsAIEnvReviewGet(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("settings ai-env-review get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	settings, err := app.AIEnvReviewSettings(ctx)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "settings_ai_env_review_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: settings, Metadata: opts.Version})
	}
	return writeAIEnvReviewSettingsSummary(opts.Stdout, settings)
}

func runSettingsAIEnvReviewSave(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("settings ai-env-review save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	inputPath := fs.String("file", "", "settings JSON file")
	readStdin := fs.Bool("stdin", false, "read settings JSON from stdin")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	raw, err := readCommandInput(opts, *inputPath, *readStdin, "AI Env Review settings input")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	var settings domain.AIEnvReviewSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_input", Message: fmt.Sprintf("decode AI Env Review settings JSON: %v", err)}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	saved, err := app.SaveAIEnvReviewSettings(ctx, settings)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "settings_ai_env_review_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: saved, Metadata: opts.Version})
	}
	return writeAIEnvReviewSettingsSummary(opts.Stdout, saved)
}

func runShell(opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "shell command is required"}, global.JSON)
	}
	switch args[0] {
	case "info":
		return runShellInfo(opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown shell command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runShellInfo(opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("shell info", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	info := shellInfo(opts.Version)
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: info, Metadata: opts.Version})
	}
	return writeShellInfoSummary(opts.Stdout, info)
}

func runEnv(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "env command is required"}, global.JSON)
	}
	switch args[0] {
	case "draft":
		return runEnvDraft(ctx, opts, global, args[1:])
	case "raw":
		return runEnvRaw(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown env command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runEnvRaw(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "env raw command is required"}, global.JSON)
	}
	switch args[0] {
	case "save":
		return runEnvRawSave(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown env raw command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runEnvDraft(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "env draft command is required"}, global.JSON)
	}
	switch args[0] {
	case "generate":
		return runEnvDraftGenerate(ctx, opts, global, args[1:])
	case "save":
		return runEnvDraftSave(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown env draft command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runEnvDraftGenerate(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env draft generate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	localPath := fs.String("path", "", "local repository path")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, *localPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*localPath) == "" {
		return writeCommandError(opts, commandError{Code: "missing_path", Message: "local repository path is required"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	draft, err := app.GenerateEnvDraft(ctx, *localPath)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "env_draft_generate_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: draft, Metadata: opts.Version})
	}
	return writeEnvDraftSummary(opts.Stdout, draft)
}

func runEnvDraftSave(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env draft save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	localPath := fs.String("path", "", "local repository path")
	inputPath := fs.String("file", "", "draft JSON file")
	readStdin := fs.Bool("stdin", false, "read draft JSON from stdin")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, *localPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*localPath) == "" {
		return writeCommandError(opts, commandError{Code: "missing_path", Message: "local repository path is required"}, *jsonOut)
	}
	raw, err := readCommandInput(opts, *inputPath, *readStdin, "draft input")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	var draft domain.EnvDraft
	if err := json.Unmarshal(raw, &draft); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_input", Message: fmt.Sprintf("decode draft JSON: %v", err)}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.SaveStructuredEnvDraft(ctx, *localPath, draft)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "env_draft_save_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeExecuteSummary(opts.Stdout, resp)
}

func runEnvRawSave(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env raw save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	localPath := fs.String("path", "", "local repository path")
	inputPath := fs.String("file", "", "raw env file")
	readStdin := fs.Bool("stdin", false, "read raw env content from stdin")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, *localPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*localPath) == "" {
		return writeCommandError(opts, commandError{Code: "missing_path", Message: "local repository path is required"}, *jsonOut)
	}
	raw, err := readCommandInput(opts, *inputPath, *readStdin, "raw env input")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.SaveRawEnv(ctx, *localPath, string(raw))
	if err != nil {
		return writeCommandError(opts, commandError{Code: "env_raw_save_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeExecuteSummary(opts.Stdout, resp)
}

func runRepo(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "repo command is required"}, global.JSON)
	}
	switch args[0] {
	case "analyze":
		return runRepoAnalyze(ctx, opts, global, args[1:])
	case "preflight":
		return runRepoPreflight(ctx, opts, global, args[1:])
	case "import", "clone":
		return runRepoImport(ctx, opts, global, args[1:])
	case "execute":
		return runRepoExecute(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown repo command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runRepoAnalyze(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("repo analyze", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	repoURL := fs.String("repo", "", "repository URL to analyze")
	localPath := fs.String("path", "", "local repository path to analyze")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, *localPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*repoURL) == "" && strings.TrimSpace(*localPath) == "" {
		return writeCommandError(opts, commandError{Code: "missing_target", Message: "repo URL or local path is required"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.Analyze(ctx, domain.AnalyzeRequest{
		RepoURL:   *repoURL,
		LocalPath: *localPath,
	})
	if err != nil {
		return writeCommandError(opts, commandError{Code: "analyze_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeAnalyzeSummary(opts.Stdout, resp)
}

func runRepoPreflight(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("repo preflight", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	repoURL := fs.String("repo", "", "repository URL to check")
	destination := fs.String("destination", "", "destination folder")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if strings.TrimSpace(*repoURL) == "" {
		return writeCommandError(opts, commandError{Code: "missing_repo", Message: "repo URL is required"}, *jsonOut)
	}
	if strings.TrimSpace(*destination) == "" {
		return writeCommandError(opts, commandError{Code: "missing_destination", Message: "destination folder is required"}, *jsonOut)
	}
	targetPath, err := service.CloneTargetPath(*repoURL, *destination)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: fmt.Sprintf("resolve clone target: %v", err)}, *jsonOut)
	}
	if err := validateAppDataDir(global.AppDataDir, targetPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.ClonePreflight(ctx, domain.ClonePreflightRequest{
		RepoURL:         *repoURL,
		DestinationRoot: *destination,
	})
	if err != nil {
		return writeCommandError(opts, commandError{Code: "preflight_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writePreflightSummary(opts.Stdout, resp)
}

func runRepoImport(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("repo import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	repoURL := fs.String("repo", "", "repository URL to import")
	destination := fs.String("destination", "", "destination folder")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if strings.TrimSpace(*repoURL) == "" {
		return writeCommandError(opts, commandError{Code: "missing_repo", Message: "repo URL is required"}, *jsonOut)
	}
	if strings.TrimSpace(*destination) == "" {
		return writeCommandError(opts, commandError{Code: "missing_destination", Message: "destination folder is required"}, *jsonOut)
	}
	targetPath, err := service.CloneTargetPath(*repoURL, *destination)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: fmt.Sprintf("resolve clone target: %v", err)}, *jsonOut)
	}
	if err := validateAppDataDir(global.AppDataDir, targetPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.ImportRepository(ctx, *repoURL, *destination)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "import_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeImportSummary(opts.Stdout, resp)
}

func runRepoExecute(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("repo execute", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	repoURL := fs.String("repo", "", "repository URL")
	localPath := fs.String("path", "", "local repository path")
	stepID := fs.String("step", "", "setup step ID")
	approve := fs.Bool("approve", false, "approve risky setup step")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, *localPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*stepID) == "" {
		return writeCommandError(opts, commandError{Code: "missing_step", Message: "step ID is required"}, *jsonOut)
	}
	if strings.TrimSpace(*repoURL) == "" && strings.TrimSpace(*localPath) == "" {
		return writeCommandError(opts, commandError{Code: "missing_target", Message: "repo URL or local path is required"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.Execute(ctx, domain.ExecuteRequest{
		RepoURL:      *repoURL,
		LocalPath:    *localPath,
		StepID:       *stepID,
		ApproveRisky: *approve,
	})
	if err != nil {
		return writeCommandError(opts, commandError{Code: "execute_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeExecuteSummary(opts.Stdout, resp)
}

func runVersion(opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: opts.Version, Metadata: opts.Version})
	}
	_, _ = fmt.Fprintf(opts.Stdout, "InstantRepo %s\nCLI contract %s\n", opts.Version.AppVersion, opts.Version.CLIContractVersion)
	_, _ = fmt.Fprintf(opts.Stdout, "Bridge contract %s\n", opts.Version.BridgeContractVersion)
	if opts.Version.GitCommit != "" {
		_, _ = fmt.Fprintf(opts.Stdout, "Git commit %s\n", opts.Version.GitCommit)
	}
	return 0
}

func runLegacy(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("instantrepo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	serveAddr := fs.String("serve", "", "HTTP listen address, for example :8080")
	repoURL := fs.String("repo", "", "GitHub repository URL to analyze")
	localPath := fs.String("path", "", "Local repository path to analyze")
	stepID := fs.String("step", "", "Plan step ID to execute after analysis")
	approve := fs.Bool("approve", false, "Allow execution of risky steps that require approval")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, global.JSON)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := validateAppDataDir(global.AppDataDir, *localPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, global.JSON)
	}

	if *repoURL == "" && *localPath == "" && *serveAddr == "" {
		if !global.JSON {
			_, _ = fmt.Fprintln(opts.Stderr, "usage: instantrepo [-serve addr] [-repo url | -path path] [-step id] [-approve]")
			return 1
		}
		return writeCommandError(opts, commandError{
			Code:    "missing_target",
			Message: "repo URL, local path, or serve address is required",
		}, global.JSON)
	}

	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, global.JSON)
	}

	if *serveAddr != "" {
		svc, ok := app.(interface{ service() *service.AppService })
		if !ok {
			return writeCommandError(opts, commandError{Code: "server_unavailable", Message: "server requires AppService"}, global.JSON)
		}
		server := &http.Server{
			Addr:              *serveAddr,
			Handler:           api.NewServer(svc.service()),
			ReadHeaderTimeout: 10 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
			return writeCommandError(opts, commandError{Code: "server_failed", Message: err.Error()}, global.JSON)
		}
		return 0
	}

	enc := json.NewEncoder(opts.Stdout)
	enc.SetIndent("", "  ")
	if *stepID != "" {
		resp, err := app.Execute(ctx, domain.ExecuteRequest{
			RepoURL:      *repoURL,
			LocalPath:    *localPath,
			StepID:       *stepID,
			ApproveRisky: *approve,
		})
		if err != nil {
			return writeCommandError(opts, commandError{Code: "execute_failed", Message: err.Error()}, global.JSON)
		}
		if err := enc.Encode(resp); err != nil {
			return writeCommandError(opts, commandError{Code: "encode_failed", Message: err.Error()}, global.JSON)
		}
		return 0
	}

	resp, err := app.Analyze(ctx, domain.AnalyzeRequest{
		RepoURL:   *repoURL,
		LocalPath: *localPath,
	})
	if err != nil {
		return writeCommandError(opts, commandError{Code: "analyze_failed", Message: err.Error()}, global.JSON)
	}
	if err := enc.Encode(resp); err != nil {
		return writeCommandError(opts, commandError{Code: "encode_failed", Message: err.Error()}, global.JSON)
	}
	return 0
}

type serviceApp struct {
	*service.AppService
}

func (a serviceApp) service() *service.AppService {
	return a.AppService
}

func newServiceApp(config AppConfig) (App, func() error, error) {
	if strings.TrimSpace(config.AppDataDir) == "" {
		app, err := service.NewAppServiceWithDefaultStore()
		if err != nil {
			return nil, nil, err
		}
		return serviceApp{AppService: app}, nil, nil
	}
	dbPath, err := store.DatabasePathForAppDataDir(config.AppDataDir)
	if err != nil {
		return nil, nil, err
	}
	sqliteStore, err := store.OpenSQLiteStore(dbPath)
	if err != nil {
		return nil, nil, err
	}
	return serviceApp{AppService: service.NewAppServiceWithInstalledRepoStore(sqliteStore)}, sqliteStore.Close, nil
}

type globalFlags struct {
	JSON           bool
	AppDataDir     string
	TargetRepoPath string
}

func parseGlobalFlags(args, environ []string) (globalFlags, []string, error) {
	global := globalFlags{AppDataDir: envValue(environ, "INSTANTREPO_APP_DATA_DIR")}
	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			global.JSON = true
		case arg == "--app-data-dir":
			i++
			if i >= len(args) {
				return global, nil, commandError{Code: "invalid_arguments", Message: "--app-data-dir requires a value"}
			}
			global.AppDataDir = args[i]
		case strings.HasPrefix(arg, "--app-data-dir="):
			global.AppDataDir = strings.TrimPrefix(arg, "--app-data-dir=")
		case arg == "-path":
			remaining = append(remaining, arg)
			i++
			if i >= len(args) {
				return global, nil, commandError{Code: "invalid_arguments", Message: "-path requires a value"}
			}
			global.TargetRepoPath = args[i]
			remaining = append(remaining, args[i])
		case strings.HasPrefix(arg, "-path="):
			global.TargetRepoPath = strings.TrimPrefix(arg, "-path=")
			remaining = append(remaining, arg)
		default:
			remaining = append(remaining, arg)
		}
	}
	return global, remaining, nil
}

func envValue(environ []string, key string) string {
	for _, item := range environ {
		name, value, ok := strings.Cut(item, "=")
		if ok && strings.EqualFold(name, key) {
			return value
		}
	}
	return ""
}

func hasJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}

func validateAppDataDir(appDataDir, targetRepoPath string) error {
	appDataDir = strings.TrimSpace(appDataDir)
	if appDataDir == "" {
		return nil
	}
	if !filepath.IsAbs(appDataDir) {
		return fmt.Errorf("app data dir must be absolute")
	}
	cleanAppData, err := filepath.Abs(appDataDir)
	if err != nil {
		return fmt.Errorf("resolve app data dir: %w", err)
	}
	cleanAppData = filepath.Clean(cleanAppData)
	if filepath.Dir(cleanAppData) == cleanAppData {
		return fmt.Errorf("app data dir must not be filesystem root")
	}
	if volume := filepath.VolumeName(cleanAppData); volume != "" && strings.EqualFold(cleanAppData, volume+string(os.PathSeparator)) {
		return fmt.Errorf("app data dir must not be drive root")
	}
	homeDir, err := os.UserHomeDir()
	if err == nil && samePath(cleanAppData, homeDir) {
		return fmt.Errorf("app data dir must not be home dir")
	}
	repoRoot, err := currentSourceRepoRoot()
	if err == nil && samePath(cleanAppData, repoRoot) {
		return fmt.Errorf("app data dir must not be repo root")
	}
	if strings.TrimSpace(targetRepoPath) != "" {
		repoAbs, err := filepath.Abs(targetRepoPath)
		if err != nil {
			return fmt.Errorf("resolve target repo path: %w", err)
		}
		repoAbs = filepath.Clean(repoAbs)
		if samePath(cleanAppData, repoAbs) || isPathInside(cleanAppData, repoAbs) {
			return fmt.Errorf("app data dir must not be target repo or inside target repo")
		}
	}
	if _, err := store.DatabasePathForAppDataDir(cleanAppData); err != nil {
		return err
	}
	if err := os.MkdirAll(cleanAppData, 0o700); err != nil {
		return fmt.Errorf("create app data dir: %w", err)
	}
	return nil
}

func cleanAppDataDir(appDataDir string) string {
	if strings.TrimSpace(appDataDir) == "" {
		return ""
	}
	abs, err := filepath.Abs(appDataDir)
	if err != nil {
		return filepath.Clean(appDataDir)
	}
	return filepath.Clean(abs)
}

func currentSourceRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Clean(wd)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA == nil {
		a = aa
	}
	if errB == nil {
		b = bb
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func isPathInside(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

type commandError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e commandError) Error() string {
	return e.Message
}

type successEnvelope struct {
	OK       bool        `json:"ok"`
	Data     any         `json:"data"`
	Metadata VersionInfo `json:"metadata"`
}

type errorEnvelope struct {
	OK       bool         `json:"ok"`
	Error    commandError `json:"error"`
	Metadata VersionInfo  `json:"metadata"`
}

func writeCommandError(opts Options, err error, jsonOut bool) int {
	var cmdErr commandError
	if !errors.As(err, &cmdErr) {
		cmdErr = commandError{Code: "command_failed", Message: err.Error()}
	}
	if jsonOut {
		if code := writeJSON(opts.Stderr, errorEnvelope{OK: false, Error: cmdErr, Metadata: opts.Version}); code != 0 {
			return code
		}
	} else {
		_, _ = fmt.Fprintln(opts.Stderr, cmdErr.Message)
	}
	return 1
}

func readCommandInput(opts Options, inputPath string, readStdin bool, label string) ([]byte, error) {
	inputPath = strings.TrimSpace(inputPath)
	if inputPath != "" && readStdin {
		return nil, commandError{Code: "invalid_arguments", Message: fmt.Sprintf("%s accepts either --file or --stdin, not both", label)}
	}
	if inputPath == "" && !readStdin {
		return nil, commandError{Code: "missing_input", Message: fmt.Sprintf("%s file or --stdin is required", label)}
	}
	if readStdin {
		raw, err := io.ReadAll(opts.Stdin)
		if err != nil {
			return nil, commandError{Code: "invalid_input", Message: fmt.Sprintf("read %s from stdin: %v", label, err)}
		}
		return raw, nil
	}
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, commandError{Code: "invalid_input", Message: fmt.Sprintf("read %s: %v", label, err)}
	}
	return raw, nil
}

func writeJSON(w io.Writer, payload any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}

func writeAnalyzeSummary(w io.Writer, resp domain.AnalyzeResponse) int {
	source := resp.Source.Path
	if source == "" {
		source = resp.Source.RepoURL
	}
	_, _ = fmt.Fprintf(w, "Repo: %s\n", source)
	_, _ = fmt.Fprintf(w, "Project: %s (%s)\n", fallbackText(resp.Plan.ProjectName, resp.Analysis.ProjectName, "unknown"), fallbackText(resp.Plan.ProjectType, resp.Analysis.ProjectType, "unknown"))
	_, _ = fmt.Fprintf(w, "Setup steps: %d\n", len(resp.Plan.Steps))
	_, _ = fmt.Fprintf(w, "Attention: %d\n", len(resp.Plan.Gaps)+len(resp.Plan.Safety.Findings))
	return 0
}

func writePreflightSummary(w io.Writer, resp domain.ClonePreflightResponse) int {
	_, _ = fmt.Fprintf(w, "Action: %s\n", fallbackText(resp.RecommendedAction, "unknown"))
	_, _ = fmt.Fprintf(w, "Target: %s\n", resp.TargetPath)
	for _, message := range resp.Messages {
		_, _ = fmt.Fprintf(w, "%s: %s\n", fallbackText(message.Severity, "info"), message.Text)
	}
	return 0
}

func writeImportSummary(w io.Writer, resp domain.AnalyzeResponse) int {
	_, _ = fmt.Fprintf(w, "Imported: %s\n", resp.Source.Path)
	_, _ = fmt.Fprintf(w, "Setup steps: %d\n", len(resp.Plan.Steps))
	return 0
}

func writeExecuteSummary(w io.Writer, resp domain.ExecuteResponse) int {
	status := "failed"
	if resp.Result.Succeeded {
		status = "succeeded"
	}
	_, _ = fmt.Fprintf(w, "Step: %s\n", resp.Result.StepID)
	_, _ = fmt.Fprintf(w, "Status: %s\n", status)
	_, _ = fmt.Fprintf(w, "Exit code: %d\n", resp.Result.ExitCode)
	writeShortOutput(w, "Stdout", resp.Result.Stdout)
	writeShortOutput(w, "Stderr", resp.Result.Stderr)
	return 0
}

func writeEnvDraftSummary(w io.Writer, draft domain.EnvDraft) int {
	serviceCredentials := 0
	actionNeeded := 0
	_, _ = fmt.Fprintf(w, "Repo: %s\n", draft.RepoPath)
	_, _ = fmt.Fprintf(w, "Env targets: %d\n", len(draft.Targets))
	for _, target := range draft.Targets {
		for _, value := range target.Values {
			if value.ValueClass == domain.EnvValueClassServiceCredential {
				serviceCredentials++
			}
			if len(value.Attention) > 0 {
				actionNeeded++
			}
		}
		_, _ = fmt.Fprintf(w, "- %s: %d values\n", target.RelativePath, len(target.Values))
	}
	_, _ = fmt.Fprintf(w, "Service credentials: %d\n", serviceCredentials)
	_, _ = fmt.Fprintf(w, "Action needed: %d\n", actionNeeded)
	return 0
}

func writeContributionSettingsSummary(w io.Writer, resp domain.EnvContributionSettingsResponse) int {
	_, _ = fmt.Fprintf(w, "Public env patterns: %t\n", resp.Settings.PublicEnvPatternsEnabled)
	_, _ = fmt.Fprintf(w, "Private/local env patterns: %t\n", resp.Settings.PrivateLocalEnvPatternsEnabled)
	_, _ = fmt.Fprintf(w, "Consent shown: %t\n", resp.Settings.ConsentShown)
	_, _ = fmt.Fprintf(w, "Queued contributions: %d\n", resp.Queue.Count)
	return 0
}

func writeAIEnvReviewSettingsSummary(w io.Writer, settings domain.AIEnvReviewSettings) int {
	_, _ = fmt.Fprintf(w, "AI Env Review enabled: %t\n", settings.Enabled)
	return 0
}

func shellInfo(version VersionInfo) map[string]string {
	info := map[string]string{
		"shell":                 "cli",
		"frontend":              "none",
		"backend":               "go-service-layer",
		"adapter":               "command",
		"appVersion":            version.AppVersion,
		"cliContractVersion":    version.CLIContractVersion,
		"bridgeContractVersion": version.BridgeContractVersion,
	}
	if strings.TrimSpace(version.GitCommit) != "" {
		info["gitCommit"] = version.GitCommit
	}
	return info
}

func writeShellInfoSummary(w io.Writer, info map[string]string) int {
	_, _ = fmt.Fprintf(w, "Shell: %s\n", fallbackText(info["shell"], "unknown"))
	_, _ = fmt.Fprintf(w, "Backend: %s\n", fallbackText(info["backend"], "unknown"))
	_, _ = fmt.Fprintf(w, "CLI contract: %s\n", info["cliContractVersion"])
	_, _ = fmt.Fprintf(w, "Bridge contract: %s\n", info["bridgeContractVersion"])
	return 0
}

func writeShortOutput(w io.Writer, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	lines := strings.Split(value, "\n")
	if len(lines) > 3 {
		lines = append(lines[:3], "...")
	}
	_, _ = fmt.Fprintf(w, "%s:\n%s\n", label, strings.Join(lines, "\n"))
}

func fallbackText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
