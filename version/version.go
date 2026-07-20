// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

// Package version exposes release metadata embedded into provider binaries.
package version

import "strings"

const ProviderName = "terraform-provider-ona"

// ProviderVersion is set during release builds to the provider release version.
var ProviderVersion = "dev"

// UserAgent returns the default provider API user-agent.
func UserAgent() string {
	return UserAgentFor(ProviderVersion)
}

// UserAgentFor returns the provider API user-agent for a specific version.
func UserAgentFor(providerVersion string) string {
	providerVersion = strings.TrimSpace(providerVersion)
	if providerVersion == "" {
		providerVersion = "dev"
	}
	return ProviderName + "/" + providerVersion
}
