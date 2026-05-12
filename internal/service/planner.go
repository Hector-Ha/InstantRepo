package service

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"instantrepo/internal/domain"
)

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) BuildPlan(analysis domain.RepositoryAnalysis, env domain.EnvironmentReport) domain.SetupPlan {
	toolIndex := map[string]domain.DetectedTool{}
	for _, tool := range env.Tools {
		toolIndex[tool.Name] = tool
	}

	gaps := make([]domain.RequirementGap, 0, len(analysis.Requirements))
	steps := make([]domain.ExecutionStep, 0, len(analysis.Steps)+4)
	for _, req := range analysis.Requirements {
		detected, ok := toolIndex[req.Tool]
		if !ok || !detected.Available {
			gaps = append(gaps, domain.RequirementGap{
				Tool:            req.Tool,
				RequiredVersion: req.VersionConstraint,
				Status:          "missing",
			})
			if installStep, ok := suggestedInstallStep(env.OS, req.Tool, analysis.RepoPath); ok {
				steps = append(steps, installStep)
			}
			continue
		}

		status := "satisfied"
		if req.VersionConstraint != "" && req.VersionConstraint != "unspecified" && !versionLooksCompatible(detected.Version, req.VersionConstraint) {
			status = "version_mismatch"
		}

		gaps = append(gaps, domain.RequirementGap{
			Tool:             req.Tool,
			RequiredVersion:  req.VersionConstraint,
			InstalledVersion: detected.Version,
			Status:           status,
		})
	}

	if step, ok := envTemplateCopyStep(env.OS, analysis); ok {
		steps = append(steps, step)
	}

	steps = append(steps, analysis.Steps...)
	safety := scanSafety(analysis.RepoPath)

	if safety.RiskLevel == domain.RiskHigh {
		steps = append([]domain.ExecutionStep{
			{
				ID:               "manual-review",
				Title:            "Review flagged repository files",
				Command:          "manual review required",
				Cwd:              analysis.RepoPath,
				Type:             "review",
				Importance:       domain.StepManual,
				Risk:             domain.RiskHigh,
				RequiresApproval: true,
				EvidenceSource:   "safety-scan",
				ConfirmedBy:      []string{"repository file scan"},
				Confidence:       0.96,
				Reason:           "Potentially risky files were detected during the pre-execution scan.",
			},
		}, steps...)
	}

	sort.SliceStable(steps, func(i, j int) bool {
		leftImportance := importancePriority(steps[i].Importance)
		rightImportance := importancePriority(steps[j].Importance)
		if leftImportance != rightImportance {
			return leftImportance < rightImportance
		}
		return stepPriority(steps[i].Type) < stepPriority(steps[j].Type)
	})

	return domain.SetupPlan{
		ProjectName: analysis.ProjectName,
		ProjectType: analysis.ProjectType,
		Confidence:  analysis.Confidence,
		Evidence:    analysis.Evidence,
		Gaps:        gaps,
		Env:         analysis.Env,
		Services:    analysis.Services,
		Steps:       steps,
		Safety:      safety,
		Unknowns:    analysis.Unknowns,
	}
}

func stepPriority(stepType string) int {
	switch stepType {
	case "review":
		return 0
	case "system-install":
		return 1
	case "env-setup":
		return 2
	case "dependency-install":
		return 3
	case "service-start":
		return 4
	case "env-review":
		return 5
	case "run":
		return 6
	default:
		return 10
	}
}

func importancePriority(importance string) int {
	switch importance {
	case domain.StepRequired:
		return 0
	case domain.StepRecommended:
		return 1
	case domain.StepManual:
		return 2
	case domain.StepOptional:
		return 3
	case domain.StepUncertain:
		return 4
	default:
		return 5
	}
}

