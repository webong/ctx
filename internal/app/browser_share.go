package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	adapterpkg "github.com/webong/ctx/internal/adapter"
	"github.com/webong/ctx/internal/config"
)

type browserEndpoint struct {
	Adapter *adapterpkg.Adapter
	Profile string
}

type browserCookie struct {
	ID                int64  `json:"-"`
	Name              string `json:"name"`
	Value             string `json:"value"`
	Domain            string `json:"domain"`
	Path              string `json:"path"`
	Expiry            int64  `json:"expiry"`
	Secure            bool   `json:"secure"`
	HTTPOnly          bool   `json:"http_only"`
	SameSite          int    `json:"same_site"`
	SameSitePolicy    string `json:"same_site_policy,omitempty"`
	OriginAttributes  string `json:"origin_attributes,omitempty"`
	PartitionKey      string `json:"partition_key,omitempty"`
	CrossSiteAncestor bool   `json:"has_cross_site_ancestor,omitempty"`
	SourceScheme      int    `json:"source_scheme,omitempty"`
	SourcePort        int    `json:"source_port,omitempty"`
}

type browserCookieBackend struct {
	list        func(string, *url.URL, string) ([]browserCookie, string, error)
	readValue   func(string, browserCookie) (string, error)
	copyProfile func(string, string, browserCookie, bool) error
}

func cookieBackendFor(provider string) (browserCookieBackend, error) {
	switch provider {
	case "firefox":
		return browserCookieBackend{readFirefoxSiteCookies, readFirefoxCookieValue, copyFirefoxCookie}, nil
	case "chrome", "chromium":
		return browserCookieBackend{
			list: func(profile string, site *url.URL, name string) ([]browserCookie, string, error) {
				return readChromiumSiteCookies(provider, profile, site, name)
			},
			readValue: func(database string, cookie browserCookie) (string, error) {
				return readChromiumCookieValue(provider, database, cookie)
			},
		}, nil
	default:
		return browserCookieBackend{}, fmt.Errorf("%s cookie extraction is not implemented", provider)
	}
}

type browserCookieBundle struct {
	Version int           `json:"version"`
	Source  string        `json:"source"`
	Site    string        `json:"site"`
	Cookie  browserCookie `json:"cookie"`
}

