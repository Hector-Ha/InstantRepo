package detector

import (
	"runtime"
	"testing"
	"time"
)

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

func slowVersionCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-Command", "Start-Sleep -Seconds 5; Write-Output done"}
	}
	return "sh", []string{"-c", "sleep 5; echo done"}
}
