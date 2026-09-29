package app

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const chromiumEpochOffsetMicros = int64(11644473600000000)

type chromiumCookieRow struct {
	ID                   int64  `json:"id"`
	Name                 string `json:"name"`
	Host                 string `json:"host"`
	Path                 string `json:"path"`
	ExpiresUTC           int64  `json:"expiresUTC"`
	IsSecure             int    `json:"isSecure"`
	IsHTTPOnly           int    `json:"isHttpOnly"`
	SameSite             int    `json:"sameSite"`
	TopFrameSiteKey      string `json:"topFrameSiteKey"`
	HasCrossSiteAncestor int    `json:"hasCrossSiteAncestor"`
	SourceScheme         int    `json:"sourceScheme"`
	SourcePort           int    `json:"sourcePort"`
	HasExpires           int    `json:"hasExpires"`
}

func readChromiumSiteCookies(provider, profile string, site *url.URL, name string) ([]browserCookie, string, error) {
	database, err := chromiumCookieDatabase(provider, profile)
	if err != nil {
		return nil, "", err
	}
	readable, cleanup, columns, err := readableCookieDatabase(database, "cookies")
	if err != nil {
		return nil, "", err
	}
	defer cleanup()
	for _, required := range []string{"host_key", "name", "path", "expires_utc", "is_secure", "is_httponly", "samesite", "value", "encrypted_value"} {
		if !hasSQLiteColumn(columns, required) {
			return nil, "", fmt.Errorf("%s cookie database lacks %s; this profile schema is not supported", provider, required)
		}
	}
	host := strings.TrimSuffix(strings.ToLower(site.Hostname()), ".")
	if host == "" {
		return nil, "", errors.New("site has no hostname")
	}
	statement := "SELECT rowid AS id,name,host_key AS host,path,expires_utc AS expiresUTC," +
		"is_secure AS isSecure,is_httponly AS isHttpOnly,samesite AS sameSite," +
		chromiumColumnExpr(columns, "top_frame_site_key", "''", "topFrameSiteKey") + "," +
		chromiumColumnExpr(columns, "has_cross_site_ancestor", "0", "hasCrossSiteAncestor") + "," +
		chromiumColumnExpr(columns, "source_scheme", "0", "sourceScheme") + "," +
		chromiumColumnExpr(columns, "source_port", "0", "sourcePort") + "," +
		chromiumColumnExpr(columns, "has_expires", "1", "hasExpires") +
		" FROM cookies WHERE host_key IN (" + cookieHostSQL(host) + ")"
	if name != "" {
		statement += " AND name=" + sqlString(name)
	}
	output, err := runSQLite(readable, true, statement)
	if err != nil {
		return nil, "", err
	}
	var rows []chromiumCookieRow
	if len(bytes.TrimSpace(output)) != 0 {
		if err := json.Unmarshal(output, &rows); err != nil {
			return nil, "", errors.New("cannot decode Chromium cookie database response")
		}
	}
	cookies := make([]browserCookie, 0, len(rows))
	for _, row := range rows {
		expiry := int64(0)
		if row.HasExpires != 0 {
			if row.ExpiresUTC <= chromiumEpochOffsetMicros {
				expiry = -1
			} else {
				expiry = (row.ExpiresUTC - chromiumEpochOffsetMicros) / 1_000_000
			}
		}
		cookie := browserCookie{
			ID: row.ID, Name: row.Name, Domain: row.Host, Path: row.Path,
			Expiry: expiry, Secure: row.IsSecure != 0, HTTPOnly: row.IsHTTPOnly != 0,
			SameSite: row.SameSite, SameSitePolicy: chromiumSameSitePolicy(row.SameSite),
			PartitionKey: row.TopFrameSiteKey, CrossSiteAncestor: row.HasCrossSiteAncestor != 0,
			SourceScheme: row.SourceScheme, SourcePort: row.SourcePort,
		}
		if cookieDomainMatches(host, cookie.Domain) && (!cookie.Secure || site.Scheme == "https") && cookieActive(cookie) {
			cookies = append(cookies, cookie)
		}
	}
	return cookies, database, nil
}

func chromiumColumnExpr(columns []string, column, fallback, alias string) string {
	if hasSQLiteColumn(columns, column) {
		return sqlIdentifier(column) + " AS " + sqlIdentifier(alias)
	}
	return fallback + " AS " + sqlIdentifier(alias)
}

