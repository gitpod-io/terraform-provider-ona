// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package version

import "testing"

func TestUserAgent(t *testing.T) {
	previous := ProviderVersion
	ProviderVersion = "1.2.3-beta.4"
	t.Cleanup(func() {
		ProviderVersion = previous
	})

	if got, want := UserAgent(), "terraform-provider-ona/1.2.3-beta.4"; got != want {
		t.Fatalf("UserAgent() = %q, want %q", got, want)
	}
}

func TestUserAgentForDefaultsEmptyVersionToDev(t *testing.T) {
	if got, want := UserAgentFor(" "), "terraform-provider-ona/dev"; got != want {
		t.Fatalf("UserAgentFor() = %q, want %q", got, want)
	}
}
