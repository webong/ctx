package main

import (
	"io"

	native "github.com/webong/ctx/adapters/browsercommon"
	"github.com/webong/ctx/browser/share"
)

type browserCookie = share.Cookie
type browserResourceRequest = share.ResourceRequest
type browserResourceBundle = share.ResourceBundle

var (
	cookieDatabaseColumns  = native.CookieDatabaseColumns
	readableCookieDatabase = native.ReadableCookieDatabase
	snapshotCookieDatabase = native.SnapshotCookieDatabase
	copyPrivateFile        = native.CopyPrivateFile
	hasSQLiteColumn        = native.HasSQLiteColumn
	runSQLite              = native.RunSQLite
	sqlString              = native.SQLString
	sqlIdentifier          = native.SQLIdentifier
	sqlBool                = native.SQLBool
	cookieHostSQL          = native.CookieHostSQL
)

func cookieActive(cookie browserCookie) bool { return share.CookieActive(cookie) }
func validateBrowserResourceBundle(bundle browserResourceBundle, resource string) error {
	return share.ValidateResourceBundle(bundle, resource)
}
func encodeBrowserNative(output io.Writer, value any) int { return native.Encode(output, value) }
func reportError(stderr io.Writer, err error) int         { return native.ReportError(stderr, err) }
func reportErrorCode(stderr io.Writer, err error, code int) int {
	return native.ReportErrorCode(stderr, err, code)
}
