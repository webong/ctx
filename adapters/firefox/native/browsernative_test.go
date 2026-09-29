package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCookieDomainMatches(t *testing.T) {
	cases := []struct {
		site, domain string
		want         bool
	}{
		{"app.example.test", ".example.test", true},
		{"example.test", ".example.test", true},
		{"app.example.test", "example.test", false},
		{"otherexample.test", ".example.test", false},
	}
	for _, test := range cases {
		if got := cookieDomainMatches(test.site, test.domain); got != test.want {
			t.Errorf("cookieDomainMatches(%q, %q) = %t, want %t", test.site, test.domain, got, test.want)
		}
	}
}

func TestFirefoxCookieReadOnlyWALSnapshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only directory permissions differ on Windows")
	}
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 is not installed")
	}
	root := t.TempDir()
	activeDB := filepath.Join(root, "active.sqlite")
	if output, err := exec.Command("sqlite3", activeDB, `CREATE TABLE moz_cookies (id INTEGER PRIMARY KEY, name TEXT, value TEXT);`).CombinedOutput(); err != nil {
		t.Fatalf("create fixture: %v %s", err, output)
	}
	command := exec.Command("sqlite3", activeDB)
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	if _, err := input.Write([]byte("PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; INSERT INTO moz_cookies(name,value) VALUES('session','fixture-secret');\n")); err != nil {
		t.Fatal(err)
	}
	wal := activeDB + "-wal"
	deadline := time.Now().Add(3 * time.Second)
	for {
		if info, err := os.Stat(wal); err == nil && info.Size() > 32 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture did not create a write-ahead log")
		}
		time.Sleep(10 * time.Millisecond)
	}
	lockedDir := filepath.Join(root, "read-only")
	if err := os.Mkdir(lockedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	lockedDB := filepath.Join(lockedDir, "cookies.sqlite")
	if err := copyPrivateFile(activeDB, lockedDB); err != nil {
		t.Fatal(err)
	}
	if err := copyPrivateFile(wal, lockedDB+"-wal"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(lockedDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedDir, 0o700) })
	readable, cleanup, columns, err := readableFirefoxCookieDatabase(lockedDB)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if readable == lockedDB || !hasSQLiteColumn(columns, "value") {
		t.Fatalf("expected private WAL snapshot, got path=%q columns=%v", readable, columns)
	}
	output, err := runSQLite(readable, true, "SELECT value FROM moz_cookies WHERE name='session'")
	if err != nil || !strings.Contains(string(output), "fixture-secret") {
		t.Fatalf("snapshot did not replay WAL: %v", err)
	}
}
