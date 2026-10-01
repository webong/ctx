package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"

	chromiumengine "github.com/webong/ctx/adapters/chromium/engine"
	"github.com/webong/ctx/browser/extension"
	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	"github.com/webong/ctx/internal/app/browser/share"
)

func chromiumConfig() chromiumengine.Config {
	return chromiumengine.Config{
		Name: "chromium", MacUserData: "Chromium", WindowsUserData: `Chromium\User Data`, LinuxUserData: "chromium",
		KeychainService: "Chromium Safe Storage", KeychainAccount: "Chromium", SecretApplication: "chromium",
		WalletFolder: "Chromium Keys", WalletKey: "Chromium Safe Storage",
		Extensions: chromiumengine.ExtensionManagementConfig{
			Executables: extension.ExecutableLocations{
				Darwin:  []string{"Chromium.app/Contents/MacOS/Chromium"},
				Linux:   []string{"chromium", "chromium-browser"},
				Windows: []string{"Chromium/Application/chrome.exe"},
			},
			ExtensionPage: "chrome://extensions/",
		},
	}
}

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 4 && args[1] == "management" {
		return chromiumengine.RunManagement(chromiumConfig(), args[0], input, stdout, stderr)
	}
	if len(args) != 3 {
		fmt.Fprintln(stderr, "ctx: Chromium sharing needs profile resource operation")
		return 2
	}
	profile, resource, operation := args[0], args[1], args[2]
	chromium := chromiumConfig()
	switch resource {
	case "status":
		if operation == "probe" {
			return kit.Encode(stdout, share.AvailabilityReport{Version: share.AvailabilityVersion, Operations: chromiumengine.Probe(chromium, profile)})
		}
	case "cookie":
		return kit.RunCookie(profile, operation, input, stdout, stderr, kit.CookieBackend{
			QueryHandleIsStorePath: true,
			Query: func(profile string, site *url.URL, includeExpired bool) ([]share.Cookie, string, error) {
				return chromiumengine.Query(chromium, profile, site, includeExpired)
			},
			List: func(profile string, site *url.URL, name string) ([]share.Cookie, string, error) {
				return chromiumengine.List(chromium, profile, site, name)
			},
			ReadValue: func(database string, cookie share.Cookie) (string, error) {
				return chromiumengine.ReadValue(chromium, database, cookie)
			},
			Import: func(profile string, cookie share.Cookie, replace bool) error {
				return chromiumengine.Import(chromium, profile, cookie, replace)
			},
		})
	case "policy":
		if operation == "export" {
			return kit.RunPolicyExport(input, stdout, stderr, chromiumPolicies())
		}
	}
	fmt.Fprintln(stderr, "ctx: unsupported Chromium share resource or operation")
	return 2
}

func chromiumPolicies() kit.PolicySources {
	switch runtime.GOOS {
	case "linux":
		return kit.PolicySources{Roots: []kit.PolicyRoot{
			{Path: "/etc/chromium/policies/managed", Level: "managed"}, {Path: "/etc/chromium/policies/recommended", Level: "recommended"},
			{Path: "/etc/chromium-browser/policies/managed", Level: "managed"}, {Path: "/etc/chromium-browser/policies/recommended", Level: "recommended"},
		}}
	case "darwin":
		return kit.PolicySources{Files: kit.ManagedPreferenceFiles("org.chromium.Chromium")}
	case "windows":
		return kit.PolicySources{RegistryKey: `Software\Policies\Chromium`}
	default:
		return kit.PolicySources{}
	}
}