func suggestedInstallStep(goos, tool, repoPath string) (domain.ExecutionStep, bool) {
	command := ""
	switch goos {
	case "windows":
		switch tool {
		case "node":
			command = "winget install OpenJS.NodeJS"
		case "git":
			command = "winget install Git.Git"
		case "python":
			command = "winget install Python.Python.3.12"
		case "docker":
			command = "winget install Docker.DockerDesktop"
		}
	case "darwin":
		switch tool {
		case "node":
			command = "brew install node"
		case "git":
			command = "brew install git"
		case "python":
			command = "brew install python"
		case "docker":
			command = "brew install --cask docker"
		}
	}

	if command == "" {
		return domain.ExecutionStep{}, false
	}

	return domain.ExecutionStep{
		ID:               "install-" + tool,
		Title:            fmt.Sprintf("Install missing tool: %s", tool),
		Command:          command,
		Cwd:              repoPath,
		Type:             "system-install",
		Importance:       domain.StepRequired,
		Risk:             domain.RiskHigh,
		RequiresApproval: true,
		EvidenceSource:   "environment",
		ConfirmedBy:      []string{tool + " missing locally"},
		Confidence:       0.98,
		Reason:           fmt.Sprintf("%s is required by the repository but is not installed locally.", tool),
	}, true
}

func envTemplateCopyStep(goos string, analysis domain.RepositoryAnalysis) (domain.ExecutionStep, bool) {
	if analysis.Env.TemplatePath == "" && len(analysis.Env.Variables) == 0 {
		return domain.ExecutionStep{}, false
	}

	title := "Prepare local .env file"
	reason := "InstantRepo can create or update the local .env file using safe defaults and clear placeholders."
	if analysis.Env.TargetExists {
		title = "Update local .env file"
		reason = "InstantRepo can merge safe defaults into the existing .env file while preserving existing values."
	}

	return domain.ExecutionStep{
		ID:               "create-env-file",
		Title:            title,
		Command:          "instantrepo internal:prepare-env",
		Cwd:              analysis.RepoPath,
		Type:             "env-setup",
		Importance:       domain.StepRequired,
		Risk:             domain.RiskLow,
		RequiresApproval: true,
		EvidenceSource:   "config",
		ConfirmedBy:      []string{"env template or env variable requirements detected"},
		Confidence:       0.97,
		Reason:           reason,
	}, true
}

func versionLooksCompatible(installedVersion, constraint string) bool {
	if constraint == "" || constraint == "unspecified" {
		return true
	}

	normalized := strings.ToLower(installedVersion + " " + constraint)
	switch {
	case strings.Contains(constraint, ">="):
		required := extractDigits(constraint)
		return required == "" || strings.Contains(normalized, required)
	default:
		required := extractDigits(constraint)
		return required == "" || strings.Contains(normalized, required)
	}
}

func extractDigits(input string) string {
	var b strings.Builder
	for _, r := range input {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func scanSafety(repoPath string) domain.SafetyReport {
	findings := []domain.SafetyFinding{}
	riskLevel := domain.RiskLow

	suspiciousExtensions := map[string]string{
		".exe": "Executable file present",
		".msi": "Installer package present",
		".bat": "Batch script present",
		".cmd": "Command script present",
		".ps1": "PowerShell script present",
		".sh":  "Shell script present",
	}

	_ = filepath.Walk(repoPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			if info != nil && info.IsDir() && shouldSkipGeneratedRepoDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if summary, ok := suspiciousExtensions[ext]; ok {
			findings = append(findings, domain.SafetyFinding{
				Severity: severityForExtension(ext),
				Summary:  summary,
				FilePath: path,
			})
			if ext == ".exe" || ext == ".msi" || ext == ".ps1" {
				riskLevel = domain.RiskHigh
			} else if riskLevel != domain.RiskHigh {
				riskLevel = domain.RiskMedium
			}
		}
		return nil
	})

	return domain.SafetyReport{
		RiskLevel: riskLevel,
		Findings:  findings,
	}
}

func shouldSkipGeneratedRepoDir(name string) bool {
	switch name {
	case "node_modules", ".git", "vendor", "build", "dist", ".next":
		return true
	default:
		return false
	}
}

func severityForExtension(ext string) string {
	switch ext {
	case ".exe", ".msi", ".ps1":
		return "high"
	default:
		return "medium"
	}
}
