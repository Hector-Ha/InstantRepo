package domain

import "time"

type AnalyzeRequest struct {
	RepoURL   string `json:"repoUrl,omitempty"`
	LocalPath string `json:"localPath,omitempty"`
}

type ExecuteRequest struct {
	RepoURL       string `json:"repoUrl,omitempty"`
	LocalPath     string `json:"localPath,omitempty"`
	StepID        string `json:"stepId"`
	ApproveRisky  bool   `json:"approveRisky"`
	ExecutionMode string `json:"executionMode,omitempty"`
}

type AnalyzeResponse struct {
	Source      RepoSource         `json:"source"`
	Analysis    RepositoryAnalysis `json:"analysis"`
	Environment EnvironmentReport  `json:"environment"`
	Plan        SetupPlan          `json:"plan"`
}

type ClonePreflightRequest struct {
	RepoURL         string `json:"repoUrl"`
	DestinationRoot string `json:"destinationRoot"`
}

type ClonePreflightResponse struct {
	RepoURL             string                  `json:"repoUrl"`
	NormalizedURL       string                  `json:"normalizedUrl"`
	DestinationRoot     string                  `json:"destinationRoot"`
	DestinationWritable bool                    `json:"destinationWritable"`
	TargetPath          string                  `json:"targetPath"`
	TargetExists        bool                    `json:"targetExists"`
	TargetEmpty         bool                    `json:"targetEmpty"`
	DuplicateRepos      []InstalledRepo         `json:"duplicateRepos"`
	PathConflict        bool                    `json:"pathConflict"`
	PathConflictRepos   []InstalledRepo         `json:"pathConflictRepos"`
	Disk                CloneDiskStatus         `json:"disk"`
	RecommendedAction   string                  `json:"recommendedAction"`
	Messages            []ClonePreflightMessage `json:"messages"`
}

type CloneDiskStatus struct {
	Status    string `json:"status"`
	FreeBytes uint64 `json:"freeBytes,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ClonePreflightMessage struct {
	Severity string `json:"severity"`
	Text     string `json:"text"`
}

const (
	CloneDiskStatusOK    = "ok"
	CloneDiskStatusWarn  = "warn"
	CloneDiskStatusBlock = "block"
)

const (
	CloneActionClone                 = "clone"
	CloneActionCloneWithAttention    = "clone-with-attention"
	CloneActionOpenExisting          = "open-existing"
	CloneActionChooseDifferentFolder = "choose-different-folder"
	CloneActionFreeDiskSpace         = "free-disk-space"
)

type InstalledRepo struct {
	ID             int64     `json:"id"`
	RawURL         string    `json:"rawUrl,omitempty"`
	NormalizedURL  string    `json:"normalizedUrl,omitempty"`
	LocalPath      string    `json:"localPath"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	LastAnalyzedAt time.Time `json:"lastAnalyzedAt"`
}

const (
	InstalledRepoStatusAnalyzed = "analyzed"
)

type InstalledRepoManagerResponse struct {
	Repos []InstalledRepoSummary `json:"repos"`
}

type InstalledRepoDetailsResponse struct {
	Repo          InstalledRepoSummary  `json:"repo"`
	SetupSessions []SetupSessionSummary `json:"setupSessions"`
}

type InstalledRepoSummary struct {
	ID              int64     `json:"id"`
	ProjectName     string    `json:"projectName"`
	LocalPath       string    `json:"localPath"`
	LocalPathExists bool      `json:"localPathExists"`
	Status          string    `json:"status"`
	LastAnalyzedAt  time.Time `json:"lastAnalyzedAt"`
	LastSetupAt     time.Time `json:"lastSetupAt"`
	LastActivityAt  time.Time `json:"lastActivityAt"`
}

