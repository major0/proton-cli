package api

import (
	"errors"
	"testing"

	"pgregory.net/rapid"
)

// TestAppVersionFormat_Property verifies that for any valid (clientID, version)
// pair, AppVersion produces "<clientID>@<version>".
func TestAppVersionFormat_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		clientID := rapid.StringMatching(`[a-z]{2,8}(-[a-z]{2,8})?`).Draw(t, "clientID")
		version := rapid.StringMatching(`[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`).Draw(t, "version")

		sc := ServiceConfig{ClientID: clientID, Version: version}
		got := sc.AppVersion(version)

		want := clientID + "@" + version + ""
		if got != want {
			t.Fatalf("AppVersion(%q) = %q, want %q", version, got, want)
		}
	})
}

// TestAppVersionDefaultVersion verifies that AppVersion("") uses the
// service's default Version field.
func TestAppVersionDefaultVersion(t *testing.T) {
	sc := ServiceConfig{ClientID: "web-lumo", Version: "1.3.3.4"}
	got := sc.AppVersion("")
	want := "web-lumo@1.3.3.4"
	if got != want {
		t.Fatalf("AppVersion(\"\") = %q, want %q", got, want)
	}
}

// TestServicesRegistry verifies that the default registry has the expected
// entries with correct hosts, client IDs, and versions.
func TestServicesRegistry(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		clientID   string
		version    string
		cookieAuth bool
	}{
		{"account", "https://account.proton.me/api", "web-account", "5.0.367.1", true},
		{"drive", "https://drive-api.proton.me/api", "web-drive", "5.2.0", false},
		{"lumo", "https://lumo.proton.me/api", "web-lumo", "1.3.3.4", true},
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
			if svc.Version != tt.version {
				t.Errorf("Version = %q, want %q", svc.Version, tt.version)
			}
			if svc.CookieAuth != tt.cookieAuth {
				t.Errorf("CookieAuth = %v, want %v", svc.CookieAuth, tt.cookieAuth)
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
	if svc.Host != "https://account.proton.me/api" {
		t.Errorf("Host = %q, want %q", svc.Host, "https://account.proton.me/api")
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

// TestLookupServiceByHost_AllRegistered verifies that LookupServiceByHost
// resolves every registered service by its hostname.
func TestLookupServiceByHost_AllRegistered(t *testing.T) {
	tests := []struct {
		host     string
		wantName string
	}{
		{"account.proton.me", "account"},
		{"drive-api.proton.me", "drive"},
		{"lumo.proton.me", "lumo"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			svc, err := LookupServiceByHost(tt.host)
			if err != nil {
				t.Fatalf("LookupServiceByHost(%q): %v", tt.host, err)
			}
			if svc.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", svc.Name, tt.wantName)
			}
		})
	}
}

// TestLookupServiceByHost_Unknown verifies that LookupServiceByHost returns
// ErrUnknownService for unregistered hostnames.
func TestLookupServiceByHost_Unknown(t *testing.T) {
	unknowns := []string{
		"unknown.proton.me",
		"example.com",
		"",
	}

	for _, host := range unknowns {
		t.Run(host, func(t *testing.T) {
			_, err := LookupServiceByHost(host)
			if err == nil {
				t.Fatalf("expected error for host %q", host)
			}
			if !errors.Is(err, ErrUnknownService) {
				t.Fatalf("expected ErrUnknownService, got: %v", err)
			}
		})
	}
}
