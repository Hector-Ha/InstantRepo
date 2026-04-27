package service

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"instantrepo/internal/domain"
)

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) RunStep(ctx context.Context, step domain.ExecutionStep) (domain.ExecutionResult, error) {
	if step.Command == "" || strings.HasPrefix(strings.ToLower(step.Command), "manual ") || strings.Contains(step.Type, "review") {
		return domain.ExecutionResult{}, fmt.Errorf("step %q is not executable", step.ID)
	}

	started := time.Now()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", step.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-lc", step.Command)
	}
	cmd.Dir = step.Cwd

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := domain.ExecutionResult{
		StepID:    step.ID,
		Command:   step.Command,
		Cwd:       step.Cwd,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Duration:  time.Since(started).String(),
		Succeeded: err == nil,
	}

	if err == nil {
		return result, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, fmt.Errorf("run step %q: %w", step.ID, err)
}
