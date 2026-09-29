package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"

	native "github.com/webong/ctx/adapters/browsercommon"
	"github.com/webong/ctx/adapters/chromiumengine"
	"github.com/webong/ctx/browser/share"
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
	case "cookie":
		return native.RunCookie(profile, operation, input, stdout, stderr, native.CookieBackend{
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
			return native.RunPolicyExport(input, stdout, stderr, chromePolicies())
		}
	}
	fmt.Fprintln(stderr, "ctx: unsupported Chrome share resource or operation")
	return 2
}

func chromePolicies() native.PolicySources {
	switch runtime.GOOS {
	case "linux":
		return native.PolicySources{Roots: []native.PolicyRoot{{Path: "/etc/opt/chrome/policies/managed", Level: "managed"}, {Path: "/etc/opt/chrome/policies/recommended", Level: "recommended"}}}
	case "darwin":
		return native.PolicySources{Files: native.ManagedPreferenceFiles("com.google.Chrome")}
	case "windows":
		return native.PolicySources{RegistryKey: `Software\Policies\Google\Chrome`}
	default:
		return native.PolicySources{}
	}
}
