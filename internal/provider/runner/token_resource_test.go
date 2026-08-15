// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type runnerTokenServiceClient struct {
	v1connect.RunnerServiceClient
	createRunnerToken func(context.Context, *connect.Request[v1.CreateRunnerTokenRequest]) (*connect.Response[v1.CreateRunnerTokenResponse], error)
}

func (c runnerTokenServiceClient) CreateRunnerToken(ctx context.Context, req *connect.Request[v1.CreateRunnerTokenRequest]) (*connect.Response[v1.CreateRunnerTokenResponse], error) {
	return c.createRunnerToken(ctx, req)
}

func TestTokenResourceCreate(t *testing.T) {
	t.Parallel()

	type Input struct {
		Response *v1.CreateRunnerTokenResponse
		Err      error
	}
	type Expectation struct {
		RunnerIDs    []string
		IDSet        bool
		TokenVersion string
		Token        string
		ErrSummary   string
		ErrDetail    string
	}
	tests := []struct {
		Name     string
		Input    Input
		Expected Expectation
	}{
		{
			Name: "success",
			Input: Input{
				Response: &v1.CreateRunnerTokenResponse{ExchangeToken: "exchange-token"},
			},
			Expected: Expectation{
				RunnerIDs:    []string{"runner-1"},
				IDSet:        true,
				TokenVersion: "v1",
				Token:        "exchange-token",
			},
		},
		{
			Name: "api_error",
			Input: Input{
				Err: connect.NewError(connect.CodeInternal, errors.New("create token failed")),
			},
			Expected: Expectation{
				RunnerIDs:  []string{"runner-1"},
				ErrSummary: "Unable to Create Ona Runner Token",
				ErrDetail:  "Ona could not complete the request while creating an Ona runner registration token.\n\nAPI error: internal: create token failed",
			},
		},
		{
			Name: "empty_response",
			Input: Input{
				Response: &v1.CreateRunnerTokenResponse{},
			},
			Expected: Expectation{
				RunnerIDs:  []string{"runner-1"},
				ErrSummary: "Unable to Create Ona Runner Token",
				ErrDetail:  "The Ona API returned an empty runner registration token.",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			var got Expectation
			tokenResource := &TokenResource{
				client: managementclient.NewWithServices(managementclient.Services{
					RunnerService: runnerTokenServiceClient{
						createRunnerToken: func(_ context.Context, req *connect.Request[v1.CreateRunnerTokenRequest]) (*connect.Response[v1.CreateRunnerTokenResponse], error) {
							got.RunnerIDs = append(got.RunnerIDs, req.Msg.GetRunnerId())
							if tc.Input.Err != nil {
								return nil, tc.Input.Err
							}
							return connect.NewResponse(tc.Input.Response), nil
						},
					},
				}),
			}

			var schemaResp resource.SchemaResponse
			tokenResource.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			plan := tfsdk.Plan{Schema: schemaResp.Schema}
			planDiags := plan.Set(ctx, TokenModel{
				ID:           types.StringUnknown(),
				RunnerID:     types.StringValue("runner-1"),
				TokenVersion: types.StringValue("v1"),
				Token:        types.StringUnknown(),
			})
			if planDiags.HasError() {
				t.Fatalf("plan.Set() diagnostics: %v", planDiags)
			}

			resp := resource.CreateResponse{
				State: tfsdk.State{Schema: schemaResp.Schema, Raw: plan.Raw},
			}
			tokenResource.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)

			if diags := resp.Diagnostics.Errors(); len(diags) > 0 {
				got.ErrSummary = diags[0].Summary()
				got.ErrDetail = diags[0].Detail()
			} else {
				var state TokenModel
				stateDiags := resp.State.Get(ctx, &state)
				if stateDiags.HasError() {
					t.Fatalf("state.Get() diagnostics: %v", stateDiags)
				}
				got.IDSet = !state.ID.IsNull() && !state.ID.IsUnknown() && state.ID.ValueString() != ""
				got.TokenVersion = state.TokenVersion.ValueString()
				got.Token = state.Token.ValueString()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("TokenResource.Create() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
