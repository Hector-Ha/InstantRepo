package service

import (
	"context"
	"os"
	"strings"
	"testing"

	"instantrepo/internal/domain"
)

func TestRunStepWithEventsStreamsOutput(t *testing.T) {
	tempDir := t.TempDir()
	executor := NewExecutor()
	events := make([]ExecutionEvent, 0, 4)

	result, err := executor.RunStepWithEvents(context.Background(), domain.ExecutionStep{
		ID:      "echo-test",
		Title:   "Echo test",
		Command: "echo instantrepo-stream-test",
		Cwd:     tempDir,
		Type:    "run",
	}, func(event ExecutionEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("RunStepWithEvents returned error: %v", err)
	}

	if !result.Succeeded {
		t.Fatalf("expected success, got result: %+v", result)
	}
	if result.ProcessID <= 0 {
		t.Fatalf("expected process id to be set, got %d", result.ProcessID)
	}
	if !strings.Contains(result.Stdout, "instantrepo-stream-test") {
		t.Fatalf("expected stdout to contain test marker, got %q", result.Stdout)
	}

	var kinds []string
	var sawOutput bool
	for _, event := range events {
		kinds = append(kinds, event.Kind)
		if event.Kind == "output" && strings.Contains(event.Message, "instantrepo-stream-test") {
			sawOutput = true
		}
		if event.ProcessID != result.ProcessID {
			t.Fatalf("expected event process id %d, got %d", result.ProcessID, event.ProcessID)
		}
	}

	if len(kinds) == 0 {
		t.Fatal("expected streaming events, got none")
	}
	if kinds[0] != "started" {
		t.Fatalf("expected first event to be started, got %q", kinds[0])
	}
	if kinds[len(kinds)-1] != "finished" {
		t.Fatalf("expected last event to be finished, got %q", kinds[len(kinds)-1])
	}
	if !sawOutput {
		t.Fatalf("expected output event containing marker, got %+v", events)
	}

	if _, err := os.Stat(tempDir); err != nil {
		t.Fatalf("expected working directory to exist: %v", err)
	}
}
