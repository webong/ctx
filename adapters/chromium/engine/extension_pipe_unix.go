//go:build !windows

package chromium

import (
	"os"
	"os/exec"
)

func startDebuggingPipe(command *exec.Cmd, incoming, outgoing *os.File) error {
	command.ExtraFiles = []*os.File{incoming, outgoing}
	return command.Start()
}
