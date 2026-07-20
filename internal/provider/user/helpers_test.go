// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
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

			api := managementclient.NewWithServices(managementclient.Services{
				IdentityService: fakeIdentityService{response: tc.Response, err: tc.Err},
			})
			holder := clientHolder{client: api}

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

type fakeIdentityService struct {
	response *connect.Response[v1.GetAuthenticatedIdentityResponse]
	err      error
}

func (f fakeIdentityService) GetIDToken(context.Context, *connect.Request[v1.GetIDTokenRequest]) (*connect.Response[v1.GetIDTokenResponse], error) {
	return nil, errors.New("unexpected GetIDToken call")
}

func (f fakeIdentityService) GetAuthenticatedIdentity(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return f.response, f.err
}

func (f fakeIdentityService) ExchangeToken(context.Context, *connect.Request[v1.ExchangeTokenRequest]) (*connect.Response[v1.ExchangeTokenResponse], error) {
	return nil, errors.New("unexpected ExchangeToken call")
}
