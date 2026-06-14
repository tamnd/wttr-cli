package wttr

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// The client's HTTP behaviour is covered in wttr_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "wttr" {
		t.Errorf("Scheme = %q, want wttr", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "wttr" {
		t.Errorf("Identity.Binary = %q, want wttr", info.Identity.Binary)
	}
}

func TestClassify_city(t *testing.T) {
	typ, id, err := Domain{}.Classify("Tokyo")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if typ != "city" {
		t.Errorf("type = %q, want city", typ)
	}
	if id != "Tokyo" {
		t.Errorf("id = %q, want Tokyo", id)
	}
}

func TestClassify_url(t *testing.T) {
	typ, id, err := Domain{}.Classify("https://wttr.in/London")
	if err != nil {
		t.Fatalf("Classify url: %v", err)
	}
	if typ != "city" {
		t.Errorf("type = %q, want city", typ)
	}
	if id != "London" {
		t.Errorf("id = %q, want London", id)
	}
}

func TestClassify_empty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("expected error on empty input, got nil")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("city", "Paris")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got == "" {
		t.Error("Locate returned empty URL")
	}
}

func TestLocate_badType(t *testing.T) {
	_, err := Domain{}.Locate("page", "foo")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
