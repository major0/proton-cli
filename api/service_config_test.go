package api

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

// TestAppVersionFormat_Property verifies that for any valid (clientID, version)
// pair, AppVersion produces "<clientID>@<version>+proton-cli".
//
// **Validates: Requirements 1.5**
// Tag: Feature: session-fork, Property 1: App version format
func TestAppVersionFormat_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clientID := rapid.StringMatching(`[a-z]{2,8}(-[a-z]{2,8})?`).Draw(t, "clientID")
		version := rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`).Draw(t, "version")

		sc := ServiceConfig{ClientID: clientID}
		got := sc.AppVersion(version)

		want := clientID + "@" + version + "+proton-cli"
		if got != want {
			t.Fatalf("AppVersion(%q) = %q, want %q", version, got, want)
		}
	})
}

// TestServicesRegistry verifies that the default registry has the expected
// entries with correct hosts and client IDs.
func TestServicesRegistry(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		clientID string
	}{
		{"account", "https://account-api.proton.me/api", "web-account"},
		{"drive", "https://drive-api.proton.me/api", "web-drive"},
		{"lumo", "https://lumo.proton.me/api", "web-lumo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, ok := Services[tt.name]
			if !ok {
				t.Fatalf("service %q not in registry", tt.name)
			}
			if svc.Name != tt.name {
				t.Errorf("Name = %q, want %q", svc.Name, tt.name)
			}
			if svc.Host != tt.host {
				t.Errorf("Host = %q, want %q", svc.Host, tt.host)
			}
			if svc.ClientID != tt.clientID {
				t.Errorf("ClientID = %q, want %q", svc.ClientID, tt.clientID)
			}
		})
	}
}

// TestLookupService_Found verifies that LookupService returns the correct
// config for a known service.
func TestLookupService_Found(t *testing.T) {
	svc, err := LookupService("account")
	if err != nil {
		t.Fatalf("LookupService(account): %v", err)
	}
	if svc.Name != "account" {
		t.Errorf("Name = %q, want %q", svc.Name, "account")
	}
	if svc.Host != "https://account-api.proton.me/api" {
		t.Errorf("Host = %q, want %q", svc.Host, "https://account-api.proton.me/api")
	}
	if svc.ClientID != "web-account" {
		t.Errorf("ClientID = %q, want %q", svc.ClientID, "web-account")
	}
}

// TestLookupService_Unknown verifies that LookupService returns
// ErrUnknownService for an unregistered service name.
func TestLookupService_Unknown(t *testing.T) {
	_, err := LookupService("unknown")
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
	if !errors.Is(err, ErrUnknownService) {
		t.Fatalf("expected ErrUnknownService, got: %v", err)
	}
}
