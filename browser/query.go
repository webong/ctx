// Package browser reads site cookies through installed, trusted ctx browser
// adapters. Storage formats and operating-system credentials remain owned by
// the adapters; this package only selects sources and combines their results.
package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/webong/ctx/adapter/browser"
	"github.com/webong/ctx/internal/mod"
	"github.com/webong/ctx/internal/platform"
)

// Mode controls how results from ordered sources are combined.
type Mode string

const (
	ModeMerge Mode = "merge"
	ModeFirst Mode = "first"
)

// InlineCookies supplies a local cookie array or {"cookies": [...]} object.
// Exactly one field may be set. It is tried before installed adapters.
type InlineCookies struct {
	JSON   []byte
	Base64 string
	File   string
}

// Options selects sites, adapters, and profiles. Sources contains explicit
// browser:profile endpoints in priority order. Browsers may contain adapter
// names and uses Profiles to select one profile per adapter; otherwise all
// discoverable profiles are used. With neither field, all trusted installed
// browser adapters are discovered. A URL or Origins entry is required unless
// AllowAllHosts is explicitly set.
type Options struct {
	URL            string
	Origins        []string
	Names          []string
	Sources        []string
	Browsers       []string
	Profiles       map[string]string
	Mode           Mode
	Inline         InlineCookies
	InlineOnly     bool
	IncludeExpired bool
	AllowAllHosts  bool
	Timeout        time.Duration
	AdapterHome    string
}

// Cookie includes its portable browser fields and the endpoint that supplied it.
type Cookie struct {
	browser.Cookie
	Source string `json:"source"`
}

// Result retains partial success when an unavailable source reports a warning.
type Result struct {
	Cookies  []Cookie `json:"cookies"`
	Warnings []string `json:"warnings,omitempty"`
}

// Get retrieves cookies from installed adapters without importing browser
// implementations into the caller. Each adapter is checked against ctx trust.
func Get(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		return Result{}, errors.New("browser query needs a context")
	}
	if options.Mode != "" && options.Mode != ModeMerge && options.Mode != ModeFirst {
		return Result{}, fmt.Errorf("unknown browser query mode %q", options.Mode)
	}
	if len(options.Sources) > 0 && len(options.Browsers) > 0 {
		return Result{}, errors.New("choose Sources or Browsers")
	}
	if options.InlineOnly && (len(options.Sources) > 0 || len(options.Browsers) > 0) {
		return Result{}, errors.New("InlineOnly cannot be combined with browser sources")
	}
	sites, err := querySites(options)
	if err != nil {
		return Result{}, err
	}
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}
	result := Result{}
	seen := map[string]bool{}
	inline, err := parseInline(options.Inline)
	if err != nil {
		return Result{}, err
	}
	for _, cookie := range inline {
		if matchesQuery(cookie, sites, options) {
			appendCookie(&result, seen, Cookie{Cookie: cookie, Source: "inline"})
		}
	}
	if options.Mode == ModeFirst && len(result.Cookies) > 0 {
		return result, nil
	}
	if options.InlineOnly {
		return result, nil
	}
	store := mod.NewStore(adapterHome(options.AdapterHome))
	sources, warnings, err := selectSources(ctx, store, options)
	if err != nil {
		return Result{}, err
	}
	result.Warnings = append(result.Warnings, warnings...)
	if len(sites) == 0 {
		sites = []*url.URL{nil}
	}
	for _, source := range sources {
		before := len(result.Cookies)
		for _, site := range sites {
			cookies, warnings, err := sourceCookies(ctx, source, site, options)
			for _, cookie := range cookies {
				if matchesQuery(cookie, sites, options) {
					appendCookie(&result, seen, Cookie{Cookie: cookie, Source: source.label})
				}
			}
			for _, warning := range warnings {
				result.Warnings = append(result.Warnings, source.label+": "+warning)
			}
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", source.label, err))
				continue
			}
		}
		if options.Mode == ModeFirst && len(result.Cookies) > before {
			break
		}
	}
	return result, nil
}

func adapterHome(override string) string {
	if override != "" {
		return override
	}
	if value := os.Getenv("CTX_ADAPTER_HOME"); value != "" {
		return value
	}
	home := os.Getenv("CTX_HOME")
	if home == "" {
		home = platform.DefaultConfigHome()
	}
	return filepath.Join(home, "adapters")
}

