//go:build windows

package service

import (
	"context"
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	winCredTypeGeneric         = 1
	winCredPersistLocalMachine = 2
	winCredUserName            = "InstantRepo"
	winCredentialBlobMaxBytes  = 5 * 512
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredFree   = advapi32.NewProc("CredFree")
	procCredDelete = advapi32.NewProc("CredDeleteW")
)

type windowsCredentialStore struct{}

type winCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func NewOSCredentialStore() CredentialStore {
	return windowsCredentialStore{}
}

func (windowsCredentialStore) Put(_ context.Context, key, value string) error {
	target, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return fmt.Errorf("credential key: %w", err)
	}
	user, err := syscall.UTF16PtrFromString(winCredUserName)
	if err != nil {
		return fmt.Errorf("credential user: %w", err)
	}
	blob := utf16.Encode([]rune(value))
	blob = append(blob, 0)
	blobBytes := unsafe.Slice((*byte)(unsafe.Pointer(&blob[0])), len(blob)*2)
	if len(blobBytes) > winCredentialBlobMaxBytes {
		return fmt.Errorf("credential value too large for OS store")
	}
	credential := winCredential{
		Type:               winCredTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blobBytes)),
		CredentialBlob:     &blobBytes[0],
		Persist:            winCredPersistLocalMachine,
		UserName:           user,
	}
	ret, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&credential)), 0)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("%w: %v", ErrCredentialStoreUnavailable, callErr)
		}
		return ErrCredentialStoreUnavailable
	}
	return nil
}

func (windowsCredentialStore) Get(_ context.Context, key string) (string, error) {
	target, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return "", fmt.Errorf("credential key: %w", err)
	}
	var credentialPtr *winCredential
	ret, _, callErr := procCredRead.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(winCredTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&credentialPtr)),
	)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return "", fmt.Errorf("%w: %v", ErrCredentialUnavailable, callErr)
		}
		return "", ErrCredentialUnavailable
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(credentialPtr)))

	if credentialPtr.CredentialBlob == nil || credentialPtr.CredentialBlobSize == 0 {
		return "", ErrCredentialUnavailable
	}
	blobSize := int(credentialPtr.CredentialBlobSize)
	if blobSize <= 0 || blobSize > winCredentialBlobMaxBytes {
		return "", ErrCredentialUnavailable
	}
	raw := unsafe.Slice(credentialPtr.CredentialBlob, blobSize)
	words := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		word := uint16(raw[i]) | uint16(raw[i+1])<<8
		if word == 0 {
			break
		}
		words = append(words, word)
	}
	return string(utf16.Decode(words)), nil
}

func (windowsCredentialStore) Delete(_ context.Context, key string) error {
	target, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return fmt.Errorf("credential key: %w", err)
	}
	ret, _, callErr := procCredDelete.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(winCredTypeGeneric),
		0,
	)
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("%w: %v", ErrCredentialUnavailable, callErr)
		}
		return ErrCredentialUnavailable
	}
	return nil
}
