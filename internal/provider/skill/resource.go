// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithIdentity = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}

func NewResource() resource.Resource {
	return &Resource{}
}

type Resource struct {
	client *managementclient.ManagementPlane
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_skill") {
		return
	}

	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_skill", err)
		return
	}
	createReq, diags := createPromptRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.AgentService().CreatePrompt(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Skill", "creating the Ona skill", err)
		return
	}
	if result == nil || result.Msg == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Skill", "The Ona API returned an empty response after creation.")
		return
	}
	remote := result.Msg.GetPrompt()
	if remote == nil || remote.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Skill", "The Ona API returned an empty skill or skill ID after creation.")
		return
	}

	data.ID = types.StringValue(remote.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{SkillID: data.ID})...)
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mapped, mappingDiags := modelFromPrompt(remote, data.ID.ValueString(), organizationID, true)
	resp.Diagnostics.Append(mappingDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &mapped)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_skill") {
		return
	}

	id := data.ID.ValueString()
	if _, err := uuid.Parse(id); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("id"), "Invalid Skill ID", "The skill state does not contain a valid UUID.")
		return
	}
	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_skill", err)
		return
	}
	resp.Diagnostics.Append(verifyPrivateOrganizationID(ctx, req.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	remote, err := r.getPrompt(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Skill", "reading the Ona skill", err)
		return
	}
	if remote == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	mapped, mappingDiags := modelFromPrompt(remote, id, organizationID, true)
	resp.Diagnostics.Append(mappingDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{SkillID: mapped.ID})...)
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &mapped)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", "ona_skill") {
		return
	}

	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_skill", err)
		return
	}
	resp.Diagnostics.Append(verifyPrivateOrganizationID(ctx, req.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateReq, diags := updatePromptRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.AgentService().UpdatePrompt(ctx, connect.NewRequest(updateReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Skill", "updating the Ona skill", err)
		return
	}
	if result == nil || result.Msg == nil {
		resp.Diagnostics.AddError("Unable to Update Ona Skill", "The Ona API returned an empty response after update.")
		return
	}
	remote := result.Msg.GetPrompt()
	mapped, mappingDiags := modelFromPrompt(remote, data.ID.ValueString(), organizationID, true)
	resp.Diagnostics.Append(mappingDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{SkillID: mapped.ID})...)
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &mapped)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_skill") {
		return
	}

	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_skill", err)
		return
	}
	resp.Diagnostics.Append(verifyPrivateOrganizationID(ctx, req.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := data.ID.ValueString()
	if _, err := uuid.Parse(id); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("id"), "Invalid Skill ID", "The skill state does not contain a valid UUID.")
		return
	}

	_, err = r.client.AgentService().DeletePrompt(ctx, connect.NewRequest(&v1.DeletePromptRequest{PromptId: id}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Skill", "deleting the Ona skill", err)
		return
	}
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, "")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var importID string
	if req.ID != "" {
		importID = req.ID
	} else if req.Identity != nil {
		var identityID types.String
		resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("skill_id"), &identityID)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !identityID.IsNull() && !identityID.IsUnknown() {
			importID = identityID.ValueString()
		}
	}
	if _, err := uuid.Parse(importID); err != nil {
		resp.Diagnostics.AddError("Invalid Ona Skill Import ID", fmt.Sprintf("Import ID %q must be a valid Prompt UUID.", importID))
		return
	}
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("skill_id"), req, resp)
}

func (r *Resource) getPrompt(ctx context.Context, id string) (*v1.Prompt, error) {
	result, err := r.client.AgentService().GetPrompt(ctx, connect.NewRequest(&v1.GetPromptRequest{PromptId: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get prompt: %w", err)
	}
	if result == nil || result.Msg == nil {
		return nil, fmt.Errorf("get prompt: Ona API returned an empty response")
	}
	return result.Msg.GetPrompt(), nil
}