func shareBrowserCommand(resolver *config.Resolver, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "cookie" || (args[1] != "list" && args[1] != "copy") {
		fmt.Fprintln(stderr, "ctx: share:browser requires cookie list|copy")
		return 2
	}
	action := args[1]
	flags := flag.NewFlagSet("share:browser cookie "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	from := flags.String("from", "", "source browser:profile (defaults to selected browser)")
	site := flags.String("site", "", "site URL")
	name := flags.String("name", "", "cookie name")
	domain := flags.String("domain", "", "exact cookie domain")
	path := flags.String("path", "", "exact cookie path")
	id := flags.Int64("id", 0, "cookie row ID from cookie list")
	originAttributes := flags.String("origin-attributes", "", "exact Firefox origin attributes")
	toProfile := flags.String("to-profile", "", "destination browser:profile")
	toFile := flags.String("to-file", "", "new file for a JSON cookie bundle")
	toStdout := flags.Bool("stdout", false, "write a JSON cookie bundle to stdout")
	replace := flags.Bool("replace", false, "replace an existing target-profile cookie")
	if err := flags.Parse(args[2:]); err != nil {
		return 2
	}
	selectedFlags := map[string]bool{}
	flags.Visit(func(value *flag.Flag) { selectedFlags[value.Name] = true })
	if len(flags.Args()) != 0 || *site == "" {
		fmt.Fprintln(stderr, "ctx: specify --site <http-or-https-URL> and no positional arguments")
		return 2
	}
	siteURL, err := parseCookieSite(*site)
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	source, err := resolveBrowserSource(resolver, *from)
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	backend, err := cookieBackendFor(source.Adapter.Manifest.Name)
	if err != nil {
		return reportError(stderr, err)
	}
	if selectedFlags["origin-attributes"] && source.Adapter.Manifest.Name != "firefox" {
		fmt.Fprintln(stderr, "ctx: --origin-attributes applies to Firefox; use --id to select a Chrome or Chromium partition")
		return 2
	}
	destinationCount := 0
	for _, present := range []bool{*toProfile != "", *toFile != "", *toStdout} {
		if present {
			destinationCount++
		}
	}
	if action == "list" {
		if destinationCount != 0 || selectedFlags["replace"] || selectedFlags["name"] || selectedFlags["domain"] || selectedFlags["path"] || selectedFlags["id"] || selectedFlags["origin-attributes"] {
			fmt.Fprintln(stderr, "ctx: cookie list accepts --from and --site only")
			return 2
		}
	} else if *name == "" || (selectedFlags["id"] && *id <= 0) || destinationCount != 1 || (selectedFlags["replace"] && *toProfile == "") {
		fmt.Fprintln(stderr, "ctx: cookie copy needs --name and exactly one of --to-profile, --to-file, or --stdout; --replace only applies to --to-profile")
		return 2
	}
	cookies, sourceDB, err := backend.list(source.Profile, siteURL, *name)
	if err != nil {
		return reportError(stderr, err)
	}
	if action == "list" {
		for _, cookie := range cookies {
			fmt.Fprintf(stdout, "%s\t%s\t%s\tsecure=%t\thttp_only=%t\texpiry=%d\tsame_site=%s\tid=%d\torigin_attributes=%s\tpartition_key=%s\tancestor=%t\n",
				cookie.Name, cookie.Domain, cookie.Path, cookie.Secure, cookie.HTTPOnly, cookie.Expiry, cookie.SameSitePolicy, cookie.ID, cookie.OriginAttributes, cookie.PartitionKey, cookie.CrossSiteAncestor)
		}
		return 0
	}
	selected := make([]browserCookie, 0, 1)
	for _, cookie := range cookies {
		if cookie.Name == *name && (*domain == "" || cookie.Domain == *domain) && (*path == "" || cookie.Path == *path) && (*id == 0 || cookie.ID == *id) && (!selectedFlags["origin-attributes"] || cookie.OriginAttributes == *originAttributes) {
			selected = append(selected, cookie)
		}
	}
	if len(selected) == 0 {
		fmt.Fprintln(stderr, "ctx: no matching cookie; use cookie list to inspect site cookie names and scopes")
		return 1
	}
	if len(selected) != 1 {
		fmt.Fprintln(stderr, "ctx: multiple cookies match; add --id from cookie list (or narrow by --domain, --path, or --origin-attributes)")
		return 1
	}
	cookie := selected[0]
	if *toProfile != "" {
		target, err := parseBrowserEndpoint(*toProfile)
		if err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		if target.Adapter.Manifest.Name != source.Adapter.Manifest.Name || backend.copyProfile == nil {
			fmt.Fprintf(stderr, "ctx: profile copy from %s into %s is not implemented; use --to-file or --stdout\n", source.Adapter.Manifest.Name, target.Adapter.Manifest.Name)
			return 1
		}
		if err := backend.copyProfile(sourceDB, target.Profile, cookie, *replace); err != nil {
			return reportError(stderr, err)
		}
		fmt.Fprintf(stdout, "shared cookie %s for %s into %s\n", cookie.Name, siteURL.Hostname(), *toProfile)
		return 0
	}
	if *toStdout {
		if file, ok := stdout.(*os.File); ok {
			info, err := file.Stat()
			if err != nil {
				return reportError(stderr, err)
			}
			if info.Mode()&os.ModeNamedPipe == 0 {
				fmt.Fprintln(stderr, "ctx: --stdout requires a pipe; use --to-file for a protected file")
				return 2
			}
		}
	} else if _, err := os.Lstat(*toFile); err == nil {
		fmt.Fprintln(stderr, "ctx: output file already exists; choose a new --to-file path")
		return 1
	} else if !errors.Is(err, os.ErrNotExist) {
		return reportError(stderr, err)
	}
	value, err := backend.readValue(sourceDB, cookie)
	if err != nil {
		return reportError(stderr, err)
	}
	cookie.Value = value
	bundle := browserCookieBundle{Version: 1, Source: source.Adapter.Manifest.Name + ":" + source.Profile, Site: siteURL.Scheme + "://" + siteURL.Host, Cookie: cookie}
	if *toStdout {
		if err := json.NewEncoder(stdout).Encode(bundle); err != nil {
			return reportError(stderr, err)
		}
		return 0
	}
	if err := writeCookieBundle(*toFile, bundle); err != nil {
		return reportError(stderr, err)
	}
	fmt.Fprintf(stdout, "shared cookie %s for %s into %s (mode 0600)\n", cookie.Name, siteURL.Hostname(), *toFile)
	return 0
}

func parseCookieSite(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return nil, errors.New("site must be an http or https URL with a hostname")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed, nil
}

func resolveBrowserSource(resolver *config.Resolver, choice string) (browserEndpoint, error) {
	if choice == "" {
		choice = os.Getenv("CTX_BROWSER")
		if choice == "" {
			resolved, err := resolver.Resolve("browser")
			if err != nil {
				return browserEndpoint{}, err
			}
			choice = resolved.Value
		}
	}
	if choice == "" {
		return browserEndpoint{}, errors.New("no source browser selected; use --from browser:profile or ctx set browser")
	}
	return parseBrowserEndpoint(choice)
}

func parseBrowserEndpoint(choice string) (browserEndpoint, error) {
	provider, profile, ok := strings.Cut(choice, ":")
	if !ok || provider == "" || profile == "" || strings.ContainsAny(profile, "\r\n") {
		return browserEndpoint{}, errors.New("browser endpoint must be <browser>:<profile>")
	}
	store := adapterStore()
	installed, err := store.Load(provider)
	if err != nil || !installed.IsRuntime("browser") {
		return browserEndpoint{}, fmt.Errorf("browser provider %s is not installed", provider)
	}
	if err := store.AssertTrusted(installed); err != nil {
		return browserEndpoint{}, err
	}
	return browserEndpoint{Adapter: installed, Profile: profile}, nil
}

func writeCookieBundle(path string, bundle browserCookieBundle) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(file).Encode(bundle); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func cookieActive(cookie browserCookie) bool {
	return cookie.Expiry == 0 || cookie.Expiry > time.Now().Unix()
}
