//go:build darwin

package chromium

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This opt-in test creates its own keychain and dummy item. It does not read
// browser credentials. Run it on a disposable CI host or explicitly opt in.
func TestNativeCookieCredentialsMacOSKeychain(t *testing.T) {
	if os.Getenv("CTX_COOKIE_NATIVE_CREDENTIAL_TESTS") != "1" {
		t.Skip("set CTX_COOKIE_NATIVE_CREDENTIAL_TESTS=1 for isolated OS credential fixtures")
	}
	keychain := filepath.Join(t.TempDir(), "ctx-fixture.keychain-db")
	run := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "/usr/bin/security", args...).Run(); err != nil {
			t.Fatalf("synthetic keychain operation %s failed: %v", args[0], err)
		}
	}
	run("create-keychain", "-p", "ctx-synthetic-keychain-password", keychain)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "/usr/bin/security", "delete-keychain", keychain).Run(); err != nil {
			t.Errorf("remove synthetic keychain: %v", err)
		}
	})
	run("unlock-keychain", "-p", "ctx-synthetic-keychain-password", keychain)
	run("add-generic-password", "-s", "CTX Synthetic Safe Storage", "-a", "CTX Synthetic",
		"-w", "ctx-synthetic-safe-storage-password", "-T", "/usr/bin/security", keychain)
	config := Config{Name: "fixture", KeychainService: "CTX Synthetic Safe Storage", KeychainAccount: "CTX Synthetic", KeychainPath: keychain}
	t.Setenv("CTX_BROWSER_FIXTURE_SAFE_STORAGE_PASSWORD_FILE", "")
	password, err := chromiumMacKeychainPassword(config)
	if err != nil || password != "ctx-synthetic-safe-storage-password" {
		t.Fatalf("isolated Keychain lookup failed: %v", err)
	}
	encrypted := fixtureChromiumCBC(t, password, 1003, "v10")
	decryptor := newChromiumCookieDecryptor(config)
	for i := 0; i < 2; i++ {
		if value, err := decryptor.decrypt("unused", encrypted); err != nil || value != "synthetic-cookie-value" {
			t.Fatalf("Keychain-backed cookie decryption failed: %v", err)
		}
	}
}
