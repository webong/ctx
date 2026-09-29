package chromium

import (
	"net/url"

	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	"github.com/webong/ctx/internal/app/browser/share"
)

// Config contains browser identity and storage conventions supplied by its adapter.
type Config struct {
	Name              string
	MacUserData       string
	WindowsUserData   string
	LinuxUserData     string
	KeychainService   string
	KeychainAccount   string
	SecretApplication string
	WalletFolder      string
	WalletKey         string
}

// Cookie is the portable cookie type returned by the Chromium engine. It is
// re-exported so adapters outside this module can use the engine without
// importing CTX's internal browser packages.
type Cookie = share.Cookie

type browserCookie = Cookie

var (
	cookieDatabaseColumns  = kit.CookieDatabaseColumns
	readableCookieDatabase = kit.ReadableCookieDatabase
	hasSQLiteColumn        = kit.HasSQLiteColumn
	runSQLite              = kit.RunSQLite
	sqlString              = kit.SQLString
	sqlIdentifier          = kit.SQLIdentifier
	sqlBool                = kit.SQLBool
	cookieHostSQL          = kit.CookieHostSQL
)

func cookieActive(cookie browserCookie) bool { return share.CookieActive(cookie) }
func cookieDomainMatches(siteHost, cookieDomain string) bool {
	return share.CookieDomainMatches(siteHost, cookieDomain)
}

func List(config Config, profile string, site *url.URL, name string) ([]Cookie, string, error) {
	return readChromiumSiteCookies(config, profile, site, name)
}
func ReadValue(config Config, database string, cookie Cookie) (string, error) {
	return readChromiumCookieValue(config, database, cookie)
}
func Import(config Config, profile string, cookie Cookie, replace bool) error {
	return importChromiumCookie(config, profile, cookie, replace)
}