func chromiumSameSitePolicy(raw int) string {
	switch raw {
	case -1:
		return "unspecified"
	case 0:
		return "none"
	case 1:
		return "lax"
	case 2:
		return "strict"
	default:
		return "unknown"
	}
}

func chromiumCookieDatabase(provider, profile string) (string, error) {
	if profile == "" || profile == "." || profile == ".." || filepath.Base(profile) != profile || strings.ContainsAny(profile, `/\\`) {
		return "", errors.New("invalid Chromium profile directory")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var root string
	switch runtime.GOOS {
	case "darwin":
		if provider == "chrome" {
			root = filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
		} else {
			root = filepath.Join(home, "Library", "Application Support", "Chromium")
		}
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			return "", errors.New("LOCALAPPDATA is not set")
		}
		if provider == "chrome" {
			root = filepath.Join(local, "Google", "Chrome", "User Data")
		} else {
			root = filepath.Join(local, "Chromium", "User Data")
		}
	default:
		root = os.Getenv("XDG_CONFIG_HOME")
		if root == "" {
			root = filepath.Join(home, ".config")
		}
		if provider == "chrome" {
			root = filepath.Join(root, "google-chrome")
		} else {
			root = filepath.Join(root, "chromium")
		}
	}
	profileDir := filepath.Join(root, profile)
	if info, err := os.Stat(filepath.Join(profileDir, "Preferences")); err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s profile %q is unavailable", provider, profile)
	}
	for _, candidate := range []string{filepath.Join(profileDir, "Network", "Cookies"), filepath.Join(profileDir, "Cookies")} {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s cookie database is unavailable for profile %q", provider, profile)
}

func readChromiumCookieValue(provider, database string, cookie browserCookie) (string, error) {
	readable, cleanup, columns, err := readableCookieDatabase(database, "cookies")
	if err != nil {
		return "", err
	}
	defer cleanup()
	statement := "SELECT value,hex(encrypted_value) AS encryptedHex FROM cookies WHERE rowid=" + strconv.FormatInt(cookie.ID, 10) +
		" AND name=" + sqlString(cookie.Name) + " AND host_key=" + sqlString(cookie.Domain) + " AND path=" + sqlString(cookie.Path)
	if hasSQLiteColumn(columns, "top_frame_site_key") {
		statement += " AND top_frame_site_key=" + sqlString(cookie.PartitionKey)
	}
	if hasSQLiteColumn(columns, "has_cross_site_ancestor") {
		ancestor := 0
		if cookie.CrossSiteAncestor {
			ancestor = 1
		}
		statement += " AND has_cross_site_ancestor=" + strconv.Itoa(ancestor)
	}
	if hasSQLiteColumn(columns, "source_scheme") {
		statement += " AND source_scheme=" + strconv.Itoa(cookie.SourceScheme)
	}
	if hasSQLiteColumn(columns, "source_port") {
		statement += " AND source_port=" + strconv.Itoa(cookie.SourcePort)
	}
	output, err := runSQLite(readable, true, statement)
	if err != nil {
		return "", err
	}
	var rows []struct {
		Value        string `json:"value"`
		EncryptedHex string `json:"encryptedHex"`
	}
	if err := json.Unmarshal(output, &rows); err != nil || len(rows) != 1 {
		return "", errors.New("Chromium cookie changed since listing; retry")
	}
	if rows[0].Value != "" && rows[0].EncryptedHex != "" {
		return "", errors.New("Chromium cookie contains both plaintext and encrypted values")
	}
	if rows[0].EncryptedHex == "" {
		return rows[0].Value, nil
	}
	ciphertext, err := hex.DecodeString(rows[0].EncryptedHex)
	if err != nil {
		return "", errors.New("Chromium cookie ciphertext is malformed")
	}
	plaintext, err := decryptChromiumCookie(provider, database, ciphertext)
	if err != nil {
		return "", err
	}
	versionOutput, err := runSQLite(readable, true, "SELECT value AS version FROM meta WHERE key='version'")
	if err != nil {
		return "", fmt.Errorf("cannot determine Chromium cookie database version: %w", err)
	}
	var versions []struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(versionOutput, &versions); err != nil || len(versions) != 1 {
		return "", errors.New("cannot determine Chromium cookie database version")
	}
	if versions[0].Version >= 24 {
		// Chromium database v24 binds the encrypted value to its host key.
		// See net/extras/sqlite/sqlite_persistent_cookie_store.cc.
		hostHash := sha256.Sum256([]byte(cookie.Domain))
		if len(plaintext) < len(hostHash) || !bytes.Equal([]byte(plaintext[:len(hostHash)]), hostHash[:]) {
			return "", errors.New("Chromium cookie domain hash does not match; cookie was not exported")
		}
		plaintext = plaintext[len(hostHash):]
	}
	if !utf8.ValidString(plaintext) {
		return "", errors.New("Chromium cookie value is not valid UTF-8")
	}
	return plaintext, nil
}

