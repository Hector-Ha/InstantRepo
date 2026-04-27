package analyzer

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"instantrepo/internal/domain"
	"instantrepo/internal/util"
)

var (
	readmeHeadingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*$`)
	readmeFencePattern   = regexp.MustCompile("^```([A-Za-z0-9_-]+)?\\s*$")
)

type readmeCommand struct {
	Command    string
	Section    string
	StepType   string
	Importance string
	Reason     string
	Confidence float64
}

func (a *RepositoryAnalyzer) enrichReadmeContext(analysis *domain.RepositoryAnalysis) {
	if analysis == nil {
		return
	}

	commands, evidence, unknowns := parseReadmeContext(analysis.RepoPath)
	for _, item := range evidence {
		if !slices.Contains(analysis.Evidence, item) {
			analysis.Evidence = append(analysis.Evidence, item)
		}
	}
	for _, item := range unknowns {
		if !slices.Contains(analysis.Unknowns, item) {
			analysis.Unknowns = append(analysis.Unknowns, item)
		}
	}

	for _, item := range commands {
		step := stepFromReadmeCommand(analysis.RepoPath, item)
		mergeStep(analysis, step)
	}
}

func parseReadmeContext(repoPath string) ([]readmeCommand, []string, []string) {
	readmePath := filepath.Join(repoPath, "README.md")
	if !util.FileExists(readmePath) {
		return nil, nil, nil
	}

	raw := util.ReadTextFile(readmePath)
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	commands := []readmeCommand{}
	evidence := []string{"README.md parsed for setup hints"}
	unknowns := []string{}

	currentHeading := "readme"
	inFence := false
	fenceLang := ""
	blockLines := []string{}

	flushBlock := func() {
		if !isShellFence(fenceLang) || len(blockLines) == 0 {
			blockLines = nil
			return
		}

		sectionCommands := commandsFromFence(currentHeading, blockLines)
		commands = append(commands, sectionCommands...)
		blockLines = nil
	}

	for _, line := range lines {
		if matches := readmeHeadingPattern.FindStringSubmatch(strings.TrimSpace(line)); len(matches) == 2 && !inFence {
			currentHeading = strings.TrimSpace(matches[1])
			continue
		}

		if matches := readmeFencePattern.FindStringSubmatch(strings.TrimSpace(line)); len(matches) == 2 {
			if inFence {
				flushBlock()
				inFence = false
				fenceLang = ""
			} else {
				inFence = true
				fenceLang = strings.ToLower(strings.TrimSpace(matches[1]))
				blockLines = []string{}
			}
			continue
		}

		if inFence {
			blockLines = append(blockLines, line)
		}
	}

	if inFence {
		flushBlock()
		unknowns = append(unknowns, "README.md contains an unterminated code fence; extracted commands may be incomplete")
	}

	if len(commands) == 0 {
		evidence = append(evidence, "README.md did not yield structured shell commands")
	}

	return dedupeReadmeCommands(commands), evidence, unknowns
}

func commandsFromFence(section string, lines []string) []readmeCommand {
	section = strings.TrimSpace(section)
	result := []readmeCommand{}
	stepType, importance, baseConfidence := classifyReadmeSection(section)

	for _, rawLine := range lines {
		command := normalizeReadmeCommand(rawLine)
		if command == "" || !looksLikeCommand(command) {
			continue
		}

		reason := fmt.Sprintf("Command extracted from README section %q.", section)
		confidence := baseConfidence
		if stepType == "run" && strings.Contains(strings.ToLower(command), "build") {
			importance = domain.StepOptional
			confidence = 0.55
		}
		if stepType == "unknown" {
			importance = domain.StepUncertain
			confidence = 0.45
		}

		result = append(result, readmeCommand{
			Command:    command,
			Section:    section,
			StepType:   stepType,
			Importance: importance,
			Reason:     reason,
			Confidence: confidence,
		})
	}

	return result
}

func classifyReadmeSection(section string) (string, string, float64) {
	lower := strings.ToLower(section)
	switch {
	case strings.Contains(lower, "install"), strings.Contains(lower, "setup"), strings.Contains(lower, "getting started"):
		return "dependency-install", domain.StepRequired, 0.72
	case strings.Contains(lower, "develop"), strings.Contains(lower, "run"), strings.Contains(lower, "start"), strings.Contains(lower, "usage"):
		return "run", domain.StepRecommended, 0.68
	case strings.Contains(lower, "build"):
		return "run", domain.StepOptional, 0.60
	case strings.Contains(lower, "docker"), strings.Contains(lower, "database"), strings.Contains(lower, "service"):
		return "service-start", domain.StepRecommended, 0.64
	case strings.Contains(lower, "env"), strings.Contains(lower, "environment"):
		return "env-setup", domain.StepManual, 0.58
	case strings.Contains(lower, "test"):
		return "run", domain.StepOptional, 0.52
	default:
		return "unknown", domain.StepUncertain, 0.45
	}
}

