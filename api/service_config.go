package api

import (
	"errors"
	"fmt"
)

// ErrUnknownService indicates that the requested service is not in the registry.
var ErrUnknownService = errors.New("unknown service")

// DefaultVersion is the fallback app version when no override is configured.
const DefaultVersion = "5.0.999.999"

// ServiceConfig holds per-service API configuration.
type ServiceConfig struct {
	Name     string // service name: "account", "drive", "lumo"
	Host     string // API base URL: "https://account-api.proton.me/api"
	ClientID string // app identifier: "web-account", "web-drive", "web-lumo"
	Version  string // default version number for this service
}

// AppVersion returns the x-pm-appversion header value for this service.
// Format: <clientID>@<version>+proton-cli
func (sc ServiceConfig) AppVersion(version string) string {
	if version == "" {
		version = sc.Version
	}
	return sc.ClientID + "@" + version + "+proton-cli"
}

// Services is the global service registry.
var Services = map[string]ServiceConfig{
	"account": {Name: "account", Host: "https://account.proton.me/api", ClientID: "web-account", Version: "5.2.0"},
	"drive":   {Name: "drive", Host: "https://drive-api.proton.me/api", ClientID: "web-drive", Version: "5.2.0"},
	"lumo":    {Name: "lumo", Host: "https://lumo.proton.me/api", ClientID: "web-lumo", Version: "1.3.3.4"},
}

// LookupService returns the ServiceConfig for the given name, or
// ErrUnknownService if the service is not registered.
func LookupService(name string) (ServiceConfig, error) {
	svc, ok := Services[name]
	if !ok {
		return ServiceConfig{}, fmt.Errorf("%w: %q", ErrUnknownService, name)
	}
	return svc, nil
}
