//go:build windows

package service

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type osDiskChecker struct{}

func (osDiskChecker) FreeBytes(path string) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("encode disk path: %w", err)
	}

	var freeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytes, nil, nil); err != nil {
		return 0, fmt.Errorf("measure disk free space: %w", err)
	}
	return freeBytes, nil
}
