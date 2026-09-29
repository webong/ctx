package main

import (
	"fmt"
	"io"
	"os"
	"runtime"

	native "github.com/webong/ctx/adapters/browsercommon"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 || args[0] != "default" || args[1] != "policy" || args[2] != "export" || runtime.GOOS != "darwin" {
		fmt.Fprintln(stderr, "ctx: Safari sharing supports policy export from the default macOS profile")
		return 2
	}
	return native.RunPolicyExport(input, stdout, stderr, native.PolicySources{Files: native.ManagedPreferenceFiles("com.apple.Safari")})
}
