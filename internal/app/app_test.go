package app

import (
	"bytes"
	"strings"
	"testing"
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
