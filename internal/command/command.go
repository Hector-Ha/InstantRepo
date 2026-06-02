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
	ListInstalledRepos(ctx context.Context) (domain.InstalledRepoManagerResponse, error)
	InstalledRepoDetails(ctx context.Context, installedRepoID int64) (domain.InstalledRepoDetailsResponse, error)
	ExportRepoDiagnostics(ctx context.Context, req domain.RepoDiagnosticExportRequest) (domain.RepoDiagnosticExport, error)
	SaveEnvVaultCredential(ctx context.Context, req domain.EnvVaultSaveRequest) (domain.EnvVaultSaveResponse, error)
	ListEnvVaultEntries(ctx context.Context) (domain.EnvVaultManagerResponse, error)
	RevealEnvVaultEntry(ctx context.Context, req domain.EnvVaultRevealRequest) (domain.EnvVaultRevealResponse, error)
	UpdateEnvVaultEntry(ctx context.Context, req domain.EnvVaultUpdateRequest) (domain.EnvVaultSaveResponse, error)
	RemoveEnvVaultEntry(ctx context.Context, entryID int64) error
	ApproveEnvVaultEntry(ctx context.Context, approval domain.EnvVaultApproval) error
	MarkEnvVaultEntryStatus(ctx context.Context, entryID int64, status string) error
	RevokeEnvVaultApproval(ctx context.Context, approvalID int64) error
	SuppressEnvVaultPrompt(ctx context.Context, suppression domain.EnvVaultPromptSuppression) error
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

func (a appWithServer) ListInstalledRepos(ctx context.Context) (domain.InstalledRepoManagerResponse, error) {
	return a.app.ListInstalledRepos(ctx)
}

func (a appWithServer) InstalledRepoDetails(ctx context.Context, installedRepoID int64) (domain.InstalledRepoDetailsResponse, error) {
	return a.app.InstalledRepoDetails(ctx, installedRepoID)
}

func (a appWithServer) ExportRepoDiagnostics(ctx context.Context, req domain.RepoDiagnosticExportRequest) (domain.RepoDiagnosticExport, error) {
	return a.app.ExportRepoDiagnostics(ctx, req)
}

func (a appWithServer) SaveEnvVaultCredential(ctx context.Context, req domain.EnvVaultSaveRequest) (domain.EnvVaultSaveResponse, error) {
	return a.app.SaveEnvVaultCredential(ctx, req)
}

func (a appWithServer) ListEnvVaultEntries(ctx context.Context) (domain.EnvVaultManagerResponse, error) {
	return a.app.ListEnvVaultEntries(ctx)
}

func (a appWithServer) RevealEnvVaultEntry(ctx context.Context, req domain.EnvVaultRevealRequest) (domain.EnvVaultRevealResponse, error) {
	return a.app.RevealEnvVaultEntry(ctx, req)
}

func (a appWithServer) UpdateEnvVaultEntry(ctx context.Context, req domain.EnvVaultUpdateRequest) (domain.EnvVaultSaveResponse, error) {
	return a.app.UpdateEnvVaultEntry(ctx, req)
}

func (a appWithServer) RemoveEnvVaultEntry(ctx context.Context, entryID int64) error {
	return a.app.RemoveEnvVaultEntry(ctx, entryID)
}

func (a appWithServer) ApproveEnvVaultEntry(ctx context.Context, approval domain.EnvVaultApproval) error {
	return a.app.ApproveEnvVaultEntry(ctx, approval)
}

func (a appWithServer) MarkEnvVaultEntryStatus(ctx context.Context, entryID int64, status string) error {
	return a.app.MarkEnvVaultEntryStatus(ctx, entryID, status)
}

func (a appWithServer) RevokeEnvVaultApproval(ctx context.Context, approvalID int64) error {
	return a.app.RevokeEnvVaultApproval(ctx, approvalID)
}

