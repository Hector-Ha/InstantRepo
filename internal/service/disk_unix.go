//go:build !windows

package service

import (
	"fmt"
	"syscall"
)

type osDiskChecker struct{}

func (osDiskChecker) FreeBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("measure disk free space: %w", err)
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
