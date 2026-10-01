package chromium

import (
	"fmt"
	"io"
	"net/url"

	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	"github.com/webong/ctx/internal/app/browser/share"
)

// Run serves the versioned cookie protocol for a Chromium-family adapter.
// Each adapter supplies its own storage paths and credential identity.
func Run(config Config, args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 4 && args[1] == "management" {
		return RunManagement(config, args[0], input, stdout, stderr)
	}
	if len(args) != 3 {
		fmt.Fprintln(stderr, "ctx: browser sharing needs profile resource operation")
		return 2
	}
	profile, resource, operation := args[0], args[1], args[2]
	switch resource {
	case "status":
		if operation == "probe" {
			operations := Probe(config, profile)
			delete(operations, "policy.export")
			return kit.Encode(stdout, share.AvailabilityReport{Version: share.AvailabilityVersion, Operations: operations})
		}
	case "cookie":
		return kit.RunCookie(profile, operation, input, stdout, stderr, kit.CookieBackend{
			QueryHandleIsStorePath: true,
			Query: func(profile string, site *url.URL, includeExpired bool) ([]share.Cookie, string, error) {
				return Query(config, profile, site, includeExpired)
			},
			List: func(profile string, site *url.URL, name string) ([]share.Cookie, string, error) {
				return List(config, profile, site, name)
			},
			ReadValue: func(database string, cookie share.Cookie) (string, error) {
				return ReadValue(config, database, cookie)
			},
			Import: func(profile string, cookie share.Cookie, replace bool) error {
				return Import(config, profile, cookie, replace)
			},
		})
	}
	fmt.Fprintln(stderr, "ctx: unsupported browser share resource or operation")
	return 2
}
