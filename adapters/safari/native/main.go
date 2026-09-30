package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"

	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	browsershare "github.com/webong/ctx/internal/app/browser/share"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 || runtime.GOOS != "darwin" {
		fmt.Fprintln(stderr, "ctx: Safari sharing requires macOS")
		return 2
	}
	switch args[1] {
	case "status":
		if args[2] == "probe" {
			return kit.Encode(stdout, browsershare.AvailabilityReport{Version: browsershare.AvailabilityVersion, Operations: safariShareStatus(args[0])})
		}
	case "cookie":
		if args[2] == "list" || args[2] == "export" || args[2] == "query" {
			return kit.RunCookie(args[0], args[2], input, stdout, stderr, kit.CookieBackend{
				List: readSafariSiteCookies, ReadValue: readSafariCookieValue, QueryValues: true,
				Query: func(profile string, site *url.URL, includeExpired bool) ([]browsershare.Cookie, string, error) {
					return querySafariCookies(profile, site, "", includeExpired, true)
				},
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
