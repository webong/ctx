package adapterkit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/webong/ctx/internal/app/browser/management"
)

type fakeManagement struct{ called bool }

func (fake *fakeManagement) ManageBrowser(_ context.Context, profile string, request management.Request) (management.Response, error) {
	fake.called = true
	if profile != "Profile 1" {
		return management.Response{}, context.Canceled
	}
	return management.Response{Version: management.Version, Kind: request.Kind, Action: request.Action, Status: "prepared", Result: json.RawMessage(`{"revision":"sha256:test"}`)}, nil
}

func TestRunManagement(t *testing.T) {
	fake := &fakeManagement{}
	var stdout, stderr strings.Builder
	input := `{"version":"1.0","kind":"extension","action":"prepare","input":{"source":"addon.zip"}}`
	if code := RunManagement(context.Background(), "Profile 1", strings.NewReader(input), &stdout, &stderr, fake); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !fake.called || !strings.Contains(stdout.String(), `"status":"prepared"`) {
		t.Fatalf("called=%v output=%s", fake.called, stdout.String())
	}
}

func TestRunManagementRejectsBadRequestBeforeBackend(t *testing.T) {
	fake := &fakeManagement{}
	var stdout, stderr strings.Builder
	if code := RunManagement(context.Background(), "Profile 1", strings.NewReader(`{"version":"1.0","kind":"extension","action":"inject"}`), &stdout, &stderr, fake); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if fake.called {
		t.Fatal("backend called for invalid request")
	}
}
