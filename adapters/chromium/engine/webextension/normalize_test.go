package webextension

import (
	"encoding/json"
	"strings"
	"testing"
)

func exportFixture() map[string]any {
	return map[string]any{
		"format": "application-defined/v1", "source": "browser-api",
		"selection": map[string]any{"browser": "chrome", "profile": "Default", "storeId": "0"},
		"sites":     []string{"https://example.test/account"}, "names": []string{"session"}, "partition": "unpartitioned",
		"cookies": []any{map[string]any{"name": "session", "value": "synthetic-secret", "domain": ".example.test", "path": "/", "secure": true, "httpOnly": true, "hostOnly": false, "session": true, "sameSite": "no_restriction", "storeId": "0"}},
	}
}
func fixtureJSON(t *testing.T, data any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
func row(data map[string]any) map[string]any { return data["cookies"].([]any)[0].(map[string]any) }

func TestNormalizeBrowserExportScopes(t *testing.T) {
	for _, family := range []string{"chrome", "firefox", "safari"} {
		t.Run(family, func(t *testing.T) {
			data := exportFixture()
			data["selection"].(map[string]any)["browser"] = family
			policy := Policy{Browser: family, Namespace: family, Partition: family != "safari", FirstPartyDomain: family == "firefox"}
			store := "0"
			if family == "firefox" {
				store = "firefox-container-7"
				data["selection"].(map[string]any)["storeId"] = store
				row(data)["storeId"] = store
				data["firstPartyDomain"] = "example.test"
				row(data)["firstPartyDomain"] = "example.test"
			}
			if policy.Partition {
				data["partition"] = map[string]any{"topLevelSite": "https://top.test"}
				row(data)["partitionKey"] = map[string]any{"topLevelSite": "https://top.test", "hasCrossSiteAncestor": true, "futureField": "opaque"}
			}
			row(data)["futureScope"] = map[string]any{"isolated": true}
			result, err := Normalize(policy, "Default", store, fixtureJSON(t, data))
			if err != nil || len(result.Cookies) != 1 || result.StoreID != store {
				t.Fatalf("normalization failed: %v", err)
			}
			cookie := result.Cookies[0]
			if cookie.Domain != ".example.test" || cookie.Expiry != 0 || cookie.SameSitePolicy != "none" || !cookie.HTTPOnly || cookie.Attributes[family+".webextension_store_id"] != store || cookie.Attributes[family+".webextension_extra.futureScope"] != `{"isolated":true}` {
				t.Fatal("portable fields or native metadata lost")
			}
			if policy.Partition && (cookie.PartitionKey != "https://top.test" || !cookie.CrossSiteAncestor || !strings.Contains(cookie.Attributes[family+".webextension_partition_key"], "futureField")) {
				t.Fatal("partition metadata lost")
			}
			if family == "firefox" && cookie.Attributes[family+".webextension_first_party_domain"] != "example.test" {
				t.Fatal("first-party isolation lost")
			}
		})
	}
}

func TestNormalizeExpiryHostOnlyAndEmptyExport(t *testing.T) {
	policy := Policy{Browser: "chrome", Namespace: "chromium", Partition: true}
	for _, expiry := range []float64{-10, 0, 0.5, 100.9} {
		data := exportFixture()
		row(data)["session"] = false
		row(data)["expirationDate"] = expiry
		row(data)["hostOnly"] = true
		result, err := Normalize(policy, "Default", "0", fixtureJSON(t, data))
		if err != nil {
			t.Fatal(err)
		}
		want := int64(-1)
		if expiry >= 1 {
			want = 100
		}
		if result.Cookies[0].Expiry != want || result.Cookies[0].Domain != "example.test" {
			t.Fatal("expiry or host-only scope changed")
		}
	}
	data := exportFixture()
	data["cookies"] = []any{}
	result, err := Normalize(policy, "Default", "0", fixtureJSON(t, data))
	if err != nil || result.Cookies == nil || len(result.Cookies) != 0 {
		t.Fatal("empty export failed")
	}
}

func TestNormalizeRejectsMismatchedOrMalformedExport(t *testing.T) {
	tests := map[string]func(map[string]any){
		"browser":              func(d map[string]any) { d["selection"].(map[string]any)["browser"] = "edge" },
		"profile":              func(d map[string]any) { d["selection"].(map[string]any)["profile"] = "Other" },
		"store":                func(d map[string]any) { d["selection"].(map[string]any)["storeId"] = "1" },
		"row store":            func(d map[string]any) { row(d)["storeId"] = "1" },
		"name":                 func(d map[string]any) { row(d)["name"] = "csrf" },
		"site":                 func(d map[string]any) { row(d)["domain"] = "other.test" },
		"boolean":              func(d map[string]any) { row(d)["secure"] = nil },
		"session expiry":       func(d map[string]any) { row(d)["expirationDate"] = 100 },
		"persistent no expiry": func(d map[string]any) { row(d)["session"] = false },
		"overflow expiry":      func(d map[string]any) { row(d)["session"] = false; row(d)["expirationDate"] = 1e30 },
		"same site":            func(d map[string]any) { row(d)["sameSite"] = "unknown" },
		"scope mismatch":       func(d map[string]any) { row(d)["partitionKey"] = map[string]any{"topLevelSite": "https://top.test"} },
		"opaque partition":     func(d map[string]any) { row(d)["partitionKey"] = map[string]any{"futureScope": true} },
		"partition URL":        func(d map[string]any) { d["partition"] = map[string]any{"topLevelSite": "https://top.test/path"} },
		"partition ancestry": func(d map[string]any) {
			d["partition"] = map[string]any{"topLevelSite": "https://top.test"}
			row(d)["partitionKey"] = map[string]any{"topLevelSite": "https://top.test", "hasCrossSiteAncestor": nil}
		},
		"first party":      func(d map[string]any) { row(d)["firstPartyDomain"] = "example.test" },
		"unsupported FPI":  func(d map[string]any) { d["firstPartyDomain"] = "" },
		"missing array":    func(d map[string]any) { delete(d, "cookies") },
		"null array":       func(d map[string]any) { d["cookies"] = nil },
		"too many sites":   func(d map[string]any) { d["sites"] = make([]string, 17) },
		"missing names":    func(d map[string]any) { delete(d, "names") },
		"metadata limit":   func(d map[string]any) { row(d)["extra"] = strings.Repeat("x", 1025) },
		"metadata control": func(d map[string]any) { row(d)["extra\x00scope"] = true },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			data := exportFixture()
			mutate(data)
			result, err := Normalize(Policy{Browser: "chrome", Namespace: "chromium", Partition: true}, "Default", "0", fixtureJSON(t, data))
			if err == nil || result.Cookies != nil || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("invalid export accepted, partial output returned, or secret disclosed")
			}
		})
	}
	for _, data := range []json.RawMessage{[]byte(`[]`), []byte(`{} trailing`), []byte("\xff"), []byte(strings.Repeat("x", (8<<20)+1))} {
		if _, err := Normalize(Policy{Browser: "chrome", Namespace: "chromium"}, "Default", "0", data); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	data := exportFixture()
	data["selection"].(map[string]any)["browser"] = "safari"
	data["partition"] = map[string]any{"topLevelSite": "https://top.test"}
	if _, err := Normalize(Policy{Browser: "safari", Namespace: "safari"}, "Default", "0", fixtureJSON(t, data)); err == nil {
		t.Fatal("unsupported Safari partition accepted")
	}
}
