package provider

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
)

// minServerVersion is the oldest Netmaker server version this provider
// supports. Raise it if a resource/data source starts depending on a
// server API introduced later (e.g. netmaker_tag's /api/v1/tags, or
// netmaker_node.is_ingress_gateway's /api/nodes/{network}/{nodeid}/gateway).
const minServerVersion = "v1.7.0"

// checkServerVersion returns an error if serverVersion is older than
// minServerVersion. A "dev" version (unreleased/local server builds)
// always passes — matching Netmaker's own compatibility check
// (logic.IsVersionCompatible) — since a dev build's actual capabilities
// can't be determined from its version string alone.
func checkServerVersion(serverVersion string) error {
	if serverVersion == "dev" {
		return nil
	}

	// Netmaker's own version strings are e.g. "v1.7.0"; strip any
	// non-numeric prefix the same way logic.IsVersionCompatible does,
	// rather than assuming a specific prefix.
	trimmed := strings.TrimFunc(serverVersion, func(r rune) bool {
		return !unicode.IsNumber(r)
	})
	got, err := version.NewVersion(trimmed)
	if err != nil {
		return fmt.Errorf("could not parse server version %q: %w", serverVersion, err)
	}

	constraint, err := version.NewConstraint(">= " + strings.TrimPrefix(minServerVersion, "v"))
	if err != nil {
		return err // unreachable: minServerVersion is a constant
	}
	if !constraint.Check(got) {
		return fmt.Errorf("server is running %s, but this provider requires %s or newer", serverVersion, minServerVersion)
	}
	return nil
}
