package app

import (
	"bytes"
	"reflect"
	"testing"

	adapterpkg "github.com/webong/ctx/internal/adapter"
)

func TestParseAdapterSelectionNamesAndNumbers(t *testing.T) {
	available := []*adapterpkg.Adapter{
		{Manifest: adapterpkg.Manifest{Name: "docker"}},
		{Manifest: adapterpkg.Manifest{Name: "kube"}},
		{Manifest: adapterpkg.Manifest{Name: "firefox"}},
	}
	got, err := parseAdapterSelection("2,docker,2", available)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"kube", "docker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %#v, want %#v", got, want)
	}
}

func TestSetupMinimalDoesNotRequireCatalog(t *testing.T) {
	t.Setenv("CTX_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := setupCommand([]string{"--minimal"}, bytes.NewReader(nil), &stdout, &stderr); code != 0 {
		t.Fatalf("setup minimal returned %d: %s", code, stderr.String())
	}
	if got := stdout.String(); got != "No new adapters selected. Existing adapters were preserved.\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestParseAdapterSelectionRejectsUnknown(t *testing.T) {
	available := []*adapterpkg.Adapter{{Manifest: adapterpkg.Manifest{Name: "docker"}}}
	if _, err := parseAdapterSelection("unknown", available); err == nil {
		t.Fatal("unknown adapter was accepted")
	}
}
