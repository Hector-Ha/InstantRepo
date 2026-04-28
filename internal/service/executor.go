package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"instantrepo/internal/domain"
)

type ExecutionEvent struct {
	Kind      string
	StepID    string
	ProcessID int
	Stream    string
	Message   string
}

type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) RunStep(ctx context.Context, step domain.ExecutionStep) (domain.ExecutionResult, error) {
	return e.RunStepWithEvents(ctx, step, nil)
}

func (e *Executor) RunStepWithEvents(ctx context.Context, step domain.ExecutionStep, onEvent func(ExecutionEvent)) (domain.ExecutionResult, error) {
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

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("create stdout pipe for step %q: %w", step.ID, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("create stderr pipe for step %q: %w", step.ID, err)
	}

	if err := cmd.Start(); err != nil {
		return domain.ExecutionResult{}, fmt.Errorf("start step %q: %w", step.ID, err)
	}

	result := domain.ExecutionResult{
		StepID:    step.ID,
		Command:   step.Command,
		Cwd:       step.Cwd,
		Succeeded: false,
	}
	if cmd.Process != nil {
		result.ProcessID = cmd.Process.Pid
	}

	emitEvent(onEvent, ExecutionEvent{
		Kind:      "started",
		StepID:    step.ID,
		ProcessID: result.ProcessID,
		Message:   fmt.Sprintf("Started process %d for step %s", result.ProcessID, step.ID),
	})

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		streamOutput(stdoutPipe, "stdout", step.ID, result.ProcessID, &stdoutBuf, onEvent)
	}()
	go func() {
		defer wg.Done()
		streamOutput(stderrPipe, "stderr", step.ID, result.ProcessID, &stderrBuf, onEvent)
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.Duration = time.Since(started).String()
	result.Succeeded = waitErr == nil

	if waitErr == nil {
		emitEvent(onEvent, ExecutionEvent{
			Kind:      "finished",
			StepID:    step.ID,
			ProcessID: result.ProcessID,
			Message:   fmt.Sprintf("Step %s finished successfully", step.ID),
		})
		return result, nil
	}

	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		emitEvent(onEvent, ExecutionEvent{
			Kind:      "finished",
			StepID:    step.ID,
			ProcessID: result.ProcessID,
			Message:   fmt.Sprintf("Step %s exited with code %d", step.ID, result.ExitCode),
		})
		return result, nil
	}

	return result, fmt.Errorf("run step %q: %w", step.ID, waitErr)
}

func streamOutput(reader io.Reader, stream, stepID string, processID int, aggregate *bytes.Buffer, onEvent func(ExecutionEvent)) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		aggregate.WriteString(line)
		aggregate.WriteString("\n")
		emitEvent(onEvent, ExecutionEvent{
			Kind:      "output",
			StepID:    stepID,
			ProcessID: processID,
			Stream:    stream,
			Message:   line,
		})
	}
}

func emitEvent(onEvent func(ExecutionEvent), event ExecutionEvent) {
	if onEvent != nil {
		onEvent(event)
	}
}
