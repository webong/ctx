//go:build darwin

package main

import (
	"encoding/binary"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func safariCookieFixture() []byte {
	record := make([]byte, 56)
	for index, field := range []string{".example.test", "session", "/account", "fixture-secret"} {
		binary.LittleEndian.PutUint32(record[16+index*4:], uint32(len(record)))
		record = append(record, []byte(field)...)
		record = append(record, 0)
	}
	binary.LittleEndian.PutUint32(record[:4], uint32(len(record)))
	binary.LittleEndian.PutUint32(record[8:12], 5) // Secure and HttpOnly
	binary.LittleEndian.PutUint64(record[40:48], math.Float64bits(float64(2000000000-safariEpochUnix)))
	page := make([]byte, 16)
	copy(page, []byte{0, 0, 1, 0})
	binary.LittleEndian.PutUint32(page[4:8], 1)
	binary.LittleEndian.PutUint32(page[8:12], 16)
	page = append(page, record...)
	store := make([]byte, 12)
	copy(store, "cook")
	binary.BigEndian.PutUint32(store[4:8], 1)
	binary.BigEndian.PutUint32(store[8:12], uint32(len(page)))
	return append(store, page...)
}

func TestSafariCookieDiskSnapshotAndCorruption(t *testing.T) {
	file := filepath.Join(t.TempDir(), "Cookies.binarycookies")
	data := safariCookieFixture()
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	site, _ := url.Parse("https://example.test/account")
	cookies, _, err := readSafariSiteCookies(file, site, "")
	if err != nil || len(cookies) != 1 || cookies[0].Value != "" || !cookies[0].Secure || !cookies[0].HTTPOnly || cookies[0].Expiry != 2000000000 {
		t.Fatalf("metadata: cookies=%d err=%v", len(cookies), err)
	}
	value, err := readSafariCookieValue(file, cookies[0])
	if err != nil || value != "fixture-secret" {
		t.Fatalf("disk value: %v", err)
	}
	other, _ := url.Parse("https://other.test/account")
	if cookies, _, err := readSafariSiteCookies(file, other, ""); err != nil || len(cookies) != 0 {
		t.Fatal("unrelated cookie returned")
	}
	// A changed store invalidates references obtained from the old snapshot.
	if err := os.WriteFile(file, append(data, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSafariCookieValue(file, cookies[0]); err == nil {
		t.Fatal("old reference accepted after store change")
	}
	bad := append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(bad[20:24], math.MaxUint32)
	if err := os.WriteFile(file, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSafariCookieStore(file, true); err == nil {
		t.Fatal("invalid record offset accepted")
	}
	bad = append([]byte(nil), data...)
	binary.LittleEndian.PutUint64(bad[68:76], math.Float64bits(math.NaN()))
	if err := os.WriteFile(file, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSafariCookieStore(file, true); err == nil {
		t.Fatal("nonfinite expiry accepted")
	}
}