func querySites(options Options) ([]*url.URL, error) {
	var raw []string
	if options.URL != "" {
		raw = append(raw, options.URL)
	}
	raw = append(raw, options.Origins...)
	if len(raw) == 0 && !options.AllowAllHosts {
		return nil, errors.New("browser query needs URL or Origins")
	}
	sites := make([]*url.URL, 0, len(raw))
	for _, value := range raw {
		site, err := browser.ParseSite(value)
		if err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, nil
}

func parseInline(input InlineCookies) ([]browser.Cookie, error) {
	count := 0
	for _, set := range []bool{input.JSON != nil, input.Base64 != "", input.File != ""} {
		if set {
			count++
		}
	}
	if count > 1 {
		return nil, errors.New("inline cookies accept exactly one of JSON, Base64, or File")
	}
	data := input.JSON
	if input.Base64 != "" {
		if len(input.Base64) > base64.StdEncoding.EncodedLen(8<<20) {
			return nil, errors.New("inline cookies exceed 8 MiB")
		}
		var err error
		data, err = base64.StdEncoding.DecodeString(input.Base64)
		if err != nil {
			return nil, fmt.Errorf("decode inline cookies: %w", err)
		}
	}
	if input.File != "" {
		file, err := os.Open(input.File)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return nil, errors.New("inline cookie file must be a regular file under 8 MiB")
		}
		data, err = io.ReadAll(io.LimitReader(file, (8<<20)+1))
		if err != nil {
			return nil, err
		}
	}
	if data == nil {
		return nil, nil
	}
	if len(data) > 8<<20 {
		return nil, errors.New("inline cookies exceed 8 MiB")
	}
	var cookies []browser.Cookie
	if err := json.Unmarshal(data, &cookies); err == nil {
		return cookies, nil
	}
	var object struct {
		Cookies []browser.Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(data, &object); err != nil || object.Cookies == nil {
		return nil, errors.New("inline cookies must be a JSON array or an object with cookies")
	}
	return object.Cookies, nil
}

func matchesQuery(cookie browser.Cookie, sites []*url.URL, options Options) bool {
	if cookie.Name == "" || cookie.Domain == "" || !strings.HasPrefix(cookie.Path, "/") {
		return false
	}
	if !options.IncludeExpired && !browser.CookieActive(cookie) {
		return false
	}
	if len(options.Names) > 0 {
		found := false
		for _, name := range options.Names {
			if cookie.Name == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(sites) == 0 {
		return options.AllowAllHosts
	}
	for _, site := range sites {
		if site == nil {
			return options.AllowAllHosts
		}
		if browser.CookieDomainMatches(site.Hostname(), cookie.Domain) &&
			(site.Path == "" || browser.CookiePathMatches(site.EscapedPath(), cookie.Path)) &&
			(!cookie.Secure || site.Scheme == "https") {
			return true
		}
	}
	return false
}

func appendCookie(result *Result, seen map[string]bool, cookie Cookie) {
	container := cookie.Attributes["firefox.origin_attributes"]
	key := strings.Join([]string{cookie.Name, cookie.Domain, cookie.Path, cookie.PartitionKey, fmt.Sprint(cookie.CrossSiteAncestor), container}, "\x00")
	if seen[key] {
		return
	}
	seen[key] = true
	result.Cookies = append(result.Cookies, cookie)
}

type selectedSource struct {
	adapter *mod.Adapter
	profile string
	label   string
}

func selectSources(ctx context.Context, store *mod.Store, options Options) ([]selectedSource, []string, error) {
	if len(options.Sources) > 0 {
		sources := make([]selectedSource, 0, len(options.Sources))
		for _, endpoint := range options.Sources {
			name, profile, ok := strings.Cut(endpoint, ":")
			if !ok || name == "" || profile == "" || strings.ContainsAny(profile, "\r\n") {
				return nil, nil, fmt.Errorf("invalid browser endpoint %q", endpoint)
			}
			adapter, err := store.Load(name)
			if err != nil {
				return nil, nil, err
			}
			if err := checkSource(store, adapter); err != nil {
				return nil, nil, err
			}
			sources = append(sources, selectedSource{adapter, profile, endpoint})
		}
		return sources, nil, nil
	}
	var adapters []*mod.Adapter
	if len(options.Browsers) > 0 {
		for _, name := range options.Browsers {
			adapter, err := store.Load(name)
			if err != nil {
				return nil, nil, err
			}
			if err := checkSource(store, adapter); err != nil {
				return nil, nil, err
			}
			adapters = append(adapters, adapter)
		}
	} else {
		installed, err := store.List()
		if err != nil {
			return nil, nil, err
		}
		for _, adapter := range installed {
			if supportsCookieQuery(adapter) {
				if trusted, _ := store.IsTrusted(adapter); trusted {
					adapters = append(adapters, adapter)
				}
			}
		}
		sort.Slice(adapters, func(i, j int) bool { return adapters[i].Manifest.Name < adapters[j].Manifest.Name })
	}
	var sources []selectedSource
	var warnings []string
	for _, adapter := range adapters {
		name := adapter.Manifest.Name
		if profile := options.Profiles[name]; profile != "" {
			if strings.ContainsAny(profile, "\r\n") {
				return nil, nil, fmt.Errorf("invalid profile for %s", name)
			}
			sources = append(sources, selectedSource{adapter, profile, name + ":" + profile})
			continue
		}
		if !adapter.HasCapability("list") {
			warnings = append(warnings, name+": profile discovery unavailable")
			continue
		}
		output, err := invoke(ctx, adapter, "list", "", nil, nil)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			profile, ok := strings.CutPrefix(strings.TrimSuffix(line, "\r"), name+":")
			if ok && profile != "" && !strings.ContainsAny(profile, "\r\n") {
				sources = append(sources, selectedSource{adapter, profile, name + ":" + profile})
			}
		}
	}
	return sources, warnings, nil
}

func checkSource(store *mod.Store, adapter *mod.Adapter) error {
	if !supportsCookieQuery(adapter) {
		return fmt.Errorf("adapter %s does not support cookie queries", adapter.Manifest.Name)
	}
	return store.AssertTrusted(adapter)
}

func supportsCookieQuery(adapter *mod.Adapter) bool {
	return adapter.IsRuntime("browser") && (adapter.HasBrowserShare("cookie.query") ||
		(adapter.HasBrowserShare("cookie.list") && adapter.HasBrowserShare("cookie.export")))
}

func sourceCookies(ctx context.Context, source selectedSource, site *url.URL, options Options) ([]browser.Cookie, []string, error) {
	if source.adapter.HasBrowserShare("cookie.query") {
		request := browser.CookieRequest{Version: browser.Version, Names: options.Names,
			IncludeExpired: options.IncludeExpired, AllowAllHosts: site == nil}
		if site != nil {
			request.Site = site.String()
		}
		payload, _ := json.Marshal(request)
		output, err := invoke(ctx, source.adapter, "share", source.profile, []string{"cookie", "query"}, payload)
		if err != nil {
			return nil, nil, err
		}
		var result browser.CookieQueryResult
		if err := json.Unmarshal(output, &result); err != nil {
			return nil, nil, fmt.Errorf("invalid cookie query: %w", err)
		}
		return result.Cookies, result.Warnings, nil
	}
	if site == nil || options.IncludeExpired {
		return nil, nil, errors.New("adapter does not support all-host or expired-cookie queries")
	}
	request, _ := json.Marshal(browser.CookieRequest{Version: browser.Version, Site: site.String()})
	output, err := invoke(ctx, source.adapter, "share", source.profile, []string{"cookie", "list"}, request)
	if err != nil {
		return nil, nil, err
	}
	var listed []browser.Cookie
	if err := json.Unmarshal(output, &listed); err != nil {
		return nil, nil, fmt.Errorf("invalid cookie list: %w", err)
	}
	var cookies []browser.Cookie
	for _, cookie := range listed {
		if len(options.Names) > 0 {
			found := false
			for _, name := range options.Names {
				if name == cookie.Name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if !browser.CookieMatchesSite(site, cookie) {
			continue
		}
		request, _ := json.Marshal(browser.CookieRequest{Version: browser.Version, Site: site.String(), Cookie: cookie})
		output, err := invoke(ctx, source.adapter, "share", source.profile, []string{"cookie", "export"}, request)
		if err != nil {
			return cookies, nil, fmt.Errorf("export %s: %w", cookie.Name, err)
		}
		var exported browser.Cookie
		if err := json.Unmarshal(output, &exported); err != nil || !browser.SameListedCookie(cookie, exported) {
			return cookies, nil, fmt.Errorf("export %s returned a different cookie", cookie.Name)
		}
		cookies = append(cookies, exported)
	}
	return cookies, nil, nil
}

func invoke(ctx context.Context, adapter *mod.Adapter, operation, profile string, args []string, input []byte) ([]byte, error) {
	command, err := adapter.CommandContext(ctx, mod.Invocation{Operation: operation, Selection: profile, Arguments: args})
	if err != nil {
		return nil, err
	}
	command.Stdin = bytes.NewReader(input)
	stdout := &boundedBuffer{limit: 8 << 20}
	stderr := &boundedBuffer{limit: 4 << 10}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return nil, err
		}
		return nil, errors.New(message)
	}
	return stdout.Bytes(), nil
}

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	if buffer.Len()+len(data) > buffer.limit {
		return 0, errors.New("browser adapter output exceeds limit")
	}
	return buffer.Buffer.Write(data)
}
