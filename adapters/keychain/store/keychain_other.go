//go:build !darwin || !cgo

package store

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("macOS Keychain requires a native macOS build with cgo")

func (Keychain) Check(context.Context) error                     { return errUnsupported }
func (Keychain) Get(context.Context, string) ([]byte, error)     { return nil, errUnsupported }
func (Keychain) Put(context.Context, string, []byte, bool) error { return errUnsupported }
