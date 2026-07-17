// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1/v1connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
)

type identityServiceClient struct {
	v1connect.IdentityServiceClient
	getAuthenticatedIdentity func(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error)
}

func (c identityServiceClient) GetAuthenticatedIdentity(ctx context.Context, req *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
	return c.getAuthenticatedIdentity(ctx, req)
}

func TestAuthenticatedOrganizationID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		OrganizationID string
		Err            string
	}

	tests := []struct {
		Name             string
		IdentityResponse *v1.GetAuthenticatedIdentityResponse
		Expected         Expectation
	}{
		{
			Name: "returns_organization_id",
			IdentityResponse: &v1.GetAuthenticatedIdentityResponse{
				OrganizationId: "org-1",
			},
			Expected: Expectation{
				OrganizationID: "org-1",
			},
		},
		{
			Name:             "rejects_identity_without_organization",
			IdentityResponse: &v1.GetAuthenticatedIdentityResponse{},
			Expected: Expectation{
				Err: "authenticated identity did not include an organization ID",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			client := managementclient.NewWithServices(managementclient.Services{
				IdentityService: identityServiceClient{
					getAuthenticatedIdentity: func(context.Context, *connect.Request[v1.GetAuthenticatedIdentityRequest]) (*connect.Response[v1.GetAuthenticatedIdentityResponse], error) {
						return connect.NewResponse(tc.IdentityResponse), nil
					},
				},
			})

			var got Expectation
			organizationID, err := (&clientHolder{client: client}).authenticatedOrganizationID(t.Context())
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
