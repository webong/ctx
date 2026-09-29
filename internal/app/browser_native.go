package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// browserNativeCommand is the implementation used by the maintained browser
// adapter executables. Third-party adapters implement the same JSON protocol
// directly and never need to call this command.
func browserNativeCommand(args []string, input io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 4 {
		fmt.Fprintln(stderr, "ctx: browser native sharing needs provider profile resource operation")
		return 2
	}
	provider, profile, operation := args[0], args[1], args[3]
	var request browserShareRequest
	if err := json.NewDecoder(io.LimitReader(input, 8<<20)).Decode(&request); err != nil || request.Version != 1 {
		fmt.Fprintln(stderr, "ctx: invalid browser share request")
		return 2
	}
	if args[2] == "policy" && operation == "export" {
		bundle, err := exportNativeBrowserPolicies(provider)
		if err != nil {
			return reportError(stderr, err)
		}
		return encodeBrowserNative(stdout, bundle)
	}
	if args[2] != "cookie" {
		fmt.Fprintln(stderr, "ctx: unsupported browser share resource")
		return 2
	}
	backend, err := cookieBackendFor(provider)
	if err != nil {
		return reportError(stderr, err)
	}
	switch operation {
	case "list", "export":
		site, err := parseCookieSite(request.Site)
		if err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		name := ""
		if operation == "export" {
			name = request.Cookie.Name
			if name == "" || request.Cookie.ID <= 0 {
				fmt.Fprintln(stderr, "ctx: cookie export needs a listed cookie ID and name")
				return 2
			}
		}
		cookies, database, err := backend.list(profile, site, name)
		if err != nil {
			return reportError(stderr, err)
		}
		if operation == "list" {
			return encodeBrowserNative(stdout, cookies)
		}
		for _, cookie := range cookies {
			if sameListedCookie(cookie, request.Cookie) {
				value, err := backend.readValue(database, cookie)
				if err != nil {
					return reportError(stderr, err)
				}
				cookie.Value = value
				return encodeBrowserNative(stdout, cookie)
			}
		}
		fmt.Fprintln(stderr, "ctx: cookie changed since listing; retry")
		return 1
	case "import":
		if request.Bundle == nil {
			fmt.Fprintln(stderr, "ctx: cookie import needs a bundle")
			return 2
		}
		if err := validateBrowserCookieBundle(*request.Bundle); err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		switch provider {
		case "firefox":
			err = importFirefoxCookie(profile, request.Bundle.Cookie, request.Replace)
		case "chrome", "chromium":
			err = importChromiumCookie(provider, profile, request.Bundle.Cookie, request.Replace)
		default:
			err = fmt.Errorf("%s adapter cannot import cookies", provider)
		}
		if err != nil {
			return reportError(stderr, err)
		}
		return 0
	default:
		fmt.Fprintln(stderr, "ctx: unsupported browser cookie operation")
		return 2
	}
}

func encodeBrowserNative(output io.Writer, value any) int {
	if err := json.NewEncoder(output).Encode(value); err != nil {
		return 1
	}
	return 0
}

func sameListedCookie(a, b browserCookie) bool {
	return a.ID == b.ID && a.Ref == b.Ref && a.Name == b.Name && a.Domain == b.Domain && a.Path == b.Path &&
		a.OriginAttributes == b.OriginAttributes && a.PartitionKey == b.PartitionKey &&
		a.CrossSiteAncestor == b.CrossSiteAncestor && a.SourceScheme == b.SourceScheme && a.SourcePort == b.SourcePort
}

func validateBrowserCookieBundle(bundle browserCookieBundle) error {
	if bundle.Version != 1 {
		return errors.New("unsupported browser cookie bundle version")
	}
	site, err := parseCookieSite(bundle.Site)
	if err != nil {
		return err
	}
	cookie := bundle.Cookie
	if cookie.Name == "" || cookie.Domain == "" || !strings.HasPrefix(cookie.Path, "/") ||
		!cookieDomainMatches(strings.ToLower(site.Hostname()), cookie.Domain) ||
		(cookie.Secure && site.Scheme != "https") || !cookieActive(cookie) {
		return errors.New("cookie bundle has an invalid or expired site scope")
	}
	return nil
}
