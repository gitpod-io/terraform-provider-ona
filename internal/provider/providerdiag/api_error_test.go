// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package providerdiag

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
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

func TestAPIErrorDetailOmitRawError(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		SecretPresent        bool
		CodePresent          bool
		HintPresent          bool
		HiddenErrorReference bool
	}
	secretErr := errors.New("private_key=private-must-not-appear client_secret=client-must-not-appear webhook_secret=webhook-must-not-appear")
	tests := []struct {
		Name     string
		Err      error
		Code     string
		Hint     string
		Expected Expectation
	}{
		{Name: "unauthenticated", Err: connect.NewError(connect.CodeUnauthenticated, secretErr), Code: "unauthenticated", Hint: "`ONA_TOKEN`", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "permission_denied", Err: connect.NewError(connect.CodePermissionDenied, secretErr), Code: "permission_denied", Hint: "does not have permission", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "not_found", Err: connect.NewError(connect.CodeNotFound, secretErr), Code: "not_found", Hint: "Verify the configured ID", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "invalid_argument", Err: connect.NewError(connect.CodeInvalidArgument, secretErr), Code: "invalid_argument", Hint: "Check the Terraform configuration", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "failed_precondition", Err: connect.NewError(connect.CodeFailedPrecondition, secretErr), Code: "failed_precondition", Hint: "configuration and prerequisites", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "rate_limited", Err: connect.NewError(connect.CodeResourceExhausted, secretErr), Code: "resource_exhausted", Hint: "Wait and rerun Terraform", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "unavailable", Err: connect.NewError(connect.CodeUnavailable, secretErr), Code: "unavailable", Hint: "Rerun Terraform after the service recovers", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "deadline_exceeded", Err: connect.NewError(connect.CodeDeadlineExceeded, secretErr), Code: "deadline_exceeded", Hint: "request timed out", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "wrapped_error", Err: fmt.Errorf("request with outer-must-not-appear: %w", connect.NewError(connect.CodeInvalidArgument, secretErr)), Code: "invalid_argument", Hint: "Check the Terraform configuration", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "non_connect_error", Err: secretErr, Code: "unknown", Hint: "could not complete the request", Expected: Expectation{CodePresent: true, HintPresent: true}},
		{Name: "nil_error", Hint: "Ona returned an empty error.", Expected: Expectation{HintPresent: true}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			detail := APIErrorDetail("creating a test resource", tc.Err, OmitRawError)
			got := Expectation{
				SecretPresent:        strings.Contains(detail, "must-not-appear"),
				CodePresent:          tc.Code != "" && strings.Contains(detail, "API error code: "+tc.Code),
				HintPresent:          strings.Contains(detail, tc.Hint),
				HiddenErrorReference: strings.Contains(detail, "in the API error"),
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("APIErrorDetail() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
