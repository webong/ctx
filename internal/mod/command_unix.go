//go:build !windows

package mod

import "os/exec"

func adapterCommand(path string, args []string) *exec.Cmd { return exec.Command(path, args...) }
