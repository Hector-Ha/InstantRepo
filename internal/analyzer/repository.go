package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/util"
)

type RepositoryAnalyzer struct{}

func NewRepositoryAnalyzer() *RepositoryAnalyzer {
	return &RepositoryAnalyzer{}
}

func (a *RepositoryAnalyzer) Analyze(repoPath string) (domain.RepositoryAnalysis, error) {
	info, err := os.Stat(repoPath)
	if err != nil {
		return domain.RepositoryAnalysis{}, fmt.Errorf("stat repository: %w", err)
	}
	if !info.IsDir() {
		return domain.RepositoryAnalysis{}, fmt.Errorf("repository path must be a directory")
	}

	analysis := domain.RepositoryAnalysis{
		ProjectName:  inferProjectName(repoPath),
		RepoPath:     repoPath,
		Confidence:   0.35,
		Evidence:     []string{},
		Requirements: []domain.ToolRequirement{},
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{},
		},
		Services: []domain.ServiceDependency{},
		Steps:    []domain.ExecutionStep{},
		Unknowns: []string{},
	}

	readmePath := filepath.Join(repoPath, "README.md")
	if util.FileExists(readmePath) {
		analysis.Evidence = append(analysis.Evidence, "README.md found")
	}

	if util.FileExists(filepath.Join(repoPath, "package.json")) {
		nodeAnalysis, err := a.analyzeNodeProject(repoPath)
		if err == nil {
			return nodeAnalysis, nil
		}
		analysis.Unknowns = append(analysis.Unknowns, "package.json found but could not be parsed")
	}

	if util.FileExists(filepath.Join(repoPath, "requirements.txt")) || util.FileExists(filepath.Join(repoPath, "pyproject.toml")) {
		return a.analyzePythonProject(repoPath), nil
	}

	if util.FileExists(filepath.Join(repoPath, "go.mod")) {
		analysis.ProjectType = "go-project"
		analysis.Confidence = 0.65
		analysis.Requirements = append(analysis.Requirements, domain.ToolRequirement{
			Tool:              "go",
			VersionConstraint: "",
			Source:            "go.mod",
			Required:          true,
		})
		analysis.Steps = append(analysis.Steps,
			domain.ExecutionStep{
				ID:               "go-mod-download",
				Title:            "Download Go modules",
				Command:          "go mod download",
				Cwd:              repoPath,
				Type:             "dependency-install",
				Importance:       domain.StepRequired,
				Risk:             domain.RiskMedium,
				RequiresApproval: true,
				EvidenceSource:   "manifest",
				ConfirmedBy:      []string{"go.mod"},
				Confidence:       0.92,
				Reason:           "Go dependencies need to be downloaded before running the project.",
			},
			domain.ExecutionStep{
				ID:               "go-run",
				Title:            "Run Go project",
				Command:          "go run .",
				Cwd:              repoPath,
				Type:             "run",
				Importance:       domain.StepRecommended,
				Risk:             domain.RiskLow,
				RequiresApproval: true,
				EvidenceSource:   "manifest",
				ConfirmedBy:      []string{"go.mod"},
				Confidence:       0.74,
				Reason:           "Common default command for a single-module Go project.",
			},
		)
		analysis.Evidence = append(analysis.Evidence, "go.mod found")
		analysis.Unknowns = append(analysis.Unknowns, "Go project support is minimal in the current MVP")
		a.enrichRuntimeContext(&analysis)
		a.enrichReadmeContext(&analysis)
		return analysis, nil
	}

	analysis.ProjectType = "unknown"
	analysis.Unknowns = append(analysis.Unknowns, "No supported manifest file found")
	a.enrichRuntimeContext(&analysis)
	a.enrichReadmeContext(&analysis)
	return analysis, nil
}

