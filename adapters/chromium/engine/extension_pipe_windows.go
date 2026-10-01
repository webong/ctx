package chromium

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Chromium adopts inherited Windows HANDLEs from remote-debugging-io-pipes.
// Go's handle list restricts inheritance to the two child pipe ends and stdio.
// https://chromium.googlesource.com/chromium/src/+/main/content/browser/devtools/devtools_agent_host_impl.cc
func startDebuggingPipe(command *exec.Cmd, incoming, outgoing *os.File) error {
	handles := []syscall.Handle{syscall.Handle(incoming.Fd()), syscall.Handle(outgoing.Fd())}
	for _, handle := range handles {
		if uint64(handle) > uint64(^uint32(0)) {
			return fmt.Errorf("Chromium debugging pipe handle exceeds its 32-bit native format")
		}
	}
	for index, handle := range handles {
		if err := syscall.SetHandleInformation(handle, syscall.HANDLE_FLAG_INHERIT, syscall.HANDLE_FLAG_INHERIT); err != nil {
			return fmt.Errorf("make Chromium pipe inheritable: %w", err)
		}
		defer syscall.SetHandleInformation(handles[index], syscall.HANDLE_FLAG_INHERIT, 0)
	}
	command.SysProcAttr = &syscall.SysProcAttr{AdditionalInheritedHandles: handles}
	command.Args = append(command.Args, "--remote-debugging-io-pipes="+strconv.FormatUint(uint64(handles[0]), 10)+","+strconv.FormatUint(uint64(handles[1]), 10))
	return command.Start()
}
