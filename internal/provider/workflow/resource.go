// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithValidateConfig = &Resource{}

func NewResource() resource.Resource { return &Resource{} }

type Resource struct {
	client *managementclient.ManagementPlane
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation"
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateModel(ctx, data, false, &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_automation") {
		return
	}
	createRequest, diags := createWorkflowRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.WorkflowService().CreateWorkflow(ctx, connect.NewRequest(createRequest))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Workflow", "creating the Ona workflow", err)
		return
	}
	workflow := result.Msg.GetWorkflow()
	if workflow.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Workflow", "The Ona API returned a created workflow without an ID.")
		return
	}

	planned := data
	data.ID = types.StringValue(workflow.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !workflowUsesCodex(workflow) {
		resp.Diagnostics.AddError("Unable to Create Codex Automation", "The Ona API created the automation without the required Codex agent. The workflow ID was saved in Terraform state so it can be inspected or removed safely.")
		return
	}
	populateModel(ctx, &data, workflow, &resp.Diagnostics)
	preservePlannedInputs(ctx, &data, planned, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() || !planned.Disabled.ValueBool() {
		return
	}

	disabled := true
	updated, err := r.client.WorkflowService().UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{WorkflowId: workflow.GetId(), Disabled: &disabled}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Disable Ona Workflow", "disabling the newly created Ona workflow", err)
		return
	}
	if !workflowUsesCodex(updated.Msg.GetWorkflow()) {
		resp.Diagnostics.AddError("Unable to Disable Codex Automation", "The Ona API returned the newly created automation without the required Codex agent after disabling it.")
		return
	}
	populateModel(ctx, &data, updated.Msg.GetWorkflow(), &resp.Diagnostics)
	preservePlannedInputs(ctx, &data, planned, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_automation") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Workflow", "Workflow ID is empty.")
		return
	}
	workflow, err := r.getWorkflow(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Workflow", "reading the Ona workflow", err)
		return
	}
	if workflow == nil || workflow.GetSpec().GetDeleting() {
		resp.State.RemoveResource(ctx)
		return
	}
	prior := data
	data = Model{}
	populateModel(ctx, &data, workflow, &resp.Diagnostics)
	preserveTerraformOnlyState(ctx, &data, prior, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", "ona_automation") {
		return
	}
	current, err := r.getWorkflow(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Workflow", "checking whether the Ona workflow uses Codex", err)
		return
	}
	if current == nil {
		resp.Diagnostics.AddError("Unable to Update Ona Workflow", "The Ona workflow no longer exists. Refresh the Terraform state and try again.")
		return
	}
	if !workflowUsesCodex(current) {
		resp.Diagnostics.AddError("Unable to Update Non-Codex Automation", "Terraform can read and delete this automation, but cannot update it because it is not pinned to the Codex agent. Migrate the automation to Codex outside Terraform or replace it before applying changes.")
		return
	}
	updateRequest, diags := updateWorkflowRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.WorkflowService().UpdateWorkflow(ctx, connect.NewRequest(updateRequest))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Workflow", "updating the Ona workflow", err)
		return
	}
	if !workflowUsesCodex(result.Msg.GetWorkflow()) {
		resp.Diagnostics.AddError("Unable to Update Codex Automation", "The Ona API returned the updated automation without the required Codex agent. Refresh the Terraform state before retrying.")
		return
	}
	planned := data
	populateModel(ctx, &data, result.Msg.GetWorkflow(), &resp.Diagnostics)
	preservePlannedInputs(ctx, &data, planned, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func workflowUsesCodex(workflow *v1.Workflow) bool {
	return workflow != nil && workflow.GetSpec() != nil && workflow.GetSpec().GetAgentId() == codexAgentID
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_automation") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	_, err := r.client.WorkflowService().DeleteWorkflow(ctx, connect.NewRequest(&v1.DeleteWorkflowRequest{WorkflowId: data.ID.ValueString(), Force: false}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Workflow", "deleting the Ona workflow", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *Resource) getWorkflow(ctx context.Context, id string) (*v1.Workflow, error) {
	result, err := r.client.WorkflowService().GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{WorkflowId: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	workflow := result.Msg.GetWorkflow()
	if workflow == nil {
		return nil, fmt.Errorf("get workflow: the Ona API returned an empty workflow")
	}
	return workflow, nil
}