type SetupSessionSummary struct {
	ID              int64     `json:"id"`
	InstalledRepoID int64     `json:"installedRepoId"`
	RepoPath        string    `json:"repoPath"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type RepoDiagnosticExportRequest struct {
	InstalledRepoID int64  `json:"installedRepoId,omitempty"`
	LocalPath       string `json:"localPath,omitempty"`
}

type RepoDiagnosticExport struct {
	SchemaVersion string                         `json:"schemaVersion"`
	GeneratedAt   time.Time                      `json:"generatedAt"`
	Repo          RepoDiagnosticRepoIdentity     `json:"repo"`
	App           RepoDiagnosticAppInfo          `json:"app"`
	Environment   RepoDiagnosticEnvironment      `json:"environment"`
	Analysis      RepoDiagnosticAnalysisSummary  `json:"analysis"`
	SetupPlan     RepoDiagnosticSetupPlanSummary `json:"setupPlan"`
	SetupSessions []RepoDiagnosticSetupSession   `json:"setupSessions"`
	AIReview      RepoDiagnosticAIReviewMetadata `json:"aiReview"`
}

type RepoDiagnosticRepoIdentity struct {
	ID             int64     `json:"id"`
	RawURL         string    `json:"rawUrl,omitempty"`
	NormalizedURL  string    `json:"normalizedUrl,omitempty"`
	LocalPath      string    `json:"localPath"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	LastAnalyzedAt time.Time `json:"lastAnalyzedAt"`
}

type RepoDiagnosticAppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RepoDiagnosticEnvironment struct {
	OS    string         `json:"os"`
	Arch  string         `json:"arch"`
	Tools []DetectedTool `json:"tools"`
}

type RepoDiagnosticAnalysisSummary struct {
	ProjectName  string                 `json:"projectName"`
	ProjectType  string                 `json:"projectType"`
	Confidence   float64                `json:"confidence"`
	Evidence     []string               `json:"evidence"`
	Requirements []ToolRequirement      `json:"requirements"`
	EnvVariables []RepoDiagnosticEnvVar `json:"envVariables"`
	Services     []ServiceDependency    `json:"services"`
	Unknowns     []string               `json:"unknowns"`
}

type RepoDiagnosticSetupPlanSummary struct {
	ProjectName  string                   `json:"projectName"`
	ProjectType  string                   `json:"projectType"`
	Confidence   float64                  `json:"confidence"`
	Evidence     []string                 `json:"evidence"`
	Gaps         []RequirementGap         `json:"gaps"`
	EnvVariables []RepoDiagnosticEnvVar   `json:"envVariables"`
	Services     []ServiceDependency      `json:"services"`
	Steps        []RepoDiagnosticPlanStep `json:"steps"`
	Safety       SafetyReport             `json:"safety"`
	Unknowns     []string                 `json:"unknowns"`
}

type RepoDiagnosticEnvVar struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Required      bool   `json:"required"`
	Secret        bool   `json:"secret"`
	CurrentStatus string `json:"currentStatus"`
	Service       string `json:"service,omitempty"`
	TargetDir     string `json:"targetDir,omitempty"`
}

type RepoDiagnosticPlanStep struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	CommandPreview   string  `json:"commandPreview"`
	Type             string  `json:"type"`
	Importance       string  `json:"importance"`
	Risk             string  `json:"risk"`
	RequiresApproval bool    `json:"requiresApproval"`
	EvidenceSource   string  `json:"evidenceSource,omitempty"`
	Confidence       float64 `json:"confidence"`
	Reason           string  `json:"reason"`
}

