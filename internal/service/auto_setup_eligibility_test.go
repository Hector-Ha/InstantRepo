package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"instantrepo/internal/domain"
)

type autoSetupEligibilityDetector struct {
	environment domain.EnvironmentReport
}

func (d autoSetupEligibilityDetector) Detect() domain.EnvironmentReport {
	return d.environment
}

func TestAutoSetupEligibilityAllowsNodeDependencyInstall(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
					ConfirmedBy:    []string{"package.json"},
					Confidence:     0.95,
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "node", Available: true},
				{Name: "npm", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "install-node-deps", AutoSetupStepAutoAllowed)
}

func TestAutoSetupEligibilityAllowsPythonVenvSetup(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "create-python-venv",
					Command:        "python -m venv .venv",
					Type:           "env-setup",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskLow,
					EvidenceSource: "manifest",
					ConfirmedBy:    []string{"requirements.txt"},
					Confidence:     0.91,
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "python", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "create-python-venv", AutoSetupStepAutoAllowed)
}

func TestAutoSetupEligibilityAllowsPythonDependencyInstallFromAnalyzePlan(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "requirements.txt"), []byte("flask\n"), 0o644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}

	app := NewAppServiceWithInstalledRepoStore(nil)
	app.detector = autoSetupEligibilityDetector{
		environment: domain.EnvironmentReport{
			OS:   "test-os",
			Arch: "test-arch",
			Tools: []domain.DetectedTool{
				{Name: "python", Available: true},
				{Name: "pip", Available: true},
			},
		},
	}

	analyzeResp, err := app.Analyze(context.Background(), domain.AnalyzeRequest{LocalPath: repoPath})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan:        analyzeResp.Plan,
		Environment: analyzeResp.Environment,
	})

	requireStepStatus(t, response, "python-install-deps", AutoSetupStepAutoAllowed)
}

func TestAutoSetupEligibilityAllowsGoModuleDownload(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "go-mod-download",
					Command:        "go mod download",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
					ConfirmedBy:    []string{"go.mod"},
					Confidence:     0.92,
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "go", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "go-mod-download", AutoSetupStepAutoAllowed)
}

func TestAutoSetupEligibilityAllowsDockerComposeServiceStart(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "start-local-services",
					Command:        `docker compose -f "docker-compose.yml" up -d`,
					Type:           "service-start",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "config",
					ConfirmedBy:    []string{"docker-compose.yml"},
					Confidence:     0.91,
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "docker", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "start-local-services", AutoSetupStepAutoAllowed)
}

func TestAutoSetupEligibilityRejectsDockerComposeBuildFlag(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "start-local-services",
					Command:        `docker compose -f "docker-compose.yml" up -d --build`,
					Type:           "service-start",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "config",
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "docker", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "start-local-services", AutoSetupStepManual)
}

func TestAutoSetupEligibilityStopsForUnresolvedEnv(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Env: domain.EnvironmentConfig{
				Variables: []domain.EnvVarRequirement{
					{
						Name:          "OPENAI_API_KEY",
						Required:      true,
						CurrentStatus: "missing",
						FillStrategy:  "user_required",
					},
				},
			},
			Steps: []domain.ExecutionStep{
				{
					ID:             "review-env-values",
					Command:        "manual env review required",
					Type:           "env-review",
					Importance:     domain.StepManual,
					Risk:           domain.RiskHigh,
					EvidenceSource: "config",
				},
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "node", Available: true},
				{Name: "npm", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "review-env-values", AutoSetupStepAttentionStop)
	requireStepStatus(t, response, "install-node-deps", AutoSetupStepAttentionStop)
}

func TestAutoSetupEligibilityTreatsReadmeOnlyCommandsAsUncertain(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "readme-install",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "readme",
					ConfirmedBy:    []string{"README.md: Setup"},
					Confidence:     0.72,
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "node", Available: true},
				{Name: "npm", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "readme-install", AutoSetupStepUncertain)
	requireStepReasonContains(t, response, "readme-install", "README-only")
}

func TestAutoSetupEligibilityStopsForHighRiskSafetyFinding(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Safety: domain.SafetyReport{
				RiskLevel: domain.RiskHigh,
				Findings: []domain.SafetyFinding{
					{Severity: "high", Summary: "PowerShell script present", FilePath: "setup.ps1"},
				},
			},
			Steps: []domain.ExecutionStep{
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "node", Available: true},
				{Name: "npm", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "install-node-deps", AutoSetupStepRiskStop)
}

func TestAutoSetupEligibilityStopsForHighRiskStep(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskHigh,
					EvidenceSource: "manifest",
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "node", Available: true},
				{Name: "npm", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "install-node-deps", AutoSetupStepRiskStop)
}

func TestAutoSetupEligibilityStopsForMissingSystemTool(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Gaps: []domain.RequirementGap{
				{Tool: "node", Status: "missing"},
			},
			Steps: []domain.ExecutionStep{
				{
					ID:             "install-node",
					Command:        "winget install OpenJS.NodeJS",
					Type:           "system-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskHigh,
					EvidenceSource: "environment",
				},
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "node", Available: false},
			},
		},
	})

	requireStepStatus(t, response, "install-node", AutoSetupStepAttentionStop)
	requireStepStatus(t, response, "install-node-deps", AutoSetupStepAttentionStop)
}

