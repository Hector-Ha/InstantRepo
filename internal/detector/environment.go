package detector

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"instantrepo/internal/domain"
)

const versionCommandTimeout = time.Second

var lookPath = exec.LookPath

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
	path, err := lookupToolPath(name)
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

func lookupToolPath(name string) (string, error) {
	type lookupResult struct {
		path string
		err  error
	}

	result := make(chan lookupResult, 1)
	go func() {
		path, err := lookPath(name)
		result <- lookupResult{path: path, err: err}
	}()

	select {
	case found := <-result:
		return found.path, found.err
	case <-time.After(versionCommandTimeout):
		return "", context.DeadlineExceeded
	}
}

func runVersionCommand(binary string, args ...string) string {
	cmd := exec.Command(binary, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		return ""
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timer := time.NewTimer(versionCommandTimeout)
	defer timer.Stop()

	select {
	case err := <-done:
		if err != nil {
			return ""
		}
	case <-timer.C:
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return ""
	}

	line := strings.TrimSpace(output.String())
	line = strings.ReplaceAll(line, "\r\n", "\n")
	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = line[:idx]
	}
	return line
}
