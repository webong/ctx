package store

import "testing"

func TestCredentialManagerItemReference(t *testing.T) {
	value, err := parseItem("target=ctx%2Fwork")
	if err != nil || value != "ctx/work" {
		t.Fatalf("target=%q err=%v", value, err)
	}
	for _, item := range []string{"", "target=", "target=one&target=two", "target=a%00b", "target=a&extra=b"} {
		if _, err := parseItem(item); err == nil {
			t.Fatalf("accepted invalid item %q", item)
		}
	}
}
