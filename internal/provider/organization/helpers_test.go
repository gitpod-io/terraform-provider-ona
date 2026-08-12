// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
)

func TestAuthenticatedOrganizationForClient(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Organization authenticatedOrganization
		Err          string
		Calls        int
	}
	tests := []struct {
		Name     string
		Response *connect.Response[v1.GetAuthenticatedIdentityResponse]
		Err      error
		Expected Expectation
	}{
		{
			Name: "returns_organization_and_principal",
			Response: connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{
				OrganizationId: "organization-id",
				Subject:        &v1.Subject{Principal: v1.Principal_PRINCIPAL_USER},
			}),
			Expected: Expectation{
				Organization: authenticatedOrganization{ID: "organization-id", Principal: v1.Principal_PRINCIPAL_USER},
				Calls:        1,
			},
		},
		{
			Name:     "propagates_api_error",
			Err:      connect.NewError(connect.CodeUnauthenticated, errors.New("bad token")),
			Expected: Expectation{Err: "get authenticated identity: unauthenticated: bad token", Calls: 1},
		},
		{
			Name:     "rejects_missing_organization_id",
			Response: connect.NewResponse(&v1.GetAuthenticatedIdentityResponse{}),
			Expected: Expectation{Err: "authenticated identity did not include an organization ID", Calls: 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			service := &organizationIdentityService{response: tc.Response, err: tc.Err}
			client := managementclient.NewWithServices(managementclient.Services{IdentityService: service})
			var got Expectation
			organization, err := authenticatedOrganizationForClient(t.Context(), client)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Organization = organization
			}
			got.Calls = service.calls
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("authenticatedOrganizationForClient() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type organizationIdentityService struct {
	v1connect.IdentityServiceClient
	response *connect.Response[v1.GetAuthenticatedIdentityResponse]
	err      error
	calls    int
}

func (s *organizationIdentityService) GetAuthenticatedIdentity(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	s.calls++
	return s.response, s.err
}