type RepoDiagnosticSetupSession struct {
	ID              int64                `json:"id"`
	InstalledRepoID int64                `json:"installedRepoId"`
	RepoPath        string               `json:"repoPath"`
	Status          string               `json:"status"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	Steps           []RepoDiagnosticStep `json:"steps"`
}

type RepoDiagnosticStep struct {
	ID             int64     `json:"id"`
	SetupSessionID int64     `json:"setupSessionId"`
	StepID         string    `json:"stepId"`
	Title          string    `json:"title"`
	CommandHash    string    `json:"commandHash"`
	CommandPreview string    `json:"commandPreview"`
	Cwd            string    `json:"cwd"`
	Status         string    `json:"status"`
	ExitCode       int       `json:"exitCode"`
	Duration       string    `json:"duration"`
	Log            string    `json:"log,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	FinishedAt     time.Time `json:"finishedAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type RepoDiagnosticAIReviewMetadata struct {
	Available bool                                  `json:"available"`
	Entries   []RepoDiagnosticAIReviewEntryMetadata `json:"entries"`
}

type RepoDiagnosticAIReviewEntryMetadata struct {
	ID          int64     `json:"id"`
	CommandHash string    `json:"commandHash,omitempty"`
	Decision    string    `json:"decision,omitempty"`
	Confidence  float64   `json:"confidence,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
}

type SetupSession struct {
	ID              int64     `json:"id"`
	InstalledRepoID int64     `json:"installedRepoId"`
	RepoPath        string    `json:"repoPath"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

const (
	SetupSessionStatusRunning   = "running"
	SetupSessionStatusSucceeded = "succeeded"
	SetupSessionStatusFailed    = "failed"
)

type StepRun struct {
	ID             int64     `json:"id"`
	SetupSessionID int64     `json:"setupSessionId"`
	StepID         string    `json:"stepId"`
	Title          string    `json:"title"`
	CommandHash    string    `json:"commandHash"`
	CommandPreview string    `json:"commandPreview"`
	Cwd            string    `json:"cwd"`
	Status         string    `json:"status"`
	ExitCode       int       `json:"exitCode"`
	Duration       string    `json:"duration"`
	LogPath        string    `json:"logPath,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	FinishedAt     time.Time `json:"finishedAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

const (
	StepRunStatusSucceeded = "succeeded"
	StepRunStatusFailed    = "failed"
)

type ExecuteResponse struct {
	Source                RepoSource                `json:"source"`
	Analysis              RepositoryAnalysis        `json:"analysis"`
	Environment           EnvironmentReport         `json:"environment"`
	Plan                  SetupPlan                 `json:"plan"`
	Result                ExecutionResult           `json:"result"`
	VaultPromptCandidates []EnvVaultPromptCandidate `json:"vaultPromptCandidates,omitempty"`
}

type RepoSource struct {
	Type    string `json:"type"`
	RepoURL string `json:"repoUrl,omitempty"`
	Path    string `json:"path"`
}

type RepositoryAnalysis struct {
	ProjectName  string              `json:"projectName"`
	ProjectType  string              `json:"projectType"`
	RepoPath     string              `json:"repoPath"`
	Confidence   float64             `json:"confidence"`
	Evidence     []string            `json:"evidence"`
	Topology     AppTopology         `json:"topology"`
	Requirements []ToolRequirement   `json:"requirements"`
	Env          EnvironmentConfig   `json:"env"`
	Services     []ServiceDependency `json:"services"`
	Steps        []ExecutionStep     `json:"steps"`
	Unknowns     []string            `json:"unknowns"`
}

type AppTopology struct {
	Signals []AppTopologySignal `json:"signals"`
}

type AppTopologySignal struct {
	Kind       string  `json:"kind"`
	TargetDir  string  `json:"targetDir,omitempty"`
	Service    string  `json:"service,omitempty"`
	Provider   string  `json:"provider,omitempty"`
	Port       int     `json:"port,omitempty"`
	Confidence float64 `json:"confidence"`
	Evidence   string  `json:"evidence"`
}

type ToolRequirement struct {
	Tool              string `json:"tool"`
	VersionConstraint string `json:"versionConstraint"`
	Source            string `json:"source"`
	Required          bool   `json:"required"`
}

type ExecutionStep struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Command          string   `json:"command"`
	Cwd              string   `json:"cwd"`
	Type             string   `json:"type"`
	Importance       string   `json:"importance"`
	Risk             string   `json:"risk"`
	RequiresApproval bool     `json:"requiresApproval"`
	EvidenceSource   string   `json:"evidenceSource,omitempty"`
	ConfirmedBy      []string `json:"confirmedBy,omitempty"`
	Confidence       float64  `json:"confidence"`
	Reason           string   `json:"reason"`
}

type EnvironmentReport struct {
	OS    string         `json:"os"`
	Arch  string         `json:"arch"`
	Tools []DetectedTool `json:"tools"`
}

type DetectedTool struct {
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Available bool   `json:"available"`
}

type SetupPlan struct {
	ProjectName string              `json:"projectName"`
	ProjectType string              `json:"projectType"`
	Confidence  float64             `json:"confidence"`
	Evidence    []string            `json:"evidence"`
	Gaps        []RequirementGap    `json:"gaps"`
	Env         EnvironmentConfig   `json:"env"`
	Services    []ServiceDependency `json:"services"`
	Steps       []ExecutionStep     `json:"steps"`
	Safety      SafetyReport        `json:"safety"`
	Unknowns    []string            `json:"unknowns"`
}

type EnvironmentConfig struct {
	TemplatePath         string                `json:"templatePath,omitempty"`
	TargetPath           string                `json:"targetPath,omitempty"`
	TargetExists         bool                  `json:"targetExists"`
	Variables            []EnvVarRequirement   `json:"variables"`
	SourceFixSuggestions []SourceFixSuggestion `json:"sourceFixSuggestions,omitempty"`
}

type SourceFixSuggestion struct {
	FilePath      string `json:"filePath"`
	Summary       string `json:"summary"`
	SuggestedText string `json:"suggestedText"`
}

type EnvVarRequirement struct {
	Name            string              `json:"name"`
	Source          string              `json:"source"`
	Required        bool                `json:"required"`
	Secret          bool                `json:"secret"`
	Confidence      float64             `json:"confidence,omitempty"`
	CurrentStatus   string              `json:"currentStatus"`
	FillStrategy    string              `json:"fillStrategy"`
	Service         string              `json:"service,omitempty"`
	TargetDir       string              `json:"targetDir,omitempty"`
	SuggestedValue  string              `json:"suggestedValue,omitempty"`
	Instructions    []string            `json:"instructions,omitempty"`
	TopologySignals []AppTopologySignal `json:"-"`
	ProjectName     string              `json:"-"`
	DefaultSource   string              `json:"-"`
	DefaultClass    string              `json:"-"`
}

const (
	EnvValueSourceExistingFile    = "existing_file"
	EnvValueSourceTemplate        = "template"
	EnvValueSourceDraft           = "draft"
	EnvValueSourceCatalog         = "catalog"
	EnvValueSourceAllocator       = "allocator"
	EnvValueSourceGeneratedSecret = "generated_secret"
	EnvValueSourceVault           = "vault"
	EnvValueSourceAIPatch         = "ai_patch"
)

const (
	EnvValueClassDevDefault           = "dev_default"
	EnvValueClassGeneratedLocalSecret = "generated_local_secret"
	EnvValueClassServiceCredential    = "service_credential"
	EnvValueClassProviderConfig       = "provider_config"
)

type EnvDraft struct {
	RepoPath string           `json:"repoPath"`
	Targets  []EnvDraftTarget `json:"targets"`
}

type EnvDraftTarget struct {
	RelativePath    string          `json:"relativePath"`
	AbsolutePath    string          `json:"absolutePath"`
	OriginalContent string          `json:"originalContent"`
	Values          []EnvDraftValue `json:"values"`
}

type EnvDraftValue struct {
	Name             string             `json:"name"`
	Value            string             `json:"value"`
	Secret           bool               `json:"secret"`
	Confidence       float64            `json:"confidence"`
	ValueClass       string             `json:"valueClass,omitempty"`
	Instructions     []string           `json:"instructions,omitempty"`
	Attention        []string           `json:"attention,omitempty"`
	Provenance       EnvValueProvenance `json:"provenance"`
	VaultBinding     *EnvVaultBinding   `json:"vaultBinding,omitempty"`
	HasExistingValue bool               `json:"hasExistingValue,omitempty"`
}

type EnvValueProvenance struct {
	Source string `json:"source"`
}

type AIEnvReviewBundle struct {
	SchemaVersion string                      `json:"schemaVersion"`
	Repo          AIEnvReviewRepo             `json:"repo"`
	FileTree      []string                    `json:"fileTree"`
	Manifests     []AIEnvReviewManifest       `json:"manifests"`
	SetupExcerpts []AIEnvReviewExcerpt        `json:"setupExcerpts"`
	EnvNames      []string                    `json:"envNames"`
	Targets       []AIEnvReviewTarget         `json:"targets"`
	UsageSnippets []AIEnvReviewUsageSnippet   `json:"usageSnippets"`
	Topology      AppTopology                 `json:"topology"`
	Candidates    []AIEnvReviewDraftCandidate `json:"candidates"`
}

type AIEnvReviewRepo struct {
	Public             bool   `json:"public"`
	URL                string `json:"url,omitempty"`
	CommitSHA          string `json:"commitSha,omitempty"`
	IdentityOmitted    bool   `json:"identityOmitted,omitempty"`
	PrivateOrUncertain bool   `json:"privateOrUncertain,omitempty"`
}

type AIEnvReviewManifest struct {
	RelativePath string            `json:"relativePath"`
	Scripts      map[string]string `json:"scripts,omitempty"`
}

type AIEnvReviewExcerpt struct {
	RelativePath string `json:"relativePath"`
	Text         string `json:"text"`
}

type AIEnvReviewTarget struct {
	RelativePath string   `json:"relativePath"`
	EnvNames     []string `json:"envNames"`
}

type AIEnvReviewUsageSnippet struct {
	RelativePath string `json:"relativePath"`
	EnvName      string `json:"envName"`
	Snippet      string `json:"snippet"`
}

type AIEnvReviewDraftCandidate struct {
	TargetRelativePath string  `json:"targetRelativePath"`
	VariableName       string  `json:"variableName"`
	ValueClass         string  `json:"valueClass"`
	CurrentValue       string  `json:"currentValue,omitempty"`
	CurrentValueState  string  `json:"currentValueState"`
	Confidence         float64 `json:"confidence"`
	Provenance         string  `json:"provenance"`
}

type EnvPatch struct {
	Operations []EnvPatchOperation `json:"operations"`
}

type EnvPatchOperation struct {
	Op                 string  `json:"op"`
	TargetRelativePath string  `json:"targetRelativePath,omitempty"`
	VariableName       string  `json:"variableName,omitempty"`
	Value              string  `json:"value,omitempty"`
	Confidence         float64 `json:"confidence,omitempty"`
	Reason             string  `json:"reason,omitempty"`
	Path               string  `json:"path,omitempty"`
	Command            string  `json:"command,omitempty"`
}

type EnvSaveResult struct {
	Targets []EnvSaveTargetResult `json:"targets"`
}

type EnvSaveTargetResult struct {
	RelativePath string `json:"relativePath"`
	Succeeded    bool   `json:"succeeded"`
	ErrorKind    string `json:"errorKind,omitempty"`
}

const (
	EnvVaultStatusReady        = "ready"
	EnvVaultStatusNeedsReview  = "needs_review"
	EnvVaultStatusActionNeeded = "action_needed"
	EnvVaultStatusInvalid      = "invalid"
)

const EnvVaultDuplicateReviewMessage = "This credential may already exist or may need review."

type EnvVaultSaveRequest struct {
	Provider     string `json:"provider"`
	VariableName string `json:"variableName"`
	DisplayName  string `json:"displayName,omitempty"`
	Value        string `json:"value"`
}

type EnvVaultSaveResponse struct {
	Entry         EnvVaultEntry `json:"entry,omitempty"`
	NeedsReview   bool          `json:"needsReview"`
	ReviewMessage string        `json:"reviewMessage,omitempty"`
}

type EnvVaultUpdateRequest struct {
	EntryID     int64  `json:"entryId"`
	DisplayName string `json:"displayName,omitempty"`
	UpdateValue bool   `json:"updateValue,omitempty"`
	Value       string `json:"value,omitempty"`
}

type EnvVaultManagerResponse struct {
	Entries []EnvVaultManagerEntry `json:"entries"`
}

type EnvVaultManagerEntry struct {
	EnvVaultEntry
	Usage     EnvVaultUsageSummary `json:"usage"`
	Approvals []EnvVaultApproval   `json:"approvals"`
}

type EnvVaultUsageSummary struct {
	TotalUseCount int                     `json:"totalUseCount"`
	Locations     []EnvVaultUsageLocation `json:"locations"`
}

type EnvVaultUsageLocation struct {
	RepoPath           string    `json:"repoPath"`
	TargetRelativePath string    `json:"targetRelativePath"`
	VariableName       string    `json:"variableName"`
	LastUsedAt         time.Time `json:"lastUsedAt"`
	UseCount           int       `json:"useCount"`
}

type EnvVaultRevealRequest struct {
	EntryID   int64 `json:"entryId"`
	Confirmed bool  `json:"confirmed"`
}

type EnvVaultRevealResponse struct {
	EntryID     int64     `json:"entryId"`
	Value       string    `json:"value"`
	RevealUntil time.Time `json:"revealUntil"`
}

type EnvVaultEntry struct {
	ID                  int64     `json:"id"`
	Provider            string    `json:"provider"`
	VariableName        string    `json:"variableName"`
	DisplayName         string    `json:"displayName"`
	FingerprintFragment string    `json:"fingerprintFragment"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type EnvVaultEntryMetadata struct {
	EnvVaultEntry
	CredentialKey string `json:"-"`
	Fingerprint   string `json:"-"`
}

type EnvVaultApproval struct {
	ID                 int64     `json:"id"`
	EntryID            int64     `json:"entryId"`
	RepoPath           string    `json:"repoPath"`
	TargetRelativePath string    `json:"targetRelativePath"`
	VariableName       string    `json:"variableName"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

const (
	EnvVaultApprovalStatusApproved = "approved"
	EnvVaultApprovalStatusRevoked  = "revoked"
)

type EnvVaultUseRecord struct {
	ID                 int64     `json:"id"`
	EntryID            int64     `json:"entryId"`
	RepoPath           string    `json:"repoPath"`
	TargetRelativePath string    `json:"targetRelativePath"`
	VariableName       string    `json:"variableName"`
	UsedAt             time.Time `json:"usedAt"`
	UseCount           int       `json:"useCount"`
}

type EnvVaultBinding struct {
	EntryID             int64  `json:"entryId"`
	Provider            string `json:"provider"`
	VariableName        string `json:"variableName"`
	DisplayName         string `json:"displayName,omitempty"`
	FingerprintFragment string `json:"fingerprint"`
	Status              string `json:"status"`
}

type EnvVaultPromptCandidate struct {
	RepoPath            string `json:"repoPath"`
	TargetRelativePath  string `json:"targetRelativePath"`
	VariableName        string `json:"variableName"`
	Provider            string `json:"provider"`
	FingerprintFragment string `json:"fingerprintFragment"`
}

type EnvVaultPromptSuppression struct {
	RepoPath           string    `json:"repoPath"`
	TargetRelativePath string    `json:"targetRelativePath"`
	VariableName       string    `json:"variableName"`
	SuppressedAt       time.Time `json:"suppressedAt"`
}

type EnvContributionSettings struct {
	PublicEnvPatternsEnabled       bool      `json:"publicEnvPatternsEnabled"`
	PrivateLocalEnvPatternsEnabled bool      `json:"privateLocalEnvPatternsEnabled"`
	ConsentShown                   bool      `json:"consentShown"`
	UpdatedAt                      time.Time `json:"updatedAt"`
}

type EnvContributionSettingsResponse struct {
	Settings EnvContributionSettings    `json:"settings"`
	Queue    EnvContributionQueueStatus `json:"queue"`
}

type EnvContributionQueueStatus struct {
	Count           int       `json:"count"`
	OldestCreatedAt time.Time `json:"oldestCreatedAt,omitempty"`
}

const (
	EnvContributionEventAnalysis    = "analysis"
	EnvContributionEventSaveOutcome = "save_outcome"
)

type EnvContributionQueueItem struct {
	ID            int64     `json:"id"`
	EventType     string    `json:"eventType"`
	PayloadJSON   string    `json:"payloadJson"`
	CreatedAt     time.Time `json:"createdAt"`
	Attempts      int       `json:"attempts"`
	LastAttemptAt time.Time `json:"lastAttemptAt,omitempty"`
}

type EnvContributionPayload struct {
	SchemaVersion  string                   `json:"schemaVersion"`
	EventType      string                   `json:"eventType"`
	AppVersion     string                   `json:"appVersion"`
	CatalogVersion string                   `json:"catalogVersion"`
	OS             EnvContributionOS        `json:"os"`
	Repo           EnvContributionRepo      `json:"repo"`
	EnvNames       []string                 `json:"envNames"`
	Targets        []EnvContributionTarget  `json:"targets"`
	Stacks         []EnvContributionStack   `json:"stacks"`
	Outcomes       []EnvContributionOutcome `json:"outcomes,omitempty"`
	AI             EnvContributionAI        `json:"ai,omitempty"`
}

type EnvContributionOS struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Arch    string `json:"arch,omitempty"`
}

type EnvContributionRepo struct {
	Public             bool   `json:"public"`
	URL                string `json:"url,omitempty"`
	CommitSHA          string `json:"commitSha,omitempty"`
	EnvRelevantDirty   bool   `json:"envRelevantDirty,omitempty"`
	IdentityOmitted    bool   `json:"identityOmitted,omitempty"`
	PrivateOrUncertain bool   `json:"privateOrUncertain,omitempty"`
}

type EnvContributionTarget struct {
	RelativePath string `json:"relativePath"`
}

type EnvContributionStack struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type EnvContributionOutcome struct {
	TargetRelativePath string `json:"targetRelativePath"`
	VariableName       string `json:"variableName"`
	ValueClass         string `json:"valueClass,omitempty"`
	FillOutcome        string `json:"fillOutcome,omitempty"`
	ValueState         string `json:"valueState"`
	Saved              bool   `json:"saved"`
	ErrorKind          string `json:"errorKind,omitempty"`
}

type EnvContributionAI struct {
	ReviewCount  int  `json:"reviewCount,omitempty"`
	PatchApplied bool `json:"patchApplied,omitempty"`
}

type EnvPortAssignment struct {
	RepoPath  string `json:"repoPath"`
	TargetDir string `json:"targetDir"`
	Purpose   string `json:"purpose"`
	Port      int    `json:"port"`
}

type AIEnvReviewSettings struct {
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type ServiceDependency struct {
	Name         string   `json:"name"`
	Scope        string   `json:"scope"`
	Provisioning string   `json:"provisioning"`
	Source       string   `json:"source"`
	Status       string   `json:"status"`
	Details      string   `json:"details,omitempty"`
	Instructions []string `json:"instructions,omitempty"`
}

type RequirementGap struct {
	Tool             string `json:"tool"`
	RequiredVersion  string `json:"requiredVersion"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	Status           string `json:"status"`
}

type SafetyReport struct {
	RiskLevel string          `json:"riskLevel"`
	Findings  []SafetyFinding `json:"findings"`
}

type SafetyFinding struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	FilePath string `json:"filePath,omitempty"`
}

type ExecutionResult struct {
	StepID    string `json:"stepId"`
	Command   string `json:"command"`
	Cwd       string `json:"cwd"`
	ProcessID int    `json:"processId"`
	ExitCode  int    `json:"exitCode"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Duration  string `json:"duration"`
	Succeeded bool   `json:"succeeded"`
}

const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

const (
	StepRequired    = "required"
	StepRecommended = "recommended"
	StepOptional    = "optional"
	StepManual      = "manual"
	StepUncertain   = "uncertain"
)
