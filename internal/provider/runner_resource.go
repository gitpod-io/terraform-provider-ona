// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	onaclient "github.com/ona/terraform-provider-ona/internal/client"
)

var _ resource.Resource = &RunnerResource{}
var _ resource.ResourceWithConfigure = &RunnerResource{}
var _ resource.ResourceWithImportState = &RunnerResource{}

func NewRunnerResource() resource.Resource {
	return &RunnerResource{}
}

type RunnerResource struct {
	client *onaclient.Client
}

type RunnerResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (r *RunnerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (r *RunnerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Ona Runner. This beta resource is import/read-only for brownfield dogfood workflows.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner ID. Use this value as the Terraform import ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner display name.",
			},
		},
	}
}

func (r *RunnerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	api, ok := req.ProviderData.(*onaclient.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = api
}

func (r *RunnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError(
		"Runner Creation Is Not Supported Yet",
		"ona_runner is currently import/read-only for brownfield dogfood workflows. Import an existing runner instead.",
	)
}

func (r *RunnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RunnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_runner resources.",
		)
		return
	}

	id := data.ID.ValueString()
	runner, err := r.findRunner(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Runner", err.Error())
		return
	}
	if runner == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(runner.GetRunnerId())
	data.Name = types.StringValue(runner.GetName())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RunnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Runner Updates Are Not Supported Yet",
		"ona_runner is currently import/read-only for brownfield dogfood workflows.",
	)
}

func (r *RunnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Runner Removed From Terraform State Only",
		"ona_runner is currently import/read-only, so deleting it from Terraform removes state only and does not delete the remote Ona runner.",
	)
	resp.State.RemoveResource(ctx)
}

func (r *RunnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *RunnerResource) findRunner(ctx context.Context, id string) (*onaclient.Runner, error) {
	if id == "" {
		return nil, fmt.Errorf("runner ID is empty")
	}

	token := ""
	for {
		resp, err := r.client.ListRunners(ctx, onaclient.ListRunnersRequest{
			Pagination: &onaclient.PaginationRequest{
				PageSize: 100,
				Token:    token,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("list runners: %w", err)
		}

		for _, runner := range resp.Runners {
			if runner.GetRunnerId() == id {
				return runner, nil
			}
		}

		if resp.Pagination == nil || resp.Pagination.GetNextToken() == "" {
			return nil, nil
		}
		token = resp.Pagination.GetNextToken()
	}
}
