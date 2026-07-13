// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
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

			ctrl := gomock.NewController(t)
			mockClient := managementclient.NewMock(ctrl)
			mockClient.IdentityService.EXPECT().
				GetAuthenticatedIdentity(gomock.Any(), gomock.Any()).
				Return(connect.NewResponse(tc.IdentityResponse), nil)

			var got Expectation
			organizationID, err := (&clientHolder{client: mockClient.Client()}).authenticatedOrganizationID(t.Context())
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
