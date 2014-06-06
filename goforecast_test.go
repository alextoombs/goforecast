package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestSetupCliApp(t *testing.T) {
	app := setupCliApp()
	if app == nil {
		t.Fatalf("App should not be nil")
	}
	if app.Name != "goforecast" {
		t.Fatalf("Expected app name of \"goforecast\", got: %s", app.Name)
	}
	if len(app.Commands) < 1 {
		t.Fatalf("Expected app to have 1+ commands defined, got: %d", len(app.Commands))
	}
}

func TestBuildGeocodingURL(t *testing.T) {
	u, err := buildGeocodingURL("http", nil)
	if err != nil {
		t.Fatalf("Expected no error, got: %s", err)
	}
	if u == nil {
		t.Fatal("URL should never be nil without error")
	}

	if !strings.Contains(u.String(), geocodeHost) {
		t.Fatalf("Expected url (%s) to contain geocode host (%s)", u.String(), geocodeHost)
	}
	if !strings.Contains(u.String(), geocodePath) {
		t.Fatalf("Expected url (%s) to contain geocode path (%s)", u.String(), geocodePath)
	}

	vals := map[string][]string{
		"foo": []string{"bar", "baz"},
	}
	u, err = buildGeocodingURL("http", url.Values(vals))
	if err != nil {
		t.Fatalf("Expected no error, got: %s", err)
	}
	if u == nil {
		t.Fatal("URL should never be nil without error")
	}
}

func TestParseGeocodingAddr(t *testing.T) {
	zip := "94109"
	vals := parseGeocodingAddr(zip)
	if addr, ok := vals["address"]; !ok || len(addr) != 1 || addr[0] != zip {
		t.Fatalf("Expected address of %s, got %v", zip, addr)
	}
}
