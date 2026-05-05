package detector

import (
	"os/exec"
	"runtime"
	"strings"

	"instantrepo/internal/domain"
)

type EnvironmentDetector struct{}

func NewEnvironmentDetector() *EnvironmentDetector {
	return &EnvironmentDetector{}
}

func (d *EnvironmentDetector) Detect() domain.EnvironmentReport {
	tools := []domain.DetectedTool{
		d.detectTool("git", []string{"--version"}),
		d.detectTool("node", []string{"--version"}),
		d.detectTool("bun", []string{"--version"}),
		d.detectTool("npm", []string{"--version"}),
		d.detectTool("pnpm", []string{"--version"}),
		d.detectTool("yarn", []string{"--version"}),
		d.detectPython(),
		d.detectPip(),
		d.detectTool("uv", []string{"--version"}),
		d.detectTool("go", []string{"version"}),
		d.detectTool("docker", []string{"--version"}),
	}

	return domain.EnvironmentReport{
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
		Tools: tools,
	}
}

func (d *EnvironmentDetector) detectTool(name string, versionArgs []string) domain.DetectedTool {
	path, err := exec.LookPath(name)
	if err != nil {
		return domain.DetectedTool{Name: name, Available: false}
	}

	version := runVersionCommand(path, versionArgs...)
	return domain.DetectedTool{
		Name:      name,
		Path:      path,
		Version:   version,
		Available: true,
	}
}

func (d *EnvironmentDetector) detectPython() domain.DetectedTool {
	for _, name := range []string{"python", "python3"} {
		tool := d.detectTool(name, []string{"--version"})
		if tool.Available {
			tool.Name = "python"
			return tool
		}
	}
	return domain.DetectedTool{Name: "python", Available: false}
}

func (d *EnvironmentDetector) detectPip() domain.DetectedTool {
	for _, name := range []string{"pip", "pip3"} {
		tool := d.detectTool(name, []string{"--version"})
		if tool.Available {
			tool.Name = "pip"
			return tool
		}
	}
	return domain.DetectedTool{Name: "pip", Available: false}
}

func runVersionCommand(binary string, args ...string) string {
	cmd := exec.Command(binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(output))
	line = strings.ReplaceAll(line, "\r\n", "\n")
	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = line[:idx]
	}
	return line
}