func (a *RepositoryAnalyzer) analyzeNodeProject(repoPath string) (domain.RepositoryAnalysis, error) {
	raw, err := os.ReadFile(filepath.Join(repoPath, "package.json"))
	if err != nil {
		return domain.RepositoryAnalysis{}, err
	}

	var pkg struct {
		Name    string            `json:"name"`
		Scripts map[string]string `json:"scripts"`
		Engines map[string]string `json:"engines"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return domain.RepositoryAnalysis{}, err
	}

	manager := "npm"
	switch {
	case util.FileExists(filepath.Join(repoPath, "pnpm-lock.yaml")):
		manager = "pnpm"
	case util.FileExists(filepath.Join(repoPath, "yarn.lock")):
		manager = "yarn"
	case util.FileExists(filepath.Join(repoPath, "package-lock.json")):
		manager = "npm"
	}

	projectName := pkg.Name
	if strings.TrimSpace(projectName) == "" {
		projectName = inferProjectName(repoPath)
	}

	analysis := domain.RepositoryAnalysis{
		ProjectName: projectName,
		ProjectType: "node-project",
		RepoPath:    repoPath,
		Confidence:  0.9,
		Evidence: []string{
			"package.json found",
			fmt.Sprintf("%s selected as package manager", manager),
		},
		Requirements: []domain.ToolRequirement{},
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{},
		},
		Services: []domain.ServiceDependency{},
		Steps:    []domain.ExecutionStep{},
		Unknowns: []string{},
	}

	nodeVersion := strings.TrimSpace(pkg.Engines["node"])
	if nodeVersion == "" && util.FileExists(filepath.Join(repoPath, ".nvmrc")) {
		nodeVersion = strings.TrimSpace(util.ReadTextFile(filepath.Join(repoPath, ".nvmrc")))
		analysis.Evidence = append(analysis.Evidence, ".nvmrc found")
	}

	if nodeVersion == "" {
		nodeVersion = "unspecified"
		analysis.Unknowns = append(analysis.Unknowns, "Node version was not declared in package.json engines or .nvmrc")
	}

	analysis.Requirements = append(analysis.Requirements,
		domain.ToolRequirement{
			Tool:              "node",
			VersionConstraint: nodeVersion,
			Source:            "package.json/.nvmrc",
			Required:          true,
		},
		domain.ToolRequirement{
			Tool:              manager,
			VersionConstraint: "",
			Source:            "package manager detection",
			Required:          true,
		},
	)

	analysis.Steps = append(analysis.Steps, domain.ExecutionStep{
		ID:               "install-node-deps",
		Title:            "Install project dependencies",
		Command:          fmt.Sprintf("%s install", manager),
		Cwd:              repoPath,
		Type:             "dependency-install",
		Importance:       domain.StepRequired,
		Risk:             domain.RiskMedium,
		RequiresApproval: true,
		EvidenceSource:   "manifest",
		ConfirmedBy:      []string{"package.json"},
		Confidence:       0.95,
		Reason:           "Node dependencies need to be installed before the project can run.",
	})

	for _, candidate := range []string{"dev", "start", "build"} {
		if _, ok := pkg.Scripts[candidate]; ok {
			importance := domain.StepRecommended
			confidence := 0.88
			if candidate == "build" {
				importance = domain.StepOptional
				confidence = 0.72
			}
			analysis.Steps = append(analysis.Steps, domain.ExecutionStep{
				ID:               "run-node-script-" + candidate,
				Title:            fmt.Sprintf("Run npm script %q", candidate),
				Command:          fmt.Sprintf("%s run %s", manager, candidate),
				Cwd:              repoPath,
				Type:             "run",
				Importance:       importance,
				Risk:             domain.RiskLow,
				RequiresApproval: true,
				EvidenceSource:   "manifest",
				ConfirmedBy:      []string{"package.json:scripts." + candidate},
				Confidence:       confidence,
				Reason:           fmt.Sprintf("Script %q is declared in package.json.", candidate),
			})
		}
	}

	if len(analysis.Steps) == 1 {
		analysis.Unknowns = append(analysis.Unknowns, "No standard run script like dev or start was found in package.json")
	}

	if slices.Contains([]string{"pnpm", "yarn"}, manager) {
		analysis.Evidence = append(analysis.Evidence, fmt.Sprintf("Lockfile indicates %s usage", manager))
	}

	a.enrichRuntimeContext(&analysis)
	a.enrichReadmeContext(&analysis)
	return analysis, nil
}

func (a *RepositoryAnalyzer) analyzePythonProject(repoPath string) domain.RepositoryAnalysis {
	analysis := domain.RepositoryAnalysis{
		ProjectName: inferProjectName(repoPath),
		ProjectType: "python-project",
		RepoPath:    repoPath,
		Confidence:  0.82,
		Evidence: []string{
			"Python manifest detected",
		},
		Requirements: []domain.ToolRequirement{},
		Env: domain.EnvironmentConfig{
			Variables: []domain.EnvVarRequirement{},
		},
		Services: []domain.ServiceDependency{},
		Steps:    []domain.ExecutionStep{},
		Unknowns: []string{},
	}

	pythonVersion := ""
	if util.FileExists(filepath.Join(repoPath, ".python-version")) {
		pythonVersion = strings.TrimSpace(util.ReadTextFile(filepath.Join(repoPath, ".python-version")))
		analysis.Evidence = append(analysis.Evidence, ".python-version found")
	}
	if pythonVersion == "" && util.FileExists(filepath.Join(repoPath, "pyproject.toml")) {
		pythonVersion = extractPythonConstraint(util.ReadTextFile(filepath.Join(repoPath, "pyproject.toml")))
		if pythonVersion != "" {
			analysis.Evidence = append(analysis.Evidence, "Python version inferred from pyproject.toml")
		}
	}
	if pythonVersion == "" {
		pythonVersion = "unspecified"
		analysis.Unknowns = append(analysis.Unknowns, "Python version was not declared")
	}

	analysis.Requirements = append(analysis.Requirements,
		domain.ToolRequirement{
			Tool:              "python",
			VersionConstraint: pythonVersion,
			Source:            "pyproject.toml/.python-version",
			Required:          true,
		},
		domain.ToolRequirement{
			Tool:              "pip",
			VersionConstraint: "",
			Source:            "Python package manager",
			Required:          true,
		},
	)

	if util.FileExists(filepath.Join(repoPath, "requirements.txt")) {
		analysis.Evidence = append(analysis.Evidence, "requirements.txt found")
		analysis.Steps = append(analysis.Steps, domain.ExecutionStep{
			ID:               "pip-install",
			Title:            "Install Python dependencies",
			Command:          "python -m pip install -r requirements.txt",
			Cwd:              repoPath,
			Type:             "dependency-install",
			Importance:       domain.StepRequired,
			Risk:             domain.RiskMedium,
			RequiresApproval: true,
			EvidenceSource:   "manifest",
			ConfirmedBy:      []string{"requirements.txt"},
			Confidence:       0.94,
			Reason:           "requirements.txt lists Python dependencies needed by the project.",
		})
	}

	if util.FileExists(filepath.Join(repoPath, "main.py")) {
		analysis.Steps = append(analysis.Steps, domain.ExecutionStep{
			ID:               "python-main",
			Title:            "Run main.py",
			Command:          "python main.py",
			Cwd:              repoPath,
			Type:             "run",
			Importance:       domain.StepRecommended,
			Risk:             domain.RiskLow,
			RequiresApproval: true,
			EvidenceSource:   "heuristic",
			ConfirmedBy:      []string{"main.py"},
			Confidence:       0.72,
			Reason:           "main.py found in the repository root.",
		})
	} else if util.FileExists(filepath.Join(repoPath, "app.py")) {
		analysis.Steps = append(analysis.Steps, domain.ExecutionStep{
			ID:               "python-app",
			Title:            "Run app.py",
			Command:          "python app.py",
			Cwd:              repoPath,
			Type:             "run",
			Importance:       domain.StepRecommended,
			Risk:             domain.RiskLow,
			RequiresApproval: true,
			EvidenceSource:   "heuristic",
			ConfirmedBy:      []string{"app.py"},
			Confidence:       0.68,
			Reason:           "app.py found in the repository root.",
		})
	} else {
		analysis.Unknowns = append(analysis.Unknowns, "No obvious Python entrypoint like main.py or app.py was found")
	}

	a.enrichRuntimeContext(&analysis)
	a.enrichReadmeContext(&analysis)
	return analysis
}

func extractPythonConstraint(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "requires-python"):
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		case strings.Contains(trimmed, "python ="):
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			}
		}
	}
	return ""
}

var workspaceSuffixPattern = regexp.MustCompile(`-[a-f0-9]{8}$`)

func inferProjectName(repoPath string) string {
	name := filepath.Base(repoPath)
	return workspaceSuffixPattern.ReplaceAllString(name, "")
}