func (a appWithServer) SuppressEnvVaultPrompt(ctx context.Context, suppression domain.EnvVaultPromptSuppression) error {
	return a.app.SuppressEnvVaultPrompt(ctx, suppression)
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
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
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
	case "vault":
		return runEnvVault(ctx, opts, global, args[1:])
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
	if err := prepareAppDataDir(global.AppDataDir, *localPath); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, *localPath); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, *localPath); err != nil {
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

func runEnvVault(ctx context.Context, opts Options, global globalFlags, args []string) int {
	if len(args) == 0 {
		return writeCommandError(opts, commandError{Code: "missing_command", Message: "env vault command is required"}, global.JSON)
	}
	switch args[0] {
	case "list":
		return runEnvVaultList(ctx, opts, global, args[1:])
	case "save":
		return runEnvVaultSave(ctx, opts, global, args[1:])
	case "update":
		return runEnvVaultUpdate(ctx, opts, global, args[1:])
	case "remove":
		return runEnvVaultRemove(ctx, opts, global, args[1:])
	case "approve":
		return runEnvVaultApprove(ctx, opts, global, args[1:])
	case "revoke":
		return runEnvVaultRevoke(ctx, opts, global, args[1:])
	case "status":
		return runEnvVaultStatus(ctx, opts, global, args[1:])
	case "suppress":
		return runEnvVaultSuppress(ctx, opts, global, args[1:])
	case "reveal":
		return runEnvVaultReveal(ctx, opts, global, args[1:])
	default:
		return writeCommandError(opts, commandError{
			Code:    "unknown_command",
			Message: fmt.Sprintf("unknown env vault command %q", args[0]),
		}, global.JSON || hasJSONFlag(args[1:]))
	}
}

func runEnvVaultList(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.ListEnvVaultEntries(ctx)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_list_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultListSummary(opts.Stdout, resp)
}

func runEnvVaultSave(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	provider := fs.String("provider", "", "credential provider")
	variableName := fs.String("variable", "", "env variable name")
	displayName := fs.String("display-name", "", "credential display name")
	inputPath := fs.String("file", "", "credential value file")
	readStdin := fs.Bool("stdin", false, "read credential value from stdin")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*variableName) == "" {
		return writeCommandError(opts, commandError{Code: "missing_variable", Message: "--variable is required"}, *jsonOut)
	}
	raw, err := readCommandInput(opts, *inputPath, *readStdin, "credential value input")
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
	resp, err := app.SaveEnvVaultCredential(ctx, domain.EnvVaultSaveRequest{
		Provider:     *provider,
		VariableName: *variableName,
		DisplayName:  *displayName,
		Value:        string(raw),
	})
	if err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_save_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultSaveSummary(opts.Stdout, resp)
}

func runEnvVaultUpdate(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("id", "", "vault entry ID")
	displayName := fs.String("display-name", "", "credential display name")
	inputPath := fs.String("file", "", "credential value file")
	readStdin := fs.Bool("stdin", false, "read credential value from stdin")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	entryID, err := requiredPositiveInt64(*idRaw, "--id", "missing_id")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	updateValue := strings.TrimSpace(*inputPath) != "" || *readStdin
	if strings.TrimSpace(*displayName) == "" && !updateValue {
		return writeCommandError(opts, commandError{Code: "missing_update", Message: "--display-name, --file, or --stdin is required"}, *jsonOut)
	}
	req := domain.EnvVaultUpdateRequest{
		EntryID:     entryID,
		DisplayName: *displayName,
		UpdateValue: updateValue,
	}
	if updateValue {
		raw, err := readCommandInput(opts, *inputPath, *readStdin, "credential value input")
		if err != nil {
			return writeCommandError(opts, err, *jsonOut)
		}
		req.Value = string(raw)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.UpdateEnvVaultEntry(ctx, req)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_update_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultSaveSummary(opts.Stdout, resp)
}

func runEnvVaultRemove(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("id", "", "vault entry ID")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	entryID, err := requiredPositiveInt64(*idRaw, "--id", "missing_id")
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
	if err := app.RemoveEnvVaultEntry(ctx, entryID); err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_remove_failed", Message: err.Error()}, *jsonOut)
	}
	resp := envVaultActionResponse{Action: "remove", EntryID: entryID}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultActionSummary(opts.Stdout, resp)
}

