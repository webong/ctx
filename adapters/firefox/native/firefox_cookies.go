package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	browsershare "github.com/webong/ctx/browser/share"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type firefoxCookieRow struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Value            string `json:"value"`
	Host             string `json:"host"`
	Path             string `json:"path"`
	Expiry           int64  `json:"expiry"`
	IsSecure         int    `json:"isSecure"`
	IsHTTPOnly       int    `json:"isHttpOnly"`
	SameSite         int    `json:"sameSite"`
	OriginAttributes string `json:"originAttributes"`
}

func readFirefoxSiteCookies(profile string, site *url.URL, name string) ([]browserCookie, string, error) {
	database, err := firefoxCookieDatabase(profile)
	if err != nil {
		return nil, "", err
	}
	readableDB, cleanup, columns, err := readableFirefoxCookieDatabase(database)
	if err != nil {
		return nil, "", err
	}
	defer cleanup()
	for _, required := range []string{"id", "name", "value", "host", "path", "expiry", "isSecure", "isHttpOnly", "sameSite", "originAttributes"} {
		if !hasSQLiteColumn(columns, required) {
			return nil, "", fmt.Errorf("Firefox cookie database lacks %s; this profile schema is not supported", required)
		}
	}
	host := strings.TrimSuffix(strings.ToLower(site.Hostname()), ".")
	if host == "" {
		return nil, "", errors.New("site has no hostname")
	}
	statement := "SELECT id,name,host,path,expiry,isSecure,isHttpOnly,sameSite,originAttributes FROM moz_cookies WHERE host IN (" + cookieHostSQL(host) + ")"
	if name != "" {
		statement += " AND name=" + sqlString(name)
	}
	output, err := runSQLite(readableDB, true, statement)
	if err != nil {
		return nil, "", err
	}
	var rows []firefoxCookieRow
	if len(bytes.TrimSpace(output)) != 0 {
		if err := json.Unmarshal(output, &rows); err != nil {
			return nil, "", errors.New("cannot decode Firefox cookie database response")
		}
	}
	cookies := make([]browserCookie, 0, len(rows))
	for _, row := range rows {
		cookie := browserCookie{
			ID: row.ID, Name: row.Name, Domain: row.Host, Path: row.Path,
			Expiry: row.Expiry, Secure: row.IsSecure != 0, HTTPOnly: row.IsHTTPOnly != 0,
			SameSitePolicy: firefoxSameSitePolicy(row.SameSite),
		}
		if row.OriginAttributes != "" {
			cookie.Attributes = map[string]string{"firefox.origin_attributes": row.OriginAttributes}
		}
		if cookieDomainMatches(host, cookie.Domain) && (!cookie.Secure || site.Scheme == "https") && cookieActive(cookie) {
			cookies = append(cookies, cookie)
		}
	}
	return cookies, database, nil
}

func readFirefoxCookieValue(database string, cookie browserCookie) (string, error) {
	readableDB, cleanup, _, err := readableFirefoxCookieDatabase(database)
	if err != nil {
		return "", err
	}
	defer cleanup()
	statement := "SELECT value FROM moz_cookies WHERE id=" + strconv.FormatInt(cookie.ID, 10) +
		" AND name=" + sqlString(cookie.Name) + " AND host=" + sqlString(cookie.Domain) +
		" AND path=" + sqlString(cookie.Path) + " AND originAttributes=" + sqlString(cookie.Attributes["firefox.origin_attributes"])
	output, err := runSQLite(readableDB, true, statement)
	if err != nil {
		return "", err
	}
	var rows []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(output, &rows); err != nil || len(rows) != 1 {
		return "", errors.New("Firefox cookie changed since listing; retry")
	}
	return rows[0].Value, nil
}

func firefoxSameSitePolicy(raw int) string {
	switch raw {
	case 0:
		return "none"
	case 1:
		return "lax"
	case 2:
		return "strict"
	case 256:
		return "unspecified"
	default:
		return "unknown"
	}
}

func cookieDomainMatches(siteHost, cookieDomain string) bool {
	return browsershare.CookieDomainMatches(siteHost, cookieDomain)
}

func firefoxCookieDatabase(profile string) (string, error) {
	directory, err := firefoxProfileDirectory(profile)
	if err != nil {
		return "", err
	}
	database := filepath.Join(directory, "cookies.sqlite")
	info, err := os.Stat(database)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Firefox cookie database is unavailable for profile %q", profile)
	}
	return database, nil
}

func firefoxProfileDirectory(profile string) (string, error) {
	ini, err := firefoxProfilesINI()
	if err != nil {
		return "", err
	}
	file, err := os.Open(ini)
	if err != nil {
		return "", fmt.Errorf("cannot open Firefox profiles.ini: %w", err)
	}
	defer file.Close()
	var matches []string
	section := ""
	fields := map[string]string{}
	flush := func() {
		if !strings.HasPrefix(section, "Profile") || fields["Name"] != profile || fields["Path"] == "" {
			return
		}
		path := filepath.FromSlash(fields["Path"])
		if fields["IsRelative"] != "0" && !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(ini), path)
		}
		matches = append(matches, filepath.Clean(path))
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			fields = map[string]string{}
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("cannot read Firefox profiles.ini: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("Firefox profile %q is not listed in profiles.ini", profile)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("Firefox profile name %q is ambiguous", profile)
	}
	return matches[0], nil
}

