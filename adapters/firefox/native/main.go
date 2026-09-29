package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	native "github.com/webong/ctx/adapters/browsercommon"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "ctx: Firefox sharing needs profile resource operation")
		return 2
	}
	profile, resource, operation := args[0], args[1], args[2]
	switch resource {
	case "cookie":
		return native.RunCookie(profile, operation, input, stdout, stderr, native.CookieBackend{
			List: readFirefoxSiteCookies, ReadValue: readFirefoxCookieValue, Import: importFirefoxCookie,
		})
	case "certificate":
		return nativeFirefoxCertificateCommand(profile, operation, input, stdout, stderr)
	case "policy":
		if operation != "export" {
			break
		}
		return native.RunPolicyExport(input, stdout, stderr, firefoxPolicySources())
	}
	fmt.Fprintln(stderr, "ctx: unsupported Firefox share resource or operation")
	return 2
}

func firefoxPolicySources() native.PolicySources {
	switch runtime.GOOS {
	case "linux":
		return native.PolicySources{Files: []native.PolicyFile{
			{Path: "/etc/firefox/policies/policies.json", Level: "managed", Format: "json"},
			{Path: "/usr/lib/firefox/distribution/policies.json", Level: "managed", Format: "json"},
			{Path: "/usr/lib64/firefox/distribution/policies.json", Level: "managed", Format: "json"},
		}}
	case "darwin":
		sources := native.PolicySources{Files: native.ManagedPreferenceFiles("org.mozilla.firefox")}
		for _, path := range []string{"/Applications/Firefox.app/Contents/Resources/distribution/policies.json", filepath.Join(os.Getenv("HOME"), "Applications/Firefox.app/Contents/Resources/distribution/policies.json")} {
			sources.Files = append(sources.Files, native.PolicyFile{Path: path, Level: "managed", Format: "json"})
		}
		return sources
	case "windows":
		return native.PolicySources{RegistryKey: `Software\Policies\Mozilla\Firefox`}
	default:
		return native.PolicySources{}
	}
}
