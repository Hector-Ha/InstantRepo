package analyzer

import (
	"path/filepath"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/util"
)

type envTargetInference struct {
	envConfig domain.EnvironmentConfig
	evidence  []string
	unknowns  []string
}

func inferEnvTargets(repoPath string) envTargetInference {
	envConfig := domain.EnvironmentConfig{
		Variables: []domain.EnvVarRequirement{},
	}
	evidence := []string{}
	unknowns := []string{}

	envFiles := findEnvFiles(repoPath)
	for _, envFile := range envFiles {
		targetDir := filepath.Dir(envFile.Path)
		targetPath := envWriteTargetPath(envFile)
		vars := parseEnvTemplate(envFile.Path)
		for i := range vars {
			vars[i].Confidence = envFile.Confidence
			if envFile.Role != envFileRoleNonLocal {
				vars[i].TargetDir = targetDir
			}
		}

		switch envFile.Role {
		case envFileRoleLocal, envFileRoleTemplate:
			if envConfig.TargetPath == "" {
				envConfig.TemplatePath = envFile.Path
				envConfig.TargetPath = targetPath
			}
			if util.FileExists(targetPath) {
				envConfig.TargetExists = true
			}
			evidence = append(evidence, filepath.Base(envFile.Path)+" found in "+targetDir)
		case envFileRoleNonLocal:
			evidence = append(evidence, filepath.Base(envFile.Path)+" used as env evidence only")
		}

		envConfig.Variables = mergeEnvVars(envConfig.Variables, vars)
	}

	codeScannedVars, inferredTargetDir, sourceFixes, codeUnknowns := scanCodeForEnvVars(repoPath)
	if len(codeScannedVars) > 0 {
		if shouldKeepCodeScannedVarsInRootEnv(repoPath, envFiles) {
			for i := range codeScannedVars {
				if strings.TrimSpace(codeScannedVars[i].TargetDir) != "" {
					codeScannedVars[i].TargetDir = repoPath
				}
			}
			inferredTargetDir = repoPath
			evidence = append(evidence, "README indicates one root .env file for the repo")
		}
		if envConfig.TargetPath == "" && inferredTargetDir != "" {
			envConfig.TargetPath = filepath.Join(inferredTargetDir, ".env")
			envConfig.TargetExists = util.FileExists(envConfig.TargetPath)
		}
		envConfig.Variables = mergeEnvVars(envConfig.Variables, codeScannedVars)
		evidence = append(evidence, "Environment variables inferred from source code scan")
	}
	envConfig.SourceFixSuggestions = append(envConfig.SourceFixSuggestions, sourceFixes...)
	unknowns = append(unknowns, codeUnknowns...)

	return envTargetInference{
		envConfig: envConfig,
		evidence:  evidence,
		unknowns:  unknowns,
	}
}

func envWriteTargetPath(envFile envFileFinding) string {
	targetDir := filepath.Dir(envFile.Path)
	if envFile.Role == envFileRoleLocal && preservesLocalEnvFileName(filepath.Base(envFile.Path)) {
		return envFile.Path
	}
	return filepath.Join(targetDir, ".env")
}

func preservesLocalEnvFileName(name string) bool {
	switch strings.ToLower(name) {
	case ".env.local", ".env.development", ".env.dev":
		return true
	default:
		return false
	}
}

func shouldKeepCodeScannedVarsInRootEnv(repoPath string, envFiles []envFileFinding) bool {
	hasRootTemplate := false
	for _, envFile := range envFiles {
		if envFile.Role == envFileRoleNonLocal {
			continue
		}
		targetDir := filepath.Dir(envFile.Path)
		if targetDir == repoPath && envFile.Role == envFileRoleTemplate && envWriteTargetPath(envFile) == filepath.Join(repoPath, ".env") {
			hasRootTemplate = true
			continue
		}
		if targetDir != repoPath {
			return false
		}
	}
	return hasRootTemplate && repoDocsSaySingleRootEnv(repoPath)
}

func repoDocsSaySingleRootEnv(repoPath string) bool {
	for _, rel := range []string{"README.md", "README", filepath.Join("docs", "setup.md"), filepath.Join("docs", "SETUP.md")} {
		path := filepath.Join(repoPath, rel)
		if !util.FileExists(path) {
			continue
		}
		if textSaysSingleRootEnv(util.ReadTextFile(path)) {
			return true
		}
	}
	return false
}

func textSaysSingleRootEnv(raw string) bool {
	text := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	if !strings.Contains(text, ".env") && !strings.Contains(text, "env file") {
		return false
	}
	singleFile := strings.Contains(text, "single env file") ||
		strings.Contains(text, "single .env file") ||
		strings.Contains(text, "one env file") ||
		strings.Contains(text, "one .env file")
	rootScope := strings.Contains(text, "repo root") ||
		strings.Contains(text, "root .env") ||
		strings.Contains(text, ".env in the root") ||
		strings.Contains(text, ".env at the root") ||
		strings.Contains(text, "monorepo")
	return singleFile && rootScope
}
