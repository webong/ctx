//go:build !windows

package app

import "os"

func replaceVirtualizerRegistry(source, target string) error { return os.Rename(source, target) }
