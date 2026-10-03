// Package store owns macOS Keychain identity and access for the Keychain adapter.
package store

import (
	"errors"
	"net/url"
	"strings"
)

// Keychain uses the default keychain unless Path names an explicitly opened
// keychain, primarily for isolated fixtures and caller-owned stores.
type Keychain struct{ Path string }

func parseItem(item string) (service, account string, err error) {
	values, err := url.ParseQuery(item)
	if err != nil || len(values) != 2 || len(values["service"]) != 1 || len(values["account"]) != 1 {
		return "", "", errors.New("Keychain item must be service=<name>&account=<name>")
	}
	service, account = values.Get("service"), values.Get("account")
	if service == "" || account == "" || strings.ContainsRune(service, 0) || strings.ContainsRune(account, 0) {
		return "", "", errors.New("Keychain service and account are required")
	}
	return service, account, nil
}
