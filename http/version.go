package http

import (
	"fmt"
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^v[1-9][0-9]*$`)

func ValidVersion(version string) bool { return versionPattern.MatchString(version) }

type versionRegistrar struct {
	parent          EndpointRegistrar
	version, prefix string
}

// WithVersion scopes adapters while preserving their shared application service.
func WithVersion(parent EndpointRegistrar, version, prefix string) (EndpointRegistrar, error) {
	if parent == nil || !ValidVersion(version) {
		return nil, fmt.Errorf("version must be v followed by a positive integer")
	}
	if prefix == "" {
		prefix = "/api/" + version
	}
	if !strings.HasPrefix(prefix, "/") || strings.HasSuffix(prefix, "/") || strings.ContainsAny(prefix, ":*?#{}") || strings.Contains(prefix, "//") {
		return nil, fmt.Errorf("invalid version prefix")
	}
	return versionRegistrar{parent: parent, version: version, prefix: prefix}, nil
}
func (v versionRegistrar) Endpoint(route Route) error {
	if route.Contract == nil {
		return fmt.Errorf("versioned endpoints require typed contracts")
	}
	contract := *route.Contract
	contract.Path = v.prefix + contract.Path
	contract.OperationID += "_" + v.version
	contract.Version = v.version
	route.Path = contract.Path
	route.Contract = &contract
	return v.parent.Endpoint(route)
}
