package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"

	chromiumengine "github.com/webong/ctx/adapters/chromium/engine"
	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	"github.com/webong/ctx/internal/app/browser/share"
)

func chromeConfig() chromiumengine.Config {
	return chromiumengine.Config{
		Name: "chrome", MacUserData: "Google/Chrome", WindowsUserData: `Google\Chrome\User Data`, LinuxUserData: "google-chrome",
		KeychainService: "Chrome Safe Storage", KeychainAccount: "Chrome", SecretApplication: "chrome",
		WalletFolder: "Chrome Keys", WalletKey: "Chrome Safe Storage",
	}
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "ctx: Chrome sharing needs profile resource operation")
		return 2
	}
	profile, resource, operation := args[0], args[1], args[2]
	chrome := chromeConfig()
	switch resource {
	case "status":
		if operation == "probe" {
			return kit.Encode(stdout, share.AvailabilityReport{Version: share.AvailabilityVersion, Operations: chromiumengine.Probe(chrome, profile)})
		}
	case "cookie":
		return kit.RunCookie(profile, operation, input, stdout, stderr, kit.CookieBackend{
			Query: func(profile string, site *url.URL, includeExpired bool) ([]share.Cookie, string, error) {
				return chromiumengine.Query(chrome, profile, site, includeExpired)
			},
			List: func(profile string, site *url.URL, name string) ([]share.Cookie, string, error) {
				return chromiumengine.List(chrome, profile, site, name)
			},
			ReadValue: func(database string, cookie share.Cookie) (string, error) {
				return chromiumengine.ReadValue(chrome, database, cookie)
			},
			Import: func(profile string, cookie share.Cookie, replace bool) error {
				return chromiumengine.Import(chrome, profile, cookie, replace)
			},
		})
	case "policy":
		if operation == "export" {
			return kit.RunPolicyExport(input, stdout, stderr, chromePolicies())
		}
	}
	fmt.Fprintln(stderr, "ctx: unsupported Chrome share resource or operation")
	return 2
}

func chromePolicies() kit.PolicySources {
	switch runtime.GOOS {
	case "linux":
		return kit.PolicySources{Roots: []kit.PolicyRoot{{Path: "/etc/opt/chrome/policies/managed", Level: "managed"}, {Path: "/etc/opt/chrome/policies/recommended", Level: "recommended"}}}
	case "darwin":
		return kit.PolicySources{Files: kit.ManagedPreferenceFiles("com.google.Chrome")}
	case "windows":
		return kit.PolicySources{RegistryKey: `Software\Policies\Google\Chrome`}
	default:
		return kit.PolicySources{}
	}
}
