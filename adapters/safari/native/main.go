package main

import (
	"fmt"
	"io"
	"os"
	"runtime"

	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 || args[0] != "default" || args[1] != "policy" || args[2] != "export" || runtime.GOOS != "darwin" {
		fmt.Fprintln(stderr, "ctx: Safari sharing supports policy export from the default macOS profile")
		return 2
	}
	return kit.RunPolicyExport(input, stdout, stderr, kit.PolicySources{Files: kit.ManagedPreferenceFiles("com.apple.Safari")})
}
