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
}

// AppVersion returns the x-pm-appversion header value for this service.
// Format: <clientID>@<version>+proton-cli
func (sc ServiceConfig) AppVersion(version string) string {
	return sc.ClientID + "@" + version + "+proton-cli"
}

// Services is the global service registry.
var Services = map[string]ServiceConfig{
	"account": {Name: "account", Host: "https://account-api.proton.me/api", ClientID: "web-account"},
	"drive":   {Name: "drive", Host: "https://drive-api.proton.me/api", ClientID: "web-drive"},
	"lumo":    {Name: "lumo", Host: "https://lumo.proton.me/api", ClientID: "web-lumo"},
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
