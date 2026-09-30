package main

import (
	"fmt"
	"io"
	"os"
	"runtime"

	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	browsershare "github.com/webong/ctx/internal/app/browser/share"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 || args[0] != "default" || runtime.GOOS != "darwin" {
		fmt.Fprintln(stderr, "ctx: Safari sharing requires the default macOS profile")
		return 2
	}
	switch args[1] {
	case "status":
		if args[2] == "probe" {
			return kit.Encode(stdout, browsershare.AvailabilityReport{Version: browsershare.AvailabilityVersion, Operations: safariShareStatus()})
		}
	case "cookie":
		if args[2] == "list" || args[2] == "export" {
			return kit.RunCookie("default", args[2], input, stdout, stderr, kit.CookieBackend{
				List: readSafariSiteCookies, ReadValue: readSafariCookieValue,
			})
		}
	case "policy":
		if args[2] == "export" {
			return kit.RunPolicyExport(input, stdout, stderr, kit.PolicySources{Files: kit.ManagedPreferenceFiles("com.apple.Safari")})
		}
	}
	fmt.Fprintln(stderr, "ctx: unsupported Safari share resource or operation")
	return 2
}
