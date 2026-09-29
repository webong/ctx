package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webong/ctx/graph/system"
)

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), Version) {
		t.Fatalf("version output %q does not contain %q", stdout.String(), Version)
	}
}

func TestMissingRunCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"run"}, &stdout, &stderr); code != 2 {
		t.Fatalf("got exit code %d", code)
	}
}

func TestCTXRecordsSystemContextAndGraphCommandReadsIt(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "ctx-state")
	t.Setenv("CTX_HOME", stateDir)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"resolve", "browser"}, &stdout, &stderr); code != 0 {
		t.Fatalf("resolve failed: %d %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"graph", "vertices", "shell-session"}, &stdout, &stderr); code != 0 {
		t.Fatalf("graph query failed: %d %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ctx.system/shell-session") {
		t.Fatalf("shell facts missing from graph query: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(stateDir, "graph.json")); err != nil {
		t.Fatalf("persistent CTX graph missing: %v", err)
	}
	store, err := systemgraph.Open(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snapshot, err := store.Store.Snapshot(context.Background(), systemgraph.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Vertices) < 4 {
		t.Fatalf("expected system facts, got %d vertices", len(snapshot.Vertices))
	}
}

func TestShellHookProducesShortLivedObserver(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"hook", "zsh"}, &stdout, &stderr); code != 0 {
		t.Fatalf("hook command failed: %d %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "__observe-shell") || !strings.Contains(stdout.String(), "precmd") {
		t.Fatalf("unexpected shell hook: %s", stdout.String())
	}
}
