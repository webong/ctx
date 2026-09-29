package browsercommon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/webong/ctx/browser/share"
)

// CookieBackend is supplied by one browser adapter. The command never chooses
// a browser implementation or receives a provider name.
type CookieBackend struct {
	List      func(profile string, site *url.URL, name string) ([]share.Cookie, string, error)
	ReadValue func(handle string, cookie share.Cookie) (string, error)
	Import    func(profile string, cookie share.Cookie, replace bool) error
}

func RunCookie(profile, operation string, input io.Reader, stdout, stderr io.Writer, backend CookieBackend) int {
	var request share.CookieRequest
	if err := json.NewDecoder(io.LimitReader(input, 8<<20)).Decode(&request); err != nil || request.Version != share.Version {
		fmt.Fprintln(stderr, "ctx: invalid browser share request")
		return 2
	}
	switch operation {
	case "list", "export":
		if backend.List == nil || (operation == "export" && backend.ReadValue == nil) {
			fmt.Fprintln(stderr, "ctx: cookie list or export is unavailable")
			return 2
		}
		site, err := share.ParseSite(request.Site)
		if err != nil {
			return ReportErrorCode(stderr, err, 2)
		}
		name := ""
		if operation == "export" {
			name = request.Cookie.Name
			if name == "" || (request.Cookie.ID <= 0 && request.Cookie.Ref == "") {
				fmt.Fprintln(stderr, "ctx: cookie export needs a listed cookie ID or reference and name")
				return 2
			}
		}
		cookies, handle, err := backend.List(profile, site, name)
		if err != nil {
			return ReportError(stderr, err)
		}
		if operation == "list" {
			return Encode(stdout, cookies)
		}
		for _, cookie := range cookies {
			if share.SameListedCookie(cookie, request.Cookie) {
				value, err := backend.ReadValue(handle, cookie)
				if err != nil {
					return ReportError(stderr, err)
				}
				cookie.Value = value
				return Encode(stdout, cookie)
			}
		}
		fmt.Fprintln(stderr, "ctx: cookie changed since listing; retry")
		return 1
	case "import":
		if backend.Import == nil {
			fmt.Fprintln(stderr, "ctx: cookie import is unavailable")
			return 2
		}
		if request.Bundle == nil {
			fmt.Fprintln(stderr, "ctx: cookie import needs a bundle")
			return 2
		}
		if err := share.ValidateCookieBundle(*request.Bundle); err != nil {
			return ReportErrorCode(stderr, err, 2)
		}
		if err := backend.Import(profile, request.Bundle.Cookie, request.Replace); err != nil {
			return ReportError(stderr, err)
		}
		return 0
	default:
		fmt.Fprintln(stderr, "ctx: unsupported browser cookie operation")
		return 2
	}
}

func RunPolicyExport(input io.Reader, stdout, stderr io.Writer, sources PolicySources) int {
	var request share.PolicyRequest
	if err := json.NewDecoder(io.LimitReader(input, 8<<20)).Decode(&request); err != nil || request.Version != share.Version {
		fmt.Fprintln(stderr, "ctx: invalid browser share request")
		return 2
	}
	bundle, err := ExportPolicies(sources)
	if err != nil {
		return ReportError(stderr, err)
	}
	return Encode(stdout, bundle)
}

func Encode(output io.Writer, value any) int {
	if err := json.NewEncoder(output).Encode(value); err != nil {
		return 1
	}
	return 0
}

func ReportError(stderr io.Writer, err error) int { return ReportErrorCode(stderr, err, 1) }
func ReportErrorCode(stderr io.Writer, err error, code int) int {
	fmt.Fprintf(stderr, "browser adapter: %v\n", err)
	return code
}
