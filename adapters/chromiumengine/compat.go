package chromiumengine

import (
	"net/url"

	native "github.com/webong/ctx/adapters/browsercommon"
	"github.com/webong/ctx/browser/share"
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

type browserCookie = share.Cookie

var (
	cookieDatabaseColumns  = native.CookieDatabaseColumns
	readableCookieDatabase = native.ReadableCookieDatabase
	hasSQLiteColumn        = native.HasSQLiteColumn
	runSQLite              = native.RunSQLite
	sqlString              = native.SQLString
	sqlIdentifier          = native.SQLIdentifier
	sqlBool                = native.SQLBool
	cookieHostSQL          = native.CookieHostSQL
)

func cookieActive(cookie browserCookie) bool { return share.CookieActive(cookie) }
func cookieDomainMatches(siteHost, cookieDomain string) bool {
	return share.CookieDomainMatches(siteHost, cookieDomain)
}

func List(config Config, profile string, site *url.URL, name string) ([]share.Cookie, string, error) {
	return readChromiumSiteCookies(config, profile, site, name)
}
func ReadValue(config Config, database string, cookie share.Cookie) (string, error) {
	return readChromiumCookieValue(config, database, cookie)
}
func Import(config Config, profile string, cookie share.Cookie, replace bool) error {
	return importChromiumCookie(config, profile, cookie, replace)
}
