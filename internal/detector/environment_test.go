package detector

import (
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestDetectToolReturnsUnavailableWhenLookPathHangs(t *testing.T) {
	originalLookPath := lookPath
	block := make(chan struct{})
	lookPath = func(string) (string, error) {
		<-block
		return "", errors.New("blocked")
	}
	t.Cleanup(func() {
		close(block)
		lookPath = originalLookPath
	})

	started := time.Now()
	tool := (&EnvironmentDetector{}).detectTool("blocked-tool", []string{"--version"})
	elapsed := time.Since(started)

	if tool.Available {
		t.Fatalf("expected blocked tool lookup to be unavailable, got %+v", tool)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected blocked tool lookup to be bounded, took %s", elapsed)
	}
}

func TestRunVersionCommandReturnsQuicklyWhenToolHangs(t *testing.T) {
	binary, args := slowVersionCommand()

	started := time.Now()
	version := runVersionCommand(binary, args...)
	elapsed := time.Since(started)

	if version != "" {
		t.Fatalf("expected no version from timed-out command, got %q", version)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected slow version command to be bounded, took %s", elapsed)
	}
}

func TestRunVersionCommandReturnsQuicklyWhenChildKeepsPipeOpen(t *testing.T) {
	binary, args := childHoldingVersionCommand()

	started := time.Now()
	version := runVersionCommand(binary, args...)
	elapsed := time.Since(started)

	if version != "" {
		t.Fatalf("expected no version from timed-out child command, got %q", version)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected child-held version command to be bounded, took %s", elapsed)
	}
}

func slowVersionCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", "Start-Sleep -Seconds 5; Write-Output done"}
	}
	return "sh", []string{"-c", "sleep 5; echo done"}
}

func childHoldingVersionCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", "Start-Process powershell -ArgumentList '-NoProfile','-Command','Start-Sleep -Seconds 5' -NoNewWindow; Start-Sleep -Seconds 5"}
	}
	return "sh", []string{"-c", "sh -c 'sleep 5' & sleep 5"}
}
