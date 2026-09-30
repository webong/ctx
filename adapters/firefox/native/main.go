package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	browsershare "github.com/webong/ctx/internal/app/browser/share"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "ctx: Firefox sharing needs profile resource operation")
		return 2
	}
	profile, resource, operation := args[0], args[1], args[2]
	switch resource {
	case "status":
		if operation == "probe" {
			return kit.Encode(stdout, firefoxShareStatus(profile))
		}
	case "cookie":
		return kit.RunCookie(profile, operation, input, stdout, stderr, kit.CookieBackend{
			List: readFirefoxSiteCookies, ReadValue: readFirefoxCookieValue, Import: importFirefoxCookie,
		})
	case "certificate":
		return nativeFirefoxCertificateCommand(profile, operation, input, stdout, stderr)
	case "policy":
		if operation != "export" {
			break
		}
		return kit.RunPolicyExport(input, stdout, stderr, firefoxPolicySources())
	}
	fmt.Fprintln(stderr, "ctx: unsupported Firefox share resource or operation")
	return 2
}

func firefoxShareStatus(profile string) browsershare.AvailabilityReport {
	operations := map[string]string{"policy.export": "ready"}
	storeReady := false
	if _, err := firefoxCookieDatabase(profile); err == nil {
		_, err = exec.LookPath("sqlite3")
		storeReady = err == nil
	}
	for _, name := range []string{"cookie.list", "cookie.export", "cookie.import"} {
		operations[name] = "blocked"
		if storeReady {
			operations[name] = "ready"
			if name == "cookie.import" {
				operations[name] = "unknown" // target profile state is checked at import time
			}
		}
	}
	certificates := "blocked"
	if _, err := exec.LookPath("certutil"); err == nil {
		if _, err := firefoxCookieDatabase(profile); err == nil {
			certificates = "unknown"
		}
	}
	operations["certificate.list"] = certificates
	operations["certificate.export"] = certificates
	operations["certificate.import"] = certificates
	if _, err := exec.LookPath("pk12util"); err != nil {
		operations["certificate.export"] = "blocked"
		operations["certificate.import"] = "blocked"
	}
	return browsershare.AvailabilityReport{Version: browsershare.AvailabilityVersion, Operations: operations}
}

func firefoxPolicySources() kit.PolicySources {
	switch runtime.GOOS {
	case "linux":
		return kit.PolicySources{Files: []kit.PolicyFile{
			{Path: "/etc/firefox/policies/policies.json", Level: "managed", Format: "json"},
			{Path: "/usr/lib/firefox/distribution/policies.json", Level: "managed", Format: "json"},
			{Path: "/usr/lib64/firefox/distribution/policies.json", Level: "managed", Format: "json"},
		}}
	case "darwin":
		sources := kit.PolicySources{Files: kit.ManagedPreferenceFiles("org.mozilla.firefox")}
		for _, path := range []string{"/Applications/Firefox.app/Contents/Resources/distribution/policies.json", filepath.Join(os.Getenv("HOME"), "Applications/Firefox.app/Contents/Resources/distribution/policies.json")} {
			sources.Files = append(sources.Files, kit.PolicyFile{Path: path, Level: "managed", Format: "json"})
		}
		return sources
	case "windows":
		return kit.PolicySources{RegistryKey: `Software\Policies\Mozilla\Firefox`}
	default:
		return kit.PolicySources{}
	}
}
