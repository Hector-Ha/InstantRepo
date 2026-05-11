//go:build !windows

package service

import "os"

func replaceEnvTarget(tempPath, targetPath string) error {
	return os.Rename(tempPath, targetPath)
}
