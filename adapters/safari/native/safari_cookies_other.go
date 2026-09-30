//go:build !darwin

package main

import (
	"errors"
	"net/url"

	browsershare "github.com/webong/ctx/internal/app/browser/share"
)

func safariShareStatus() map[string]string {
	return map[string]string{"policy.export": "blocked", "cookie.list": "blocked", "cookie.export": "blocked"}
}

func readSafariSiteCookies(string, *url.URL, string) ([]browsershare.Cookie, string, error) {
	return nil, "", errors.New("Safari cookies are only available on macOS")
}

func readSafariCookieValue(string, browsershare.Cookie) (string, error) {
	return "", errors.New("Safari cookies are only available on macOS")
}
