//go:build !windows

package adapter

import "os/exec"

func adapterCommand(path string, args []string) *exec.Cmd { return exec.Command(path, args...) }
