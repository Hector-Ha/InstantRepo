package main

import (
	"context"
	"os"

	"instantrepo/internal/command"
)

var (
	appVersion = "dev"
	gitCommit  = ""
)

func main() {
	os.Exit(command.Run(context.Background(), command.Options{
		Args:    os.Args[1:],
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: versionInfo(),
	}))
}

func versionInfo() command.VersionInfo {
	return command.VersionInfo{
		AppVersion:         appVersion,
		GitCommit:          gitCommit,
		CLIContractVersion: command.CLIContractVersion,
	}
}
