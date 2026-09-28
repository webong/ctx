package adapter

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixtureAdapter(t *testing.T, root, name, kind string) string {
	t.Helper()
	directory := filepath.Join(root, name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	selector := name
	capabilities := "list,validate,run,doctor"
	if kind == "browser" {
		selector = "browser"
		capabilities = "list,validate,open,doctor"
	}
	manifest := `api_version = "1"
name = "` + name + `"
kind = "` + kind + `"
executable = "ctx-` + name + `"
description = "Test adapter"
capabilities = "` + capabilities + `"
selector_key = "` + selector + `"
commands = "` + name + `"
first_party = "false"
`
	if err := os.WriteFile(filepath.Join(directory, "adapter.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	executable := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(filepath.Join(directory, "ctx-"+name), executable, 0o755); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestStoreInstallTrustAndTamper(t *testing.T) {
	root := t.TempDir()
	source := fixtureAdapter(t, filepath.Join(root, "source"), "echo", "selector")
	store := NewStore(filepath.Join(root, "installed"))
	installed, err := store.Install(source)
	if err != nil {
		t.Fatal(err)
	}
	if trusted, err := store.IsTrusted(installed); err != nil || trusted {
		t.Fatalf("new adapter unexpectedly trusted: trusted=%v err=%v", trusted, err)
	}
	if err := store.Trust(installed); err != nil {
		t.Fatal(err)
	}
	if err := store.AssertTrusted(installed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed.ExecutablePath(), []byte("changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if trusted, err := store.IsTrusted(installed); err != nil || trusted {
		t.Fatalf("modified adapter remained trusted: trusted=%v err=%v", trusted, err)
	}
}

func TestManifestDefaultsAndKinds(t *testing.T) {
	root := t.TempDir()
	directory := fixtureAdapter(t, root, "firefox", "browser")
	loaded, err := LoadDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Manifest.Kind != "browser" || loaded.Manifest.SelectorKey != "browser" || !loaded.HasCapability("open") {
		t.Fatalf("unexpected manifest: %#v", loaded.Manifest)
	}
}

func TestCommandProtocol(t *testing.T) {
	directory := fixtureAdapter(t, t.TempDir(), "echo", "selector")
	loaded, err := LoadDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	command, err := loaded.Command(Invocation{
		Operation: "run", Selection: "staging", Arguments: []string{"hello"},
		Values: map[string]string{"echo": "staging"}, Profile: "client", Project: "/project", Command: "echoctl",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(command.Args, " ")
	if !strings.Contains(joined, "run staging -- hello") {
		t.Fatalf("unexpected command arguments: %q", joined)
	}
	environment := strings.Join(command.Env, "\n")
	for _, expected := range []string{"CTX_ADAPTER_API=1", "CTX_ADAPTER_NAME=echo", "CTX_ADAPTER_COMMAND=echoctl", "CTX_ADAPTER_VALUE_ECHO=staging"} {
		if !strings.Contains(environment, expected) {
			t.Fatalf("missing %s", expected)
		}
	}
}

func TestRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require elevated Windows privileges")
	}
	directory := fixtureAdapter(t, t.TempDir(), "echo", "selector")
	if err := os.Symlink("adapter.toml", filepath.Join(directory, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDirectory(directory); err == nil {
		t.Fatal("adapter containing a symlink was accepted")
	}
}
