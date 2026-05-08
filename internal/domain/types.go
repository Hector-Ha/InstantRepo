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

type ExecuteResponse struct {
	Source      RepoSource         `json:"source"`
	Analysis    RepositoryAnalysis `json:"analysis"`
	Environment EnvironmentReport  `json:"environment"`
	Plan        SetupPlan          `json:"plan"`
	Result      ExecutionResult    `json:"result"`
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
	Requirements []ToolRequirement   `json:"requirements"`
	Env          EnvironmentConfig   `json:"env"`
	Services     []ServiceDependency `json:"services"`
	Steps        []ExecutionStep     `json:"steps"`
	Unknowns     []string            `json:"unknowns"`
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
	TemplatePath string              `json:"templatePath,omitempty"`
	TargetPath   string              `json:"targetPath,omitempty"`
	TargetExists bool                `json:"targetExists"`
	Variables    []EnvVarRequirement `json:"variables"`
}

type EnvVarRequirement struct {
	Name           string   `json:"name"`
	Source         string   `json:"source"`
	Required       bool     `json:"required"`
	Secret         bool     `json:"secret"`
	CurrentStatus  string   `json:"currentStatus"`
	FillStrategy   string   `json:"fillStrategy"`
	Service        string   `json:"service,omitempty"`
	TargetDir      string   `json:"targetDir,omitempty"`
	SuggestedValue string   `json:"suggestedValue,omitempty"`
	Instructions   []string `json:"instructions,omitempty"`
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