func firefoxProfilesINI() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Firefox", "profiles.ini"), nil
	case "windows":
		root := os.Getenv("APPDATA")
		if root == "" {
			return "", errors.New("APPDATA is not set")
		}
		return filepath.Join(root, "Mozilla", "Firefox", "profiles.ini"), nil
	default:
		standard := filepath.Join(home, ".mozilla", "firefox", "profiles.ini")
		if _, err := os.Stat(standard); err == nil {
			return standard, nil
		}
		return filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox", "profiles.ini"), nil
	}
}

func firefoxCookieColumns(database string) ([]string, error) {
	return cookieDatabaseColumns(database, "moz_cookies")
}

// Firefox keeps these names for profile-specific callers and its WAL fixture.
func readableFirefoxCookieDatabase(database string) (string, func(), []string, error) {
	return readableCookieDatabase(database, "moz_cookies")
}

func snapshotFirefoxCookieDatabase(database string) (string, func(), error) {
	return snapshotCookieDatabase(database)
}

func importFirefoxCookie(profile string, cookie browserCookie, replace bool) error {
	if len(cookie.Attributes) != 0 || cookie.PartitionKey != "" {
		return errors.New("Firefox profile import cannot map a container or partitioned cookie")
	}
	database, err := firefoxCookieDatabase(profile)
	if err != nil {
		return err
	}
	if err := ensureFirefoxProfileClosed(database); err != nil {
		return err
	}
	columns, err := firefoxCookieColumns(database)
	if err != nil {
		return err
	}
	for _, required := range []string{"name", "value", "host", "path", "expiry", "isSecure", "isHttpOnly", "sameSite", "originAttributes"} {
		if !hasSQLiteColumn(columns, required) {
			return fmt.Errorf("Firefox target cookie database lacks %s", required)
		}
	}
	sameSite, err := firefoxImportedSameSite(cookie)
	if err != nil {
		return err
	}
	identity := "name=" + sqlString(cookie.Name) + " AND host=" + sqlString(cookie.Domain) + " AND path=" + sqlString(cookie.Path) + " AND originAttributes=''"
	if !replace {
		output, err := runSQLite(database, true, "SELECT count(*) AS existing FROM moz_cookies WHERE "+identity)
		if err != nil {
			return err
		}
		var counts []struct {
			Existing int `json:"existing"`
		}
		if err := json.Unmarshal(output, &counts); err != nil || len(counts) != 1 {
			return errors.New("cannot check destination Firefox cookie")
		}
		if counts[0].Existing != 0 {
			return errors.New("destination already has this cookie; use --replace to overwrite it")
		}
	}
	values := map[string]string{
		"name": sqlString(cookie.Name), "value": sqlString(cookie.Value),
		"host": sqlString(cookie.Domain), "path": sqlString(cookie.Path),
		"expiry": strconv.FormatInt(cookie.Expiry, 10), "isSecure": sqlBool(cookie.Secure),
		"isHttpOnly": sqlBool(cookie.HTTPOnly), "sameSite": strconv.Itoa(sameSite),
		"originAttributes": "''",
		"creationTime":     strconv.FormatInt(time.Now().UnixMicro(), 10),
		"lastAccessed":     strconv.FormatInt(time.Now().UnixMicro(), 10),
	}
	var names, expressions []string
	for _, column := range columns {
		if value, ok := values[column]; ok {
			names = append(names, sqlIdentifier(column))
			expressions = append(expressions, value)
		}
	}
	statement := "BEGIN IMMEDIATE; "
	if replace {
		statement += "DELETE FROM moz_cookies WHERE " + identity + "; "
	}
	statement += "INSERT INTO moz_cookies (" + strings.Join(names, ",") + ") VALUES (" + strings.Join(expressions, ",") + "); COMMIT;"
	_, err = runSQLite(database, false, statement)
	return err
}

func firefoxImportedSameSite(cookie browserCookie) (int, error) {
	switch cookie.SameSitePolicy {
	case "none":
		return 0, nil
	case "lax":
		return 1, nil
	case "strict":
		return 2, nil
	case "unspecified", "":
		return 256, nil
	default:
		return 0, errors.New("unsupported cookie SameSite policy")
	}
}

func ensureFirefoxProfileClosed(database string) error {
	if _, err := exec.LookPath("lsof"); err != nil {
		return errors.New("lsof is required to check Firefox profile locks before copying")
	}
	profileDir := filepath.Dir(database)
	for _, path := range []string{database, filepath.Join(profileDir, ".parentlock"), filepath.Join(profileDir, "parent.lock"), filepath.Join(profileDir, "lock")} {
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		command := exec.CommandContext(ctx, "lsof", "-t", path)
		output, err := command.Output()
		cancel()
		if err == nil && len(bytes.TrimSpace(output)) != 0 {
			return fmt.Errorf("Firefox profile %s appears to be open; close it before copying cookies", profileDir)
		}
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
				continue
			}
			return fmt.Errorf("cannot check Firefox profile lock in %s", profileDir)
		}
	}
	return nil
}
