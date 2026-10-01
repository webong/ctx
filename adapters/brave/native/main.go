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

func braveConfig() chromiumengine.Config {
	return chromiumengine.Config{
		Name: "brave", MacUserData: "BraveSoftware/Brave-Browser", WindowsUserData: `BraveSoftware\Brave-Browser\User Data`, LinuxUserData: "BraveSoftware/Brave-Browser",
		KeychainService: "Brave Safe Storage", KeychainAccount: "Brave", SecretApplication: "brave",
		WalletFolder: "Brave Keys", WalletKey: "Brave Safe Storage",
	}
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "ctx: Brave sharing needs profile resource operation")
		return 2
	}
	profile, resource, operation := args[0], args[1], args[2]
	brave := braveConfig()
	switch resource {
	case "status":
		if operation == "probe" {
			return kit.Encode(stdout, share.AvailabilityReport{Version: share.AvailabilityVersion, Operations: chromiumengine.Probe(brave, profile)})
		}
	case "cookie":
		return kit.RunCookie(profile, operation, input, stdout, stderr, kit.CookieBackend{
			QueryHandleIsStorePath: true,
			Query: func(profile string, site *url.URL, includeExpired bool) ([]share.Cookie, string, error) {
				return chromiumengine.Query(brave, profile, site, includeExpired)
			},
			List: func(profile string, site *url.URL, name string) ([]share.Cookie, string, error) {
				return chromiumengine.List(brave, profile, site, name)
			},
			ReadValue: func(database string, cookie share.Cookie) (string, error) {
				return chromiumengine.ReadValue(brave, database, cookie)
			},
			Import: func(profile string, cookie share.Cookie, replace bool) error {
				return chromiumengine.Import(brave, profile, cookie, replace)
			},
		})
	case "policy":
		if operation == "export" {
			return kit.RunPolicyExport(input, stdout, stderr, bravePolicies())
		}
	}
	fmt.Fprintln(stderr, "ctx: unsupported Brave share resource or operation")
	return 2
}

func bravePolicies() kit.PolicySources {
	switch runtime.GOOS {
	case "linux":
		return kit.PolicySources{Roots: []kit.PolicyRoot{{Path: "/etc/brave/policies/managed", Level: "managed"}, {Path: "/etc/brave/policies/recommended", Level: "recommended"}}}
	case "darwin":
		return kit.PolicySources{Files: kit.ManagedPreferenceFiles("com.brave.Browser")}
	case "windows":
		return kit.PolicySources{RegistryKey: `Software\Policies\BraveSoftware\Brave`}
	default:
		return kit.PolicySources{}
	}
}
