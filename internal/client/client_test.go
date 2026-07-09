package client

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAPIBaseURL(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result string
		Err    string
	}

	tests := []struct {
		Name     string
		Host     string
		Expected Expectation
	}{
		{
			Name: "default_host",
			Expected: Expectation{
				Result: "https://app.gitpod.io/api",
			},
		},
		{
			Name: "schemeless_host_gets_https",
			Host: "example.com",
			Expected: Expectation{
				Result: "https://example.com/api",
			},
		},
		{
			Name: "trims_existing_path_slash",
			Host: "https://example.com/",
			Expected: Expectation{
				Result: "https://example.com/api",
			},
		},
		{
			Name: "preserves_base_path",
			Host: "https://example.com/control-plane/",
			Expected: Expectation{
				Result: "https://example.com/control-plane/api",
			},
		},
		{
			Name: "rejects_unsupported_scheme",
			Host: "ftp://example.com",
			Expected: Expectation{
				Err: `unsupported Ona host scheme "ftp"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, err := APIBaseURL(tc.Host)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("APIBaseURL() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveHost(t *testing.T) {
	// not parallel: uses process environment variables.

	type Expectation struct {
		Result string
	}

	tests := []struct {
		Name       string
		Configured string
		OnaHost    string
		Expected   Expectation
	}{
		{
			Name:       "configured_takes_precedence",
			Configured: "https://configured.example.com",
			OnaHost:    "https://ona.example.com",
			Expected: Expectation{
				Result: "https://configured.example.com",
			},
		},
		{
			Name:    "ona_env_takes_precedence",
			OnaHost: "https://ona.example.com",
			Expected: Expectation{
				Result: "https://ona.example.com",
			},
		},
		{
			Name: "default_fallback",
			Expected: Expectation{
				Result: DefaultHost,
			},
		},
		{
			Name:    "default_when_only_unrelated_env_is_set",
			OnaHost: "",
			Expected: Expectation{
				Result: DefaultHost,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			// not parallel: uses process environment variables.
			t.Setenv("ONA_HOST", tc.OnaHost)
			t.Setenv("OTHER_HOST", "https://other.example.com")

			got := Expectation{
				Result: resolveHost(tc.Configured),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("resolveHost() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	// not parallel: uses process environment variables.

	type Expectation struct {
		Result string
	}

	tests := []struct {
		Name       string
		Configured string
		OnaToken   string
		Expected   Expectation
	}{
		{
			Name:       "configured_takes_precedence",
			Configured: "configured-token",
			OnaToken:   "ona-token",
			Expected: Expectation{
				Result: "configured-token",
			},
		},
		{
			Name:     "ona_env_takes_precedence",
			OnaToken: "ona-token",
			Expected: Expectation{
				Result: "ona-token",
			},
		},
		{
			Name: "empty_when_unset",
			Expected: Expectation{
				Result: "",
			},
		},
		{
			Name: "empty_when_only_unrelated_env_is_set",
			Expected: Expectation{
				Result: "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			// not parallel: uses process environment variables.
			t.Setenv("ONA_TOKEN", tc.OnaToken)
			t.Setenv("OTHER_TOKEN", "other-token")

			got := Expectation{
				Result: resolveToken(tc.Configured),
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("resolveToken() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
