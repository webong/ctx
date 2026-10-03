// Package store owns Windows Credential Manager identity and API calls.
package store

import (
	"errors"
	"net/url"
	"strings"
)

type CredentialManager struct{}

func parseItem(item string) (string, error) {
	values, err := url.ParseQuery(item)
	if err != nil || len(values) != 1 || len(values["target"]) != 1 || values.Get("target") == "" || strings.ContainsRune(values.Get("target"), 0) {
		return "", errors.New("Credential Manager item must be target=<name>")
	}
	return values.Get("target"), nil
}
