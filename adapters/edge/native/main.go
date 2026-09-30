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

func edgeConfig() chromiumengine.Config {
	return chromiumengine.Config{
		Name: "edge", MacUserData: "Microsoft Edge", WindowsUserData: `Microsoft\Edge\User Data`, LinuxUserData: "microsoft-edge",
		KeychainService: "Microsoft Edge Safe Storage", KeychainAccount: "Microsoft Edge", SecretApplication: "microsoft-edge",
		WalletFolder: "Microsoft Edge Keys", WalletKey: "Microsoft Edge Safe Storage",
	}
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "ctx: Edge sharing needs profile resource operation")
		return 2
	}
	profile, resource, operation := args[0], args[1], args[2]
	edge := edgeConfig()
	switch resource {
	case "status":
		if operation == "probe" {
			return kit.Encode(stdout, share.AvailabilityReport{Version: share.AvailabilityVersion, Operations: chromiumengine.Probe(edge, profile)})
		}
	case "cookie":
		return kit.RunCookie(profile, operation, input, stdout, stderr, kit.CookieBackend{
			List: func(profile string, site *url.URL, name string) ([]share.Cookie, string, error) {
				return chromiumengine.List(edge, profile, site, name)
			},
			ReadValue: func(database string, cookie share.Cookie) (string, error) {
				return chromiumengine.ReadValue(edge, database, cookie)
			},
			Import: func(profile string, cookie share.Cookie, replace bool) error {
				return chromiumengine.Import(edge, profile, cookie, replace)
			},
		})
	case "policy":
		if operation == "export" {
			return kit.RunPolicyExport(input, stdout, stderr, edgePolicies())
		}
	}
	fmt.Fprintln(stderr, "ctx: unsupported Edge share resource or operation")
	return 2
}

func edgePolicies() kit.PolicySources {
	switch runtime.GOOS {
	case "linux":
		return kit.PolicySources{Roots: []kit.PolicyRoot{{Path: "/etc/opt/edge/policies/managed", Level: "managed"}, {Path: "/etc/opt/edge/policies/recommended", Level: "recommended"}}}
	case "darwin":
		return kit.PolicySources{Files: kit.ManagedPreferenceFiles("com.microsoft.Edge")}
	case "windows":
		return kit.PolicySources{RegistryKey: `Software\Policies\Microsoft\Edge`}
	default:
		return kit.PolicySources{}
	}
}
