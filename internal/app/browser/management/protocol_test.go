package management

import (
	"encoding/json"
	"testing"
)

func TestNewRequestAndAllowedOperations(t *testing.T) {
	request, err := NewRequest("extension", "prepare", map[string]string{"source": "/tmp/addon.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRequest(request); err != nil {
		t.Fatal(err)
	}
	if request.Version != Version {
		t.Fatalf("version = %q", request.Version)
	}
	if !contains(Operations()["userscript"], "activate") || !contains(Operations()["bookmarklet"], "decode") {
		t.Fatal("expected userscript and bookmarklet operations")
	}
}

func TestValidateRequestRejectsInvalidOperationsAndNonObjects(t *testing.T) {
	for _, request := range []Request{
		{Version: Version, Kind: "extension", Action: "inject"},
		{Version: Version, Kind: "../extension", Action: "prepare"},
		{Version: Version, Kind: "extension", Action: "prepare", Input: json.RawMessage(`[]`)},
	} {
		if err := ValidateRequest(request); err == nil {
			t.Fatalf("accepted %#v", request)
		}
	}
}

func TestValidateResponseKeepsInstallAndActivationSeparate(t *testing.T) {
	activate, _ := NewRequest("extension", "activate", nil)
	if err := ValidateResponse(Response{Version: Version, Kind: activate.Kind, Action: activate.Action, Status: "installed"}, activate); err == nil {
		t.Fatal("activation reported a persistent install")
	}
	install, _ := NewRequest("extension", "install", nil)
	if err := ValidateResponse(Response{Version: Version, Kind: install.Kind, Action: install.Action, Status: "activated"}, install); err == nil {
		t.Fatal("installation reported a session activation")
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
