// Package browser exposes shared browser resource and profile-management
// protocols to external Go adapters.
// Browser implementations remain separate executables that ctx invokes through
// the adapter API; this package only provides shared request and response code.
package browser

import (
	"context"
	"encoding/json"
	"io"
	"net/url"

	kit "github.com/webong/ctx/internal/app/browser/adapterkit"
	"github.com/webong/ctx/internal/app/browser/management"
	"github.com/webong/ctx/internal/app/browser/share"
)

const Version = share.Version
const AvailabilityVersion = share.AvailabilityVersion

type AvailabilityReport = share.AvailabilityReport
type Cookie = share.Cookie
type CookieBundle = share.CookieBundle
type CookieRequest = share.CookieRequest
type CookieQueryResult = share.CookieQueryResult
type ResourceBundle = share.ResourceBundle
type ResourceRequest = share.ResourceRequest
type PolicyEntry = share.PolicyEntry
type PolicyRequest = share.PolicyRequest
type PolicyBundle = share.PolicyBundle

type ManagementRequest = management.Request
type ManagementResponse = management.Response
type ManagementBackend = kit.ManagementBackend
type SessionTarget = management.SessionTarget
type InjectionOptions = management.InjectionOptions
type UserscriptRegistration = management.UserscriptRegistration
type ReplayResult = management.ReplayResult
type PageSessionRuntime = management.PageSessionRuntime
type PageSession = management.PageSession

const ManagementVersion = management.Version

type CookieBackend = kit.CookieBackend
type PolicyFile = kit.PolicyFile
type PolicyRoot = kit.PolicyRoot
type PolicySources = kit.PolicySources

func RunCookie(profile, operation string, input io.Reader, stdout, stderr io.Writer, backend CookieBackend) int {
	return kit.RunCookie(profile, operation, input, stdout, stderr, backend)
}

func RunPolicyExport(input io.Reader, stdout, stderr io.Writer, sources PolicySources) int {
	return kit.RunPolicyExport(input, stdout, stderr, sources)
}

func RunManagement(ctx context.Context, profile string, input io.Reader, stdout, stderr io.Writer, backend ManagementBackend) int {
	return kit.RunManagement(ctx, profile, input, stdout, stderr, backend)
}

func NewManagementRequest(kind, action string, input any) (ManagementRequest, error) {
	return management.NewRequest(kind, action, input)
}

func ValidateManagementRequest(request ManagementRequest) error {
	return management.ValidateRequest(request)
}

func ValidateManagementResponse(response ManagementResponse, request ManagementRequest) error {
	return management.ValidateResponse(response, request)
}

func ManagementOperations() map[string][]string { return management.Operations() }

func ManagementResult(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func ExportPolicies(sources PolicySources) (PolicyBundle, error) {
	return kit.ExportPolicies(sources)
}

func ManagedPreferenceFiles(domain string) []PolicyFile { return kit.ManagedPreferenceFiles(domain) }

func ParseSite(raw string) (*url.URL, error)         { return share.ParseSite(raw) }
func ValidateCookieBundle(bundle CookieBundle) error { return share.ValidateCookieBundle(bundle) }
func ValidateResourceBundle(bundle ResourceBundle, resource string) error {
	return share.ValidateResourceBundle(bundle, resource)
}
func SameListedCookie(a, b Cookie) bool { return share.SameListedCookie(a, b) }
func CookieActive(cookie Cookie) bool   { return share.CookieActive(cookie) }
func CookieDomainMatches(siteHost, cookieDomain string) bool {
	return share.CookieDomainMatches(siteHost, cookieDomain)
}
func CookieMatchesSite(site *url.URL, cookie Cookie) bool {
	return share.CookieMatchesSite(site, cookie)
}
func CookieMatchesSiteOptions(site *url.URL, cookie Cookie, includeExpired bool) bool {
	return share.CookieMatchesSiteOptions(site, cookie, includeExpired)
}
func CookiePathMatches(requestPath, cookiePath string) bool {
	return share.CookiePathMatches(requestPath, cookiePath)
}

func CookieHostSQL(host string) string { return kit.CookieHostSQL(host) }
func CookieDatabaseColumns(database, table string) ([]string, error) {
	return kit.CookieDatabaseColumns(database, table)
}
func ReadableCookieDatabase(database, table string) (string, func(), []string, error) {
	return kit.ReadableCookieDatabase(database, table)
}
func SnapshotCookieDatabase(database string) (string, func(), error) {
	return kit.SnapshotCookieDatabase(database)
}
func CopyPrivateFile(source, target string) error { return kit.CopyPrivateFile(source, target) }
func HasSQLiteColumn(columns []string, wanted string) bool {
	return kit.HasSQLiteColumn(columns, wanted)
}
func RunSQLite(database string, readonly bool, query string) ([]byte, error) {
	return kit.RunSQLite(database, readonly, query)
}
func SQLString(value string) string     { return kit.SQLString(value) }
func SQLIdentifier(value string) string { return kit.SQLIdentifier(value) }
func SQLBool(value bool) string         { return kit.SQLBool(value) }