func runEnvVaultApprove(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault approve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("id", "", "vault entry ID")
	repoPath := fs.String("repo-path", "", "local repository path")
	target := fs.String("target", "", "target env file relative path")
	variableName := fs.String("variable", "", "env variable name")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, *repoPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	entryID, err := requiredPositiveInt64(*idRaw, "--id", "missing_id")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	if strings.TrimSpace(*repoPath) == "" || strings.TrimSpace(*target) == "" || strings.TrimSpace(*variableName) == "" {
		return writeCommandError(opts, commandError{Code: "missing_approval_target", Message: "--repo-path, --target, and --variable are required"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	approval := domain.EnvVaultApproval{
		EntryID:            entryID,
		RepoPath:           *repoPath,
		TargetRelativePath: *target,
		VariableName:       *variableName,
	}
	if err := app.ApproveEnvVaultEntry(ctx, approval); err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_approve_failed", Message: err.Error()}, *jsonOut)
	}
	resp := envVaultActionResponse{Action: "approve", EntryID: entryID, RepoPath: *repoPath, TargetRelativePath: *target, VariableName: *variableName}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultActionSummary(opts.Stdout, resp)
}

func runEnvVaultRevoke(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault revoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("approval-id", "", "vault approval ID")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	approvalID, err := requiredPositiveInt64(*idRaw, "--approval-id", "missing_approval_id")
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
	if err := app.RevokeEnvVaultApproval(ctx, approvalID); err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_revoke_failed", Message: err.Error()}, *jsonOut)
	}
	resp := envVaultActionResponse{Action: "revoke", ApprovalID: approvalID}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultActionSummary(opts.Stdout, resp)
}

func runEnvVaultStatus(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("id", "", "vault entry ID")
	status := fs.String("status", "", "vault entry status")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	entryID, err := requiredPositiveInt64(*idRaw, "--id", "missing_id")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	if strings.TrimSpace(*status) == "" {
		return writeCommandError(opts, commandError{Code: "missing_status", Message: "--status is required"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	if err := app.MarkEnvVaultEntryStatus(ctx, entryID, *status); err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_status_failed", Message: err.Error()}, *jsonOut)
	}
	resp := envVaultActionResponse{Action: "status", EntryID: entryID, Status: *status}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultActionSummary(opts.Stdout, resp)
}

func runEnvVaultSuppress(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault suppress", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	repoPath := fs.String("repo-path", "", "local repository path")
	target := fs.String("target", "", "target env file relative path")
	variableName := fs.String("variable", "", "env variable name")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, *repoPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	if strings.TrimSpace(*repoPath) == "" || strings.TrimSpace(*target) == "" || strings.TrimSpace(*variableName) == "" {
		return writeCommandError(opts, commandError{Code: "missing_suppression_target", Message: "--repo-path, --target, and --variable are required"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	suppression := domain.EnvVaultPromptSuppression{
		RepoPath:           *repoPath,
		TargetRelativePath: *target,
		VariableName:       *variableName,
	}
	if err := app.SuppressEnvVaultPrompt(ctx, suppression); err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_suppress_failed", Message: err.Error()}, *jsonOut)
	}
	resp := envVaultActionResponse{Action: "suppress", RepoPath: *repoPath, TargetRelativePath: *target, VariableName: *variableName}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultActionSummary(opts.Stdout, resp)
}

func runEnvVaultReveal(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("env vault reveal", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("id", "", "vault entry ID")
	confirmReveal := fs.Bool("confirm-reveal", false, "confirm raw credential reveal")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	entryID, err := requiredPositiveInt64(*idRaw, "--id", "missing_id")
	if err != nil {
		return writeCommandError(opts, err, *jsonOut)
	}
	if !*confirmReveal {
		return writeCommandError(opts, commandError{Code: "reveal_not_confirmed", Message: "env vault reveal requires --confirm-reveal"}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.RevealEnvVaultEntry(ctx, domain.EnvVaultRevealRequest{EntryID: entryID, Confirmed: true})
	if err != nil {
		return writeCommandError(opts, commandError{Code: "env_vault_reveal_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeEnvVaultRevealSummary(opts.Stdout, resp)
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
	case "list":
		return runRepoList(ctx, opts, global, args[1:])
	case "details":
		return runRepoDetails(ctx, opts, global, args[1:])
	case "diagnostics":
		return runRepoDiagnostics(ctx, opts, global, args[1:])
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
	if err := prepareAppDataDir(global.AppDataDir, *localPath); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, targetPath); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, targetPath); err != nil {
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
	if err := prepareAppDataDir(global.AppDataDir, *localPath); err != nil {
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

func runRepoList(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("repo list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.ListInstalledRepos(ctx)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "repo_list_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeInstalledReposSummary(opts.Stdout, resp)
}

func runRepoDetails(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("repo details", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("id", "", "installed repo ID")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	if err := prepareAppDataDir(global.AppDataDir, ""); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	installedRepoID, err := requiredPositiveInt64(*idRaw, "--id", "missing_id")
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
	resp, err := app.InstalledRepoDetails(ctx, installedRepoID)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "repo_details_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeInstalledRepoDetailsSummary(opts.Stdout, resp)
}

func runRepoDiagnostics(ctx context.Context, opts Options, global globalFlags, args []string) int {
	fs := flag.NewFlagSet("repo diagnostics", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", global.JSON, "write JSON output")
	idRaw := fs.String("id", "", "installed repo ID")
	localPath := fs.String("path", "", "local repository path")
	appDataDir := fs.String("app-data-dir", global.AppDataDir, "app data directory")
	if err := fs.Parse(args); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: err.Error()}, *jsonOut)
	}
	global.AppDataDir = strings.TrimSpace(*appDataDir)
	idSet := strings.TrimSpace(*idRaw) != ""
	pathSet := strings.TrimSpace(*localPath) != ""
	if !idSet && !pathSet {
		return writeCommandError(opts, commandError{Code: "missing_target", Message: "--id or --path is required"}, *jsonOut)
	}
	if idSet && pathSet {
		return writeCommandError(opts, commandError{Code: "invalid_arguments", Message: "use either --id or --path, not both"}, *jsonOut)
	}
	if err := prepareAppDataDir(global.AppDataDir, *localPath); err != nil {
		return writeCommandError(opts, commandError{Code: "invalid_app_data_dir", Message: err.Error()}, *jsonOut)
	}
	req := domain.RepoDiagnosticExportRequest{LocalPath: *localPath}
	if idSet {
		installedRepoID, err := requiredPositiveInt64(*idRaw, "--id", "missing_id")
		if err != nil {
			return writeCommandError(opts, err, *jsonOut)
		}
		req = domain.RepoDiagnosticExportRequest{InstalledRepoID: installedRepoID}
	}
	app, cleanup, err := opts.NewApp(AppConfig{AppDataDir: cleanAppDataDir(global.AppDataDir)})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return writeCommandError(opts, commandError{Code: "app_init_failed", Message: err.Error()}, *jsonOut)
	}
	resp, err := app.ExportRepoDiagnostics(ctx, req)
	if err != nil {
		return writeCommandError(opts, commandError{Code: "diagnostics_failed", Message: err.Error()}, *jsonOut)
	}
	if *jsonOut {
		return writeJSON(opts.Stdout, successEnvelope{OK: true, Data: resp, Metadata: opts.Version})
	}
	return writeRepoDiagnosticsSummary(opts.Stdout, resp)
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
	if err := prepareAppDataDir(global.AppDataDir, *localPath); err != nil {
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

func requiredPositiveInt64(raw, flagName, missingCode string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, commandError{Code: missingCode, Message: fmt.Sprintf("%s is required", flagName)}
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, commandError{Code: "invalid_arguments", Message: fmt.Sprintf("%s must be a positive integer", flagName)}
	}
	return value, nil
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

type envVaultActionResponse struct {
	Action             string `json:"action"`
	EntryID            int64  `json:"entryId,omitempty"`
	ApprovalID         int64  `json:"approvalId,omitempty"`
	Status             string `json:"status,omitempty"`
	RepoPath           string `json:"repoPath,omitempty"`
	TargetRelativePath string `json:"targetRelativePath,omitempty"`
	VariableName       string `json:"variableName,omitempty"`
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

func writeInstalledReposSummary(w io.Writer, resp domain.InstalledRepoManagerResponse) int {
	_, _ = fmt.Fprintf(w, "Installed repos: %d\n", len(resp.Repos))
	for _, repo := range resp.Repos {
		_, _ = fmt.Fprintf(w, "- #%d %s [%s]\n", repo.ID, fallbackText(repo.ProjectName, "Installed Repo"), fallbackText(repo.Status, "unknown"))
		_, _ = fmt.Fprintf(w, "  Path: %s\n", repo.LocalPath)
		_, _ = fmt.Fprintf(w, "  Last activity: %s\n", formatSummaryTime(repo.LastActivityAt))
	}
	return 0
}

func writeInstalledRepoDetailsSummary(w io.Writer, resp domain.InstalledRepoDetailsResponse) int {
	repo := resp.Repo
	_, _ = fmt.Fprintf(w, "Repo #%d: %s\n", repo.ID, fallbackText(repo.ProjectName, "Installed Repo"))
	_, _ = fmt.Fprintf(w, "Path: %s\n", repo.LocalPath)
	_, _ = fmt.Fprintf(w, "Status: %s\n", fallbackText(repo.Status, "unknown"))
	_, _ = fmt.Fprintf(w, "Last analyzed: %s\n", formatSummaryTime(repo.LastAnalyzedAt))
	_, _ = fmt.Fprintf(w, "Last setup: %s\n", formatSummaryTime(repo.LastSetupAt))
	_, _ = fmt.Fprintf(w, "Setup sessions: %d\n", len(resp.SetupSessions))
	for _, session := range resp.SetupSessions {
		_, _ = fmt.Fprintf(w, "- #%d %s updated %s\n", session.ID, fallbackText(session.Status, "unknown"), formatSummaryTime(session.UpdatedAt))
	}
	return 0
}

func writeRepoDiagnosticsSummary(w io.Writer, export domain.RepoDiagnosticExport) int {
	_, _ = fmt.Fprintf(w, "Repo diagnostics: #%d %s\n", export.Repo.ID, export.Repo.LocalPath)
	_, _ = fmt.Fprintf(w, "Schema: %s\n", export.SchemaVersion)
	_, _ = fmt.Fprintf(w, "Project: %s (%s)\n", fallbackText(export.SetupPlan.ProjectName, export.Analysis.ProjectName, "unknown"), fallbackText(export.SetupPlan.ProjectType, export.Analysis.ProjectType, "unknown"))
	_, _ = fmt.Fprintf(w, "Plan steps: %d\n", len(export.SetupPlan.Steps))
	_, _ = fmt.Fprintf(w, "Setup sessions: %d\n", len(export.SetupSessions))
	_, _ = fmt.Fprintln(w, "Logs: redacted and truncated")
	return 0
}

func writeEnvVaultListSummary(w io.Writer, resp domain.EnvVaultManagerResponse) int {
	_, _ = fmt.Fprintf(w, "Vault entries: %d\n", len(resp.Entries))
	for _, entry := range resp.Entries {
		_, _ = fmt.Fprintf(w, "- #%d %s [%s]\n", entry.ID, fallbackText(entry.DisplayName, entry.VariableName, "credential"), fallbackText(entry.Status, "unknown"))
		_, _ = fmt.Fprintf(w, "  Provider: %s\n", entry.Provider)
		_, _ = fmt.Fprintf(w, "  Variable: %s\n", entry.VariableName)
		_, _ = fmt.Fprintf(w, "  Fingerprint: %s\n", entry.FingerprintFragment)
		_, _ = fmt.Fprintf(w, "  Uses: %d\n", entry.Usage.TotalUseCount)
		_, _ = fmt.Fprintf(w, "  Approvals: %d\n", len(entry.Approvals))
	}
	return 0
}

func writeEnvVaultSaveSummary(w io.Writer, resp domain.EnvVaultSaveResponse) int {
	entry := resp.Entry
	if resp.NeedsReview {
		_, _ = fmt.Fprintln(w, "Vault entry needs review")
		if resp.ReviewMessage != "" {
			_, _ = fmt.Fprintf(w, "Review: %s\n", resp.ReviewMessage)
		}
	} else {
		_, _ = fmt.Fprintln(w, "Vault entry saved")
	}
	if entry.ID != 0 {
		_, _ = fmt.Fprintf(w, "Entry ID: %d\n", entry.ID)
		_, _ = fmt.Fprintf(w, "Display name: %s\n", entry.DisplayName)
		_, _ = fmt.Fprintf(w, "Provider: %s\n", entry.Provider)
		_, _ = fmt.Fprintf(w, "Variable: %s\n", entry.VariableName)
		_, _ = fmt.Fprintf(w, "Status: %s\n", entry.Status)
		_, _ = fmt.Fprintf(w, "Fingerprint: %s\n", entry.FingerprintFragment)
	}
	return 0
}

func writeEnvVaultActionSummary(w io.Writer, resp envVaultActionResponse) int {
	_, _ = fmt.Fprintf(w, "Vault action: %s\n", resp.Action)
	if resp.EntryID != 0 {
		_, _ = fmt.Fprintf(w, "Entry ID: %d\n", resp.EntryID)
	}
	if resp.ApprovalID != 0 {
		_, _ = fmt.Fprintf(w, "Approval ID: %d\n", resp.ApprovalID)
	}
	if resp.Status != "" {
		_, _ = fmt.Fprintf(w, "Status: %s\n", resp.Status)
	}
	if resp.RepoPath != "" {
		_, _ = fmt.Fprintf(w, "Repo: %s\n", resp.RepoPath)
	}
	if resp.TargetRelativePath != "" {
		_, _ = fmt.Fprintf(w, "Target: %s\n", resp.TargetRelativePath)
	}
	if resp.VariableName != "" {
		_, _ = fmt.Fprintf(w, "Variable: %s\n", resp.VariableName)
	}
	return 0
}

func writeEnvVaultRevealSummary(w io.Writer, resp domain.EnvVaultRevealResponse) int {
	_, _ = fmt.Fprintln(w, "Credential reveal confirmed")
	_, _ = fmt.Fprintf(w, "Entry ID: %d\n", resp.EntryID)
	_, _ = fmt.Fprintf(w, "Reveal until: %s\n", formatSummaryTime(resp.RevealUntil))
	_, _ = fmt.Fprintf(w, "Value: %s\n", resp.Value)
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

func formatSummaryTime(value time.Time) string {
	if value.IsZero() {
		return "never"
	}
	return value.UTC().Format(time.RFC3339)
}