func TestAutoSetupEligibilityStopsWhenRequiredToolMissingFromEnvironment(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
				},
			},
		},
		Environment: domain.EnvironmentReport{
			Tools: []domain.DetectedTool{
				{Name: "node", Available: false},
				{Name: "npm", Available: true},
			},
		},
	})

	requireStepStatus(t, response, "install-node-deps", AutoSetupStepAttentionStop)
}

func TestAutoSetupEligibilityExcludesLaunchBuildAndTestSteps(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "run-node-script-dev",
					Command:        "npm run dev",
					Type:           "run",
					Importance:     domain.StepRecommended,
					Risk:           domain.RiskLow,
					EvidenceSource: "manifest",
				},
				{
					ID:             "run-node-script-build",
					Command:        "npm run build",
					Type:           "run",
					Importance:     domain.StepOptional,
					Risk:           domain.RiskLow,
					EvidenceSource: "manifest",
				},
				{
					ID:             "go-test",
					Command:        "go test ./...",
					Type:           "run",
					Importance:     domain.StepOptional,
					Risk:           domain.RiskLow,
					EvidenceSource: "manifest",
				},
			},
		},
	})

	requireStepStatus(t, response, "run-node-script-dev", AutoSetupStepLaunchOnly)
	requireStepStatus(t, response, "run-node-script-build", AutoSetupStepManual)
	requireStepStatus(t, response, "go-test", AutoSetupStepManual)
}

func TestAutoSetupEligibilityRepresentsInstallScriptPolicy(t *testing.T) {
	request := AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
				},
			},
		},
	}

	normal := ClassifyAutoSetupSteps(request)
	if normal.InstallScriptPolicy != InstallScriptPolicyNormalLifecycleScripts {
		t.Fatalf("expected default install script policy %q, got %q", InstallScriptPolicyNormalLifecycleScripts, normal.InstallScriptPolicy)
	}
	requireStepCommandPreview(t, normal, "install-node-deps", "npm install")

	request.InstallScriptPolicy = InstallScriptPolicySkipLifecycleScripts
	skip := ClassifyAutoSetupSteps(request)
	if skip.InstallScriptPolicy != InstallScriptPolicySkipLifecycleScripts {
		t.Fatalf("expected install script policy %q, got %q", InstallScriptPolicySkipLifecycleScripts, skip.InstallScriptPolicy)
	}
	requireStepCommandPreview(t, skip, "install-node-deps", "npm install --ignore-scripts")
}

func TestAutoSetupEligibilityStopsAfterFailedPriorDependency(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "install-node-deps",
					Command:        "npm install",
					Type:           "dependency-install",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
				},
				{
					ID:             "start-local-services",
					Command:        `docker compose -f "docker-compose.yml" up -d`,
					Type:           "service-start",
					Importance:     domain.StepRequired,
					Risk:           domain.RiskMedium,
					EvidenceSource: "config",
				},
			},
		},
		PriorStepStatuses: map[string]string{
			"install-node-deps": domain.StepRunStatusFailed,
		},
	})

	requireStepStatus(t, response, "install-node-deps", AutoSetupStepAttentionStop)
	requireStepStatus(t, response, "start-local-services", AutoSetupStepAttentionStop)
}

func TestAutoSetupEligibilityMarksManualStepsManual(t *testing.T) {
	response := ClassifyAutoSetupSteps(AutoSetupEligibilityRequest{
		Plan: domain.SetupPlan{
			Steps: []domain.ExecutionStep{
				{
					ID:             "manual-migrate",
					Command:        "manual database migration required",
					Type:           "review",
					Importance:     domain.StepManual,
					Risk:           domain.RiskMedium,
					EvidenceSource: "manifest",
				},
			},
		},
	})

	requireStepStatus(t, response, "manual-migrate", AutoSetupStepManual)
}

func requireStepStatus(t *testing.T, response AutoSetupEligibilityResponse, stepID, want string) {
	t.Helper()

	for _, step := range response.Steps {
		if step.StepID != stepID {
			continue
		}
		if step.Status != want {
			t.Fatalf("expected step %q status %q, got %q", stepID, want, step.Status)
		}
		return
	}

	t.Fatalf("expected step %q in response, got %+v", stepID, response.Steps)
}

func requireStepCommandPreview(t *testing.T, response AutoSetupEligibilityResponse, stepID, want string) {
	t.Helper()

	for _, step := range response.Steps {
		if step.StepID != stepID {
			continue
		}
		if step.CommandPreview != want {
			t.Fatalf("expected step %q command preview %q, got %q", stepID, want, step.CommandPreview)
		}
		return
	}

	t.Fatalf("expected step %q in response, got %+v", stepID, response.Steps)
}

func requireStepReasonContains(t *testing.T, response AutoSetupEligibilityResponse, stepID, want string) {
	t.Helper()

	for _, step := range response.Steps {
		if step.StepID != stepID {
			continue
		}
		if !strings.Contains(step.Reason, want) {
			t.Fatalf("expected step %q reason to contain %q, got %q", stepID, want, step.Reason)
		}
		return
	}

	t.Fatalf("expected step %q in response, got %+v", stepID, response.Steps)
}
