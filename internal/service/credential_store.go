//go:build !windows

package service

import "context"

type unavailableOSCredentialStore struct{}

func NewOSCredentialStore() CredentialStore {
	return unavailableOSCredentialStore{}
}

func (unavailableOSCredentialStore) Put(context.Context, string, string) error {
	return ErrCredentialStoreUnavailable
}

func (unavailableOSCredentialStore) Get(context.Context, string) (string, error) {
	return "", ErrCredentialStoreUnavailable
}

func (unavailableOSCredentialStore) Delete(context.Context, string) error {
	return ErrCredentialStoreUnavailable
}
