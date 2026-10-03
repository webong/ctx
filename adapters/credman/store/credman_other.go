//go:build !windows

package store

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("Windows Credential Manager requires Windows")

func (CredentialManager) Check(context.Context) error                     { return errUnsupported }
func (CredentialManager) Get(context.Context, string) ([]byte, error)     { return nil, errUnsupported }
func (CredentialManager) Put(context.Context, string, []byte, bool) error { return errUnsupported }
