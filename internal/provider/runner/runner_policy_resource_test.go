// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1/v1connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type runnerServiceClient struct {
	v1connect.RunnerServiceClient
	listRunnerPolicies func(context.Context, *connect.Request[v1.ListRunnerPoliciesRequest]) (*connect.Response[v1.ListRunnerPoliciesResponse], error)
}

func (c runnerServiceClient) ListRunnerPolicies(ctx context.Context, req *connect.Request[v1.ListRunnerPoliciesRequest]) (*connect.Response[v1.ListRunnerPoliciesResponse], error) {
	return c.listRunnerPolicies(ctx, req)
}

func TestParseRunnerPolicyImportID(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Parts []string
		Err   string
	}

	tests := []struct {
		Name     string
		ID       string
		Expected Expectation
	}{
		{
			Name: "valid_runner_and_group",
			ID:   "runner-1/group-1",
			Expected: Expectation{
				Parts: []string{"runner-1", "group-1"},
			},
		},
		{
			Name: "missing_group",
			ID:   "runner-1",
			Expected: Expectation{
				Err: "Invalid Import ID",
			},
		},
		{
			Name: "empty_runner",
			ID:   "/group-1",
			Expected: Expectation{
				Err: "Invalid Import ID",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			parts, diags := parseRunnerPolicyImportID(tc.ID)
			if diags.HasError() {
				got.Err = diags.Errors()[0].Summary()
			} else {
				got.Parts = parts
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("parseRunnerPolicyImportID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateRunnerPolicyRole(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	tests := []struct {
		Name     string
		Role     string
		Expected Expectation
	}{
		{
			Name: "user_is_supported",
			Role: "user",
		},
		{
			Name: "admin_is_rejected_until_backend_create_supports_it",
			Role: "admin",
			Expected: Expectation{
				Err: "Unsupported Runner Policy Role",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			var diags diag.Diagnostics
			validateRunnerPolicyRole(types.StringValue(tc.Role), &diags)
			if diags.HasError() {
				got.Err = diags.Errors()[0].Summary()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateRunnerPolicyRole() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPolicyResourceFindRunnerPolicyPagesUntilMatch(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		GroupID string
		Role    v1.RunnerRole
		Tokens  []string
		Err     string
	}

	ctx := t.Context()
	tokens := []string{}

	resource := &PolicyResource{
		client: managementclient.NewWithServices(managementclient.Services{
			RunnerService: runnerServiceClient{
				listRunnerPolicies: func(_ context.Context, req *connect.Request[v1.ListRunnerPoliciesRequest]) (*connect.Response[v1.ListRunnerPoliciesResponse], error) {
					token := req.Msg.GetPagination().GetToken()
					tokens = append(tokens, token)
					if req.Msg.GetRunnerId() != "runner-1" {
						return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("runner_id = %q", req.Msg.GetRunnerId()))
					}
					if token == "" {
						return connect.NewResponse(&v1.ListRunnerPoliciesResponse{
							Pagination: &v1.PaginationResponse{NextToken: "next"},
							Policies: []*v1.RunnerPolicy{
								{GroupId: "group-other", Role: v1.RunnerRole_RUNNER_ROLE_USER},
							},
						}), nil
					}
					return connect.NewResponse(&v1.ListRunnerPoliciesResponse{
						Pagination: &v1.PaginationResponse{},
						Policies: []*v1.RunnerPolicy{
							{GroupId: "group-1", Role: v1.RunnerRole_RUNNER_ROLE_USER},
						},
					}), nil
				},
			},
		}),
	}

	var got Expectation
	policy, err := resource.findRunnerPolicy(ctx, "runner-1", "group-1")
	if err != nil {
		got.Err = err.Error()
	} else {
		got.GroupID = policy.GetGroupId()
		got.Role = policy.GetRole()
		got.Tokens = tokens
	}

	expected := Expectation{
		GroupID: "group-1",
		Role:    v1.RunnerRole_RUNNER_ROLE_USER,
		Tokens:  []string{"", "next"},
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("findRunnerPolicy() mismatch (-want +got):\n%s", diff)
	}
}
