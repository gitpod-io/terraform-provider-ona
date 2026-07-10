// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package providerdiag

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestAPIErrorDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		contains []string
	}{
		{
			name: "unauthenticated",
			err:  connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token")),
			contains: []string{
				"Ona rejected the API token",
				"`ONA_TOKEN`",
				"API error:",
				"invalid token",
			},
		},
		{
			name: "permission denied",
			err:  connect.NewError(connect.CodePermissionDenied, errors.New("not allowed")),
			contains: []string{
				"does not have permission",
				"organization and resource",
				"not allowed",
			},
		},
		{
			name: "invalid argument",
			err:  connect.NewError(connect.CodeInvalidArgument, errors.New("name too short")),
			contains: []string{
				"one or more Terraform arguments are invalid",
				"name too short",
			},
		},
		{
			name: "failed precondition",
			err:  connect.NewError(connect.CodeFailedPrecondition, errors.New("runner does not have a public key")),
			contains: []string{
				"remote resource is not in the required state",
				"runner does not have a public key",
			},
		},
		{
			name: "non connect",
			err:  errors.New("dial failed"),
			contains: []string{
				"Ona could not complete the request",
				"dial failed",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			detail := APIErrorDetail("creating a test resource", tc.err)
			for _, expected := range tc.contains {
				if !strings.Contains(detail, expected) {
					t.Fatalf("APIErrorDetail() = %q, want substring %q", detail, expected)
				}
			}
		})
	}
}