func isShellFence(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "sh", "bash", "shell", "zsh", "powershell", "ps1", "cmd", "console":
		return true
	default:
		return false
	}
}

func normalizeReadmeCommand(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	trimmed = strings.TrimPrefix(trimmed, "$ ")
	trimmed = strings.TrimPrefix(trimmed, "> ")
	trimmed = strings.TrimPrefix(trimmed, "PS> ")
	trimmed = strings.TrimPrefix(trimmed, "PS > ")

	if strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "||") {
		return trimmed
	}
	return trimmed
}

func looksLikeCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	if lower == "" {
		return false
	}

	prefixes := []string{
		"npm ", "pnpm ", "yarn ", "node ", "python ", "python3 ", "pip ", "pip3 ",
		"go ", "docker ", "docker-compose ", "cargo ", "make ", "bun ", "uv ",
		"cp ", "copy ", "mv ", "set ", "export ", "npx ", "pnpx ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func stepFromReadmeCommand(repoPath string, item readmeCommand) domain.ExecutionStep {
	return domain.ExecutionStep{
		ID:               "readme-" + shortHash(item.Section+"|"+item.Command),
		Title:            readmeStepTitle(item),
		Command:          item.Command,
		Cwd:              repoPath,
		Type:             item.StepType,
		Importance:       item.Importance,
		Risk:             riskForReadmeStep(item),
		RequiresApproval: true,
		EvidenceSource:   "readme",
		ConfirmedBy:      []string{"README.md: " + item.Section},
		Confidence:       item.Confidence,
		Reason:           item.Reason,
	}
}

func readmeStepTitle(item readmeCommand) string {
	switch item.StepType {
	case "dependency-install":
		return fmt.Sprintf("README install command from %q", item.Section)
	case "service-start":
		return fmt.Sprintf("README service command from %q", item.Section)
	case "env-setup":
		return fmt.Sprintf("README environment command from %q", item.Section)
	default:
		return fmt.Sprintf("README command from %q", item.Section)
	}
}

func riskForReadmeStep(item readmeCommand) string {
	lower := strings.ToLower(item.Command)
	switch {
	case strings.Contains(lower, "sudo "), strings.Contains(lower, "rm -rf"), strings.Contains(lower, "curl ") && strings.Contains(lower, "|"):
		return domain.RiskHigh
	case strings.Contains(lower, "install"), strings.Contains(lower, "compose up"), strings.Contains(lower, "migrate"):
		return domain.RiskMedium
	default:
		return domain.RiskLow
	}
}

func mergeStep(analysis *domain.RepositoryAnalysis, candidate domain.ExecutionStep) {
	for i := range analysis.Steps {
		if sameCommand(analysis.Steps[i].Command, candidate.Command) {
			if analysis.Steps[i].EvidenceSource == "" {
				analysis.Steps[i].EvidenceSource = "manifest"
			}
			if analysis.Steps[i].Confidence == 0 {
				analysis.Steps[i].Confidence = 0.9
			}
			analysis.Steps[i].ConfirmedBy = appendUnique(analysis.Steps[i].ConfirmedBy, candidate.ConfirmedBy...)
			if analysis.Steps[i].EvidenceSource == "manifest" {
				analysis.Steps[i].EvidenceSource = "manifest+readme"
			} else if analysis.Steps[i].EvidenceSource == "heuristic" {
				analysis.Steps[i].EvidenceSource = "heuristic+readme"
			}
			if candidate.Confidence > analysis.Steps[i].Confidence {
				analysis.Steps[i].Confidence = candidate.Confidence
			}
			if analysis.Steps[i].Importance == "" || analysis.Steps[i].Importance == domain.StepUncertain {
				analysis.Steps[i].Importance = candidate.Importance
			}
			return
		}
	}

	analysis.Steps = append(analysis.Steps, candidate)
}

func sameCommand(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func appendUnique(existing []string, additions ...string) []string {
	for _, item := range additions {
		if item == "" || slices.Contains(existing, item) {
			continue
		}
		existing = append(existing, item)
	}
	return existing
}

func dedupeReadmeCommands(items []readmeCommand) []readmeCommand {
	result := make([]readmeCommand, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Command))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func shortHash(input string) string {
	sum := sha1.Sum([]byte(input))
	return hex.EncodeToString(sum[:])[:8]
}
