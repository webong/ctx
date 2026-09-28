//go:build windows

package adapter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func adapterCommand(path string, args []string) *exec.Cmd {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cmd", ".bat":
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		return exec.Command(shell, append([]string{"/d", "/s", "/c", path}, args...)...)
	case ".ps1":
		return exec.Command("powershell.exe", append([]string{"-NoLogo", "-NoProfile", "-File", path}, args...)...)
	default:
		return exec.Command(path, args...)
	}
}
