package analyzer

import (
	"path/filepath"

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
		targetPath := filepath.Join(targetDir, ".env")
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
