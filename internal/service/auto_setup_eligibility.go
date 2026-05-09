package service

import (
	"strings"

	"instantrepo/internal/domain"
)

const (
	AutoSetupStepAutoAllowed   = "auto-allowed"
	AutoSetupStepAttentionStop = "attention-stop"
	AutoSetupStepRiskStop      = "risk-stop"
	AutoSetupStepManual        = "manual"
	AutoSetupStepUncertain     = "uncertain"
	AutoSetupStepLaunchOnly    = "launch-only"
)

const (
	InstallScriptPolicyNormalLifecycleScripts = "normal-lifecycle-scripts"
	InstallScriptPolicySkipLifecycleScripts   = "skip-lifecycle-scripts"
)

type AutoSetupEligibilityRequest struct {
	Plan                domain.SetupPlan
	Environment         domain.EnvironmentReport
	InstallScriptPolicy string
	PriorStepStatuses   map[string]string
}

type AutoSetupEligibilityResponse struct {
	InstallScriptPolicy string
	Steps               []AutoSetupStepEligibility
}

type AutoSetupStepEligibility struct {
	StepID         string
	Status         string
	Reason         string
	CommandPreview string
}

func ClassifyAutoSetupSteps(req AutoSetupEligibilityRequest) AutoSetupEligibilityResponse {
	installScriptPolicy := normalizeInstallScriptPolicy(req.InstallScriptPolicy)
	response := AutoSetupEligibilityResponse{
		InstallScriptPolicy: installScriptPolicy,
		Steps:               make([]AutoSetupStepEligibility, 0, len(req.Plan.Steps)),
	}
	safetyBlocked := hasHighRiskSafety(req.Plan.Safety)
	envBlocked := hasUnresolvedUserEnv(req.Plan.Env)
	toolBlocked := hasMissingSystemTool(req.Plan.Gaps)
	availableTools := availableToolSet(req.Environment)
	priorFailed := false

	for _, step := range req.Plan.Steps {
		autoCandidate := isAutoAllowedCandidate(step)
		eligibility := AutoSetupStepEligibility{
			StepID:         step.ID,
			Status:         AutoSetupStepUncertain,
			Reason:         "Step needs review before Auto Setup can run it.",
			CommandPreview: commandPreviewForPolicy(step, installScriptPolicy),
		}
		if safetyBlocked {
			eligibility.Status = AutoSetupStepRiskStop
			eligibility.Reason = "High-risk safety finding stops Auto Setup before commands run."
		} else if step.Risk == domain.RiskHigh && step.Type != "system-install" && step.Type != "env-review" {
			eligibility.Status = AutoSetupStepRiskStop
			eligibility.Reason = "High-risk setup step is excluded from Auto Setup."
		} else if priorFailed || req.PriorStepStatuses[step.ID] == domain.StepRunStatusFailed {
			eligibility.Status = AutoSetupStepAttentionStop
			eligibility.Reason = "Failed prior setup step needs attention before Auto Setup can continue."
		} else if step.Type == "system-install" || toolBlocked {
			eligibility.Status = AutoSetupStepAttentionStop
			eligibility.Reason = "Missing System Tool needs user attention outside Auto Setup."
		} else if isReadmeOnlyStep(step) {
			eligibility.Status = AutoSetupStepUncertain
			eligibility.Reason = "README-only command lacks manifest or config confirmation."
		} else if step.Type == "env-review" || envBlocked {
			eligibility.Status = AutoSetupStepAttentionStop
			eligibility.Reason = "Unresolved env values need user attention before Auto Setup can continue."
		} else if isBuildOrTestStep(step) {
			eligibility.Status = AutoSetupStepManual
			eligibility.Reason = "Build and test commands are excluded from Auto Setup."
		} else if isManualStep(step) {
			eligibility.Status = AutoSetupStepManual
			eligibility.Reason = "Manual setup step needs user action outside Auto Setup."
		} else if isLaunchStep(step) {
			eligibility.Status = AutoSetupStepLaunchOnly
			eligibility.Reason = "Launch commands run after setup and are not part of Auto Setup."
		} else if missingTools := missingRequiredTools(step, availableTools); autoCandidate && len(missingTools) > 0 {
			eligibility.Status = AutoSetupStepAttentionStop
			eligibility.Reason = "Missing System Tool needs user attention outside Auto Setup: " + strings.Join(missingTools, ", ")
		} else if autoCandidate {
			eligibility.Status = AutoSetupStepAutoAllowed
			eligibility.Reason = "Setup step is backed by manifest evidence."
		}
		response.Steps = append(response.Steps, eligibility)
		if req.PriorStepStatuses[step.ID] == domain.StepRunStatusFailed {
			priorFailed = true
		}
	}

	return response
}

func availableToolSet(env domain.EnvironmentReport) map[string]bool {
	tools := make(map[string]bool, len(env.Tools))
	for _, tool := range env.Tools {
		tools[strings.ToLower(tool.Name)] = tool.Available
	}
	return tools
}

func isAutoAllowedCandidate(step domain.ExecutionStep) bool {
	return isNodeDependencyInstall(step) || isPythonVenvSetup(step) || isPythonDependencyInstall(step) || isGoDependencyInstall(step) || isDockerComposeServiceStart(step)
}

