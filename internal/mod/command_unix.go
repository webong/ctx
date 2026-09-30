//go:build !windows

package mod

import (
	"context"
	"os/exec"
)

func adapterCommand(path string, args []string) *exec.Cmd { return exec.Command(path, args...) }

func adapterCommandContext(ctx context.Context, path string, args []string) *exec.Cmd {
	if ctx == nil {
		return adapterCommand(path, args)
	}
	return exec.CommandContext(ctx, path, args...)
}
