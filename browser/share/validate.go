package share

import (
	"encoding/json"
	"errors"
	"maps"
	"net/url"
	"strings"
	"time"
)

func ParseSite(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return nil, errors.New("site must be an http or https URL with a hostname")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed, nil
}

func CookieActive(cookie Cookie) bool {
	return cookie.Expiry == 0 || cookie.Expiry > time.Now().Unix()
}

func CookieDomainMatches(siteHost, cookieDomain string) bool {
	siteHost = strings.TrimSuffix(strings.ToLower(siteHost), ".")
	cookieDomain = strings.ToLower(cookieDomain)
	if strings.HasPrefix(cookieDomain, ".") {
		base := strings.TrimPrefix(cookieDomain, ".")
		return siteHost == base || strings.HasSuffix(siteHost, "."+base)
	}
	return siteHost == cookieDomain
}

func SameListedCookie(a, b Cookie) bool {
	return a.ID == b.ID && a.Ref == b.Ref && a.Name == b.Name && a.Domain == b.Domain && a.Path == b.Path &&
		a.PartitionKey == b.PartitionKey && a.CrossSiteAncestor == b.CrossSiteAncestor && maps.Equal(a.Attributes, b.Attributes)
}

func ValidateCookieBundle(bundle CookieBundle) error {
	if bundle.Version != Version {
		return errors.New("unsupported browser cookie bundle version")
	}
	site, err := ParseSite(bundle.Site)
	if err != nil {
		return err
	}
	cookie := bundle.Cookie
	if cookie.Name == "" || cookie.Domain == "" || !strings.HasPrefix(cookie.Path, "/") ||
		!CookieDomainMatches(site.Hostname(), cookie.Domain) ||
		(cookie.Secure && site.Scheme != "https") || !CookieActive(cookie) {
		return errors.New("cookie bundle has an invalid or expired site scope")
	}
	if (strings.HasPrefix(cookie.Name, "__Secure-") && !cookie.Secure) ||
		(strings.HasPrefix(cookie.Name, "__Host-") && (!cookie.Secure || strings.HasPrefix(cookie.Domain, ".") || cookie.Path != "/")) ||
		(cookie.SameSitePolicy == "none" && !cookie.Secure) {
		return errors.New("cookie bundle violates secure prefix or SameSite requirements")
	}
	if len(cookie.Attributes) > 32 {
		return errors.New("cookie bundle has too many adapter attributes")
	}
	for key, value := range cookie.Attributes {
		if !strings.Contains(key, ".") || len(key) > 128 || len(value) > 1024 || strings.ContainsAny(key, " \t\r\n") {
			return errors.New("cookie bundle has an invalid adapter attribute")
		}
	}
	return nil
}

func ValidateResourceBundle(bundle ResourceBundle, resource string) error {
	if bundle.Version != Version || bundle.Resource != resource || len(bundle.Payload) == 0 || !json.Valid(bundle.Payload) {
		return errors.New("resource bundle has an invalid version, type, or payload")
	}
	return nil
}