func decryptChromiumCookie(provider, database string, ciphertext []byte) (string, error) {
	if len(ciphertext) < 3 {
		return "", errors.New("Chromium cookie uses an unsupported encryption format")
	}
	version := string(ciphertext[:3])
	if runtime.GOOS == "windows" {
		return decryptChromiumWindowsCookie(database, ciphertext)
	}
	var password string
	var iterations int
	switch runtime.GOOS {
	case "darwin":
		if version != "v10" {
			return "", errors.New("Chromium cookie encryption format is not supported on macOS")
		}
		secret, err := chromiumMacKeychainPassword(provider)
		if err != nil {
			return "", err
		}
		password, iterations = secret, 1003
	case "linux":
		switch version {
		case "v10":
			password, iterations = "peanuts", 1
		case "v11":
			secret, err := chromiumLinuxSecret(provider)
			if err != nil {
				return "", err
			}
			password, iterations = secret, 1
		default:
			return "", errors.New("Chromium cookie encryption format is not supported on Linux")
		}
	default:
		return "", fmt.Errorf("encrypted Chromium cookie export is not supported on %s", runtime.GOOS)
	}
	key := chromiumPBKDF2Key([]byte(password), iterations)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.New("cannot initialize Chromium cookie decryptor")
	}
	data := ciphertext[3:]
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return "", errors.New("Chromium cookie ciphertext has an invalid length")
	}
	plaintext := make([]byte, len(data))
	iv := bytes.Repeat([]byte(" "), aes.BlockSize)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, data)
	padding := int(plaintext[len(plaintext)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(plaintext) {
		return "", errors.New("Chromium cookie decryption failed")
	}
	for _, value := range plaintext[len(plaintext)-padding:] {
		if int(value) != padding {
			return "", errors.New("Chromium cookie decryption failed")
		}
	}
	return string(plaintext[:len(plaintext)-padding]), nil
}

func decryptChromiumWindowsCookie(database string, ciphertext []byte) (string, error) {
	if bytes.HasPrefix(ciphertext, []byte("v20")) {
		return "", errors.New("Chrome cookie uses App-Bound Encryption; standalone profile export is unavailable")
	}
	if !bytes.HasPrefix(ciphertext, []byte("v10")) && !bytes.HasPrefix(ciphertext, []byte("v11")) {
		plaintext, err := windowsDPAPIUnprotect(ciphertext)
		return string(plaintext), err
	}
	key, err := chromiumWindowsLegacyKey(database)
	if err != nil {
		return "", err
	}
	data := ciphertext[3:]
	if len(data) < 12+16 {
		return "", errors.New("Chromium cookie ciphertext is too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, data[:12], data[12:], nil)
	if err != nil {
		return "", errors.New("Chromium cookie decryption failed")
	}
	return string(plaintext), nil
}

func chromiumWindowsLegacyKey(database string) ([]byte, error) {
	root := chromiumUserDataRootFromDatabase(database)
	content, err := os.ReadFile(filepath.Join(root, "Local State"))
	if err != nil {
		return nil, fmt.Errorf("cannot read Chromium Local State: %w", err)
	}
	var state struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(content, &state); err != nil || state.OSCrypt.EncryptedKey == "" {
		return nil, errors.New("Chromium Local State has no legacy encrypted key")
	}
	encoded, err := base64.StdEncoding.DecodeString(state.OSCrypt.EncryptedKey)
	if err != nil || !bytes.HasPrefix(encoded, []byte("DPAPI")) {
		return nil, errors.New("Chromium Local State uses an unsupported key format")
	}
	return windowsDPAPIUnprotect(encoded[5:])
}

func chromiumUserDataRootFromDatabase(database string) string {
	profileDir := filepath.Dir(database)
	if filepath.Base(profileDir) == "Network" {
		profileDir = filepath.Dir(profileDir)
	}
	return filepath.Dir(profileDir)
}

func windowsDPAPIUnprotect(ciphertext []byte) ([]byte, error) {
	const script = `Add-Type -AssemblyName System.Security; $raw=[Convert]::FromBase64String($env:CTX_DPAPI_DATA); $clear=[Security.Cryptography.ProtectedData]::Unprotect($raw,$null,[Security.Cryptography.DataProtectionScope]::CurrentUser); [Console]::Out.Write([Convert]::ToBase64String($clear))`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	command.Env = append(os.Environ(), "CTX_DPAPI_DATA="+base64.StdEncoding.EncodeToString(ciphertext))
	output, err := command.Output()
	if err != nil {
		return nil, errors.New("Windows DPAPI could not unlock this Chromium cookie for the current user")
	}
	plaintext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(output)))
	if err != nil {
		return nil, errors.New("Windows DPAPI returned malformed data")
	}
	return plaintext, nil
}

func chromiumPBKDF2Key(password []byte, iterations int) []byte {
	// Chromium's desktop OSCrypt v10/v11 key derivation uses PBKDF2-HMAC-SHA1
	// with saltysalt; macOS uses 1003 rounds and Linux uses one.
	mac := hmac.New(sha1.New, password)
	_, _ = mac.Write([]byte("saltysalt\x00\x00\x00\x01"))
	previous := mac.Sum(nil)
	derived := append([]byte(nil), previous...)
	for i := 1; i < iterations; i++ {
		mac.Reset()
		_, _ = mac.Write(previous)
		previous = mac.Sum(nil)
		for j := range derived {
			derived[j] ^= previous[j]
		}
	}
	return derived[:16]
}

func chromiumMacKeychainPassword(provider string) (string, error) {
	service, account := "Chrome Safe Storage", "Chrome"
	if provider == "chromium" {
		service, account = "Chromium Safe Storage", "Chromium"
	}
	if _, err := exec.LookPath("security"); err != nil {
		return "", errors.New("macOS security command is required to unlock Chromium cookies")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, "security", "find-generic-password", "-w", "-s", service, "-a", account).Output()
	if err != nil || len(output) == 0 {
		return "", fmt.Errorf("cannot read %s from macOS Keychain; unlock it for ctx and retry", service)
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r"), nil
}

func chromiumLinuxSecret(provider string) (string, error) {
	if strings.Contains(strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP")), "KDE") {
		if secret, err := chromiumKWalletSecret(provider); err == nil {
			return secret, nil
		}
		if secret, err := chromiumSecretServiceSecret(provider); err == nil {
			return secret, nil
		}
	} else {
		if secret, err := chromiumSecretServiceSecret(provider); err == nil {
			return secret, nil
		}
		if secret, err := chromiumKWalletSecret(provider); err == nil {
			return secret, nil
		}
	}
	return "", fmt.Errorf("cannot read the %s cookie key from Secret Service or KWallet", provider)
}

func chromiumSecretServiceSecret(provider string) (string, error) {
	if _, err := exec.LookPath("secret-tool"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		output, lookupErr := exec.CommandContext(ctx, "secret-tool", "lookup", "application", provider).Output()
		cancel()
		if lookupErr == nil && len(output) > 0 {
			return strings.TrimRight(string(output), "\r\n"), nil
		}
	}
	return "", errors.New("Secret Service key unavailable")
}

func chromiumKWalletSecret(provider string) (string, error) {
	if _, err := exec.LookPath("kwallet-query"); err == nil {
		folder, key := "Chromium Keys", "Chromium Safe Storage"
		if provider == "chrome" {
			folder, key = "Chrome Keys", "Chrome Safe Storage"
		}
		wallet := os.Getenv("CTX_KWALLET_NAME")
		if wallet == "" {
			wallet = "kdewallet"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		output, lookupErr := exec.CommandContext(ctx, "kwallet-query", "-f", folder, "-r", key, wallet).Output()
		cancel()
		if lookupErr == nil && len(output) > 0 {
			return strings.TrimRight(string(output), "\r\n"), nil
		}
	}
	return "", errors.New("KWallet key unavailable")
}