func missingRequiredTools(step domain.ExecutionStep, availableTools map[string]bool) []string {
	var required []string
	switch {
	case isNodeDependencyInstall(step):
		required = []string{"node", firstCommandToken(step.Command)}
	case isPythonVenvSetup(step):
		required = []string{"python"}
	case isPythonDependencyInstall(step):
		required = []string{"python", firstCommandToken(step.Command)}
	case isGoDependencyInstall(step):
		required = []string{"go"}
	case isDockerComposeServiceStart(step):
		required = []string{"docker"}
	}

	missing := []string{}
	seen := map[string]bool{}
	for _, tool := range required {
		tool = strings.ToLower(strings.TrimSpace(tool))
		if tool == "" || seen[tool] {
			continue
		}
		seen[tool] = true
		if !availableTools[tool] {
			missing = append(missing, tool)
		}
	}
	return missing
}

func firstCommandToken(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func normalizeInstallScriptPolicy(policy string) string {
	if policy == InstallScriptPolicySkipLifecycleScripts {
		return InstallScriptPolicySkipLifecycleScripts
	}
	return InstallScriptPolicyNormalLifecycleScripts
}

func commandPreviewForPolicy(step domain.ExecutionStep, policy string) string {
	if policy != InstallScriptPolicySkipLifecycleScripts || !isNodeDependencyInstall(step) {
		return step.Command
	}
	command := strings.TrimSpace(step.Command)
	if strings.Contains(command, "--ignore-scripts") {
		return command
	}
	return command + " --ignore-scripts"
}

func isReadmeOnlyStep(step domain.ExecutionStep) bool {
	return step.EvidenceSource == "readme"
}

func hasHighRiskSafety(safety domain.SafetyReport) bool {
	if safety.RiskLevel == domain.RiskHigh {
		return true
	}
	for _, finding := range safety.Findings {
		if finding.Severity == "high" {
			return true
		}
	}
	return false
}

func hasMissingSystemTool(gaps []domain.RequirementGap) bool {
	for _, gap := range gaps {
		if gap.Status == "missing" || gap.Status == "version_mismatch" {
			return true
		}
	}
	return false
}

func isBuildOrTestStep(step domain.ExecutionStep) bool {
	command := strings.ToLower(strings.TrimSpace(step.Command))
	if strings.Contains(command, "--build") {
		return true
	}
	if strings.Contains(command, " run build") || strings.Contains(command, " run test") {
		return true
	}
	if strings.HasPrefix(command, "go test") || command == "npm test" || command == "pnpm test" || command == "bun test" || command == "yarn test" {
		return true
	}
	if strings.Contains(command, " build") || strings.Contains(command, " test") {
		return true
	}
	return false
}

func isManualStep(step domain.ExecutionStep) bool {
	return step.Importance == domain.StepManual || strings.Contains(step.Type, "review") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(step.Command)), "manual ")
}

func isLaunchStep(step domain.ExecutionStep) bool {
	return step.Type == "run" || step.Type == "launch"
}

func isNodeDependencyInstall(step domain.ExecutionStep) bool {
	if step.Type != "dependency-install" || step.Importance != domain.StepRequired {
		return false
	}
	if step.EvidenceSource != "manifest" && step.EvidenceSource != "manifest+readme" {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(step.Command))
	return command == "npm install" || command == "pnpm install" || command == "bun install" || command == "yarn install"
}

func hasUnresolvedUserEnv(env domain.EnvironmentConfig) bool {
	for _, item := range env.Variables {
		if !item.Required {
			continue
		}
		if item.CurrentStatus == "configured" {
			continue
		}
		if item.FillStrategy == "user_required" {
			return true
		}
	}
	return false
}

func isPythonVenvSetup(step domain.ExecutionStep) bool {
	if step.Type != "env-setup" || step.Importance != domain.StepRequired || step.Risk != domain.RiskLow {
		return false
	}
	if step.EvidenceSource != "manifest" && step.EvidenceSource != "manifest+readme" {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(step.Command))
	return strings.HasPrefix(command, "python -m venv ") || strings.HasPrefix(command, "python3 -m venv ")
}

func isPythonDependencyInstall(step domain.ExecutionStep) bool {
	if step.Type != "dependency-install" || step.Importance != domain.StepRequired {
		return false
	}
	if step.EvidenceSource != "manifest" && step.EvidenceSource != "manifest+readme" {
		return false
	}

	command := strings.ToLower(strings.TrimSpace(step.Command))
	switch {
	case strings.HasPrefix(command, "python -m pip install "):
		return true
	case strings.HasPrefix(command, "python3 -m pip install "):
		return true
	case strings.HasPrefix(command, "uv pip install "):
		return true
	case command == "poetry install":
		return true
	case command == "pipenv install":
		return true
	default:
		return false
	}
}

func isGoDependencyInstall(step domain.ExecutionStep) bool {
	if step.Type != "dependency-install" || step.Importance != domain.StepRequired {
		return false
	}
	if step.EvidenceSource != "manifest" && step.EvidenceSource != "manifest+readme" {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(step.Command))
	return command == "go mod download"
}

func isDockerComposeServiceStart(step domain.ExecutionStep) bool {
	if step.Type != "service-start" || step.Importance != domain.StepRequired {
		return false
	}
	if step.EvidenceSource != "config" {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(step.Command))
	if strings.Contains(command, "--build") {
		return false
	}
	return strings.HasPrefix(command, "docker compose ") && strings.Contains(command, " up -d")
}
