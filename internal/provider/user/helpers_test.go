// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
)

func TestAuthenticatedOrganizationID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		OrganizationID string
		Err            string
	}
	tests := []struct {
		Name     string
		Response *connect.Response[v1.GetAuthenticatedIdentityResponse]
		Err      error
		Expected Expectation
	}{
		{
			Name:     "returns_organization_id",
			Response: connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{OrganizationId: testOrganizationID}),
			Expected: Expectation{OrganizationID: testOrganizationID},
		},
		{
			Name:     "propagates_api_error",
			Err:      connect.NewError(connect.CodeUnauthenticated, errors.New("bad token")),
			Expected: Expectation{Err: "get authenticated identity: unauthenticated: bad token"},
		},
		{
			Name:     "rejects_empty_response",
			Expected: Expectation{Err: "get authenticated identity: Ona returned an empty response"},
		},
		{
			Name:     "rejects_missing_organization_id",
			Response: connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{}),
			Expected: Expectation{Err: "authenticated identity did not include an organization ID"},
		},
		{
			Name:     "rejects_invalid_organization_id",
			Response: connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{OrganizationId: "invalid"}),
			Expected: Expectation{Err: `authenticated identity included invalid organization ID "invalid": invalid UUID length: 7`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			api := managementclient.NewMock(ctrl)
			api.IdentityService.EXPECT().GetAuthenticatedIdentity(gomock.Any(), gomock.Any()).Return(tc.Response, tc.Err)
			holder := clientHolder{client: api.Client()}

			var got Expectation
			organizationID, err := holder.authenticatedOrganizationID(t.Context())
			if err != nil {
				got.Err = err.Error()
			} else {
				got.OrganizationID = organizationID
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("authenticatedOrganizationID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
