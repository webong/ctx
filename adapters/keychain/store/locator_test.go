package store

import "testing"

func TestKeychainItemReference(t *testing.T) {
	service, account, err := parseItem("service=Chrome+Safe+Storage&account=Chrome")
	if err != nil || service != "Chrome Safe Storage" || account != "Chrome" {
		t.Fatalf("service=%q account=%q err=%v", service, account, err)
	}
	for _, item := range []string{"", "service=one", "service=one&account=", "service=a%00b&account=c", "service=a&account=b&extra=c"} {
		if _, _, err := parseItem(item); err == nil {
			t.Fatalf("accepted invalid item %q", item)
		}
	}
}
