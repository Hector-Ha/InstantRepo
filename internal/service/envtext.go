package service

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	envLinePattern      = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$`)
	envLinePartsPattern = regexp.MustCompile(`^(\s*(?:export\s+)?)([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$`)
)

func formatEnvAssignment(name, value string) string {
	if needsQuoting(value) {
		return fmt.Sprintf("%s=%q", name, value)
	}
	return fmt.Sprintf("%s=%s", name, value)
}

func formatEnvAssignmentLike(line, name, value string) string {
	prefix := ""
	if matches := envLinePartsPattern.FindStringSubmatch(strings.TrimRight(line, "\r")); len(matches) == 4 {
		prefix = matches[1]
	}
	return prefix + formatEnvAssignment(name, value)
}

func needsQuoting(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r == ' ' || r == '#' || r == '"' {
			return true
		}
	}
	return false
}

func cleanWriteValue(value string) string {
	trimmed := strings.TrimSpace(value)
	return strings.Trim(trimmed, `"'`)
}

func parseEnvAssignments(content string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		matches := envLinePattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if len(matches) != 3 {
			continue
		}
		values[matches[1]] = cleanWriteValue(matches[2])
	}
	return values
}

func relativeEnvTargetPath(repoPath, targetPath string) string {
	relative, err := filepath.Rel(repoPath, targetPath)
	if err != nil {
		return filepath.Base(targetPath)
	}
	return relative
}

func redactServiceCredentialAssignments(content string, serviceCredentialNames map[string]bool) string {
	if len(serviceCredentialNames) == 0 {
		return content
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, line := range lines {
		matches := envLinePattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if len(matches) != 3 {
			continue
		}
		name := matches[1]
		if !serviceCredentialNames[name] {
			continue
		}
		if strings.TrimSpace(matches[2]) == "" {
			continue
		}
		lines[i] = formatEnvAssignmentLike(line, name, "")
	}
	return strings.Join(lines, "\n")
}

func replaceEnvAssignments(content string, values map[string]string) string {
	if len(values) == 0 {
		return content
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, line := range lines {
		matches := envLinePattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if len(matches) != 3 {
			continue
		}
		value, ok := values[matches[1]]
		if !ok {
			continue
		}
		lines[i] = formatEnvAssignmentLike(line, matches[1], value)
	}
	return strings.Join(lines, "\n")
}
