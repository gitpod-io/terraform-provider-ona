// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithIdentity = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithValidateConfig = &Resource{}
var _ resource.ResourceWithModifyPlan = &Resource{}

type Resource struct {
	client     *managementclient.ManagementPlane
	deployment string
}

func NewResource() resource.Resource { return &Resource{} }

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_github_app_integration"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if data, ok := req.ProviderData.(*providerdata.Data); ok && data != nil {
		r.deployment = data.APIBaseURL
	}
}

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateCredentials(ctx, config, false, true, &resp.Diagnostics)
}

func (r *Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan, config Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mutation := req.State.Raw.IsNull()
	if !mutation {
		var state Model
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
		mutation = !plan.CredentialsVersion.IsUnknown() && !plan.CredentialsVersion.Equal(state.CredentialsVersion)
	}
	if mutation {
		if plan.CredentialsVersion.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("credentials_version"), "Missing GitHub App Credentials Version", "Set a positive credentials_version on creation and credential rotation. Keep the existing marker when credentials do not change.")
			return
		}
		validateCredentials(ctx, config, true, true, &resp.Diagnostics)
	}
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	auth := mutationAuth(ctx, plan, config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_github_app_integration") {
		return
	}
	definition, err := r.githubDefinition(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve GitHub App Definition", err.Error())
		return
	}
	result, err := r.client.IntegrationService().CreateIntegration(ctx, connect.NewRequest(&v1.CreateIntegrationRequest{
		IntegrationDefinitionId: definition.GetId(), Enabled: plan.Enabled.ValueBool(), Auth: auth,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create GitHub App Integration", safeAPIError("creating the GitHub App integration", err).Error())
		return
	}
	if result == nil || result.Msg == nil || result.Msg.GetIntegration().GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create GitHub App Integration", "Ona returned no integration ID after creation.")
		return
	}
	remote := result.Msg.GetIntegration()
	plan.ID = types.StringValue(remote.GetId())
	plan.Credentials = types.ObjectNull(credentialsTypes)
	plan.AppSlug = types.StringNull()
	plan.ClientID = types.StringNull()
	plan.Setup = types.ObjectNull(setupTypes)
	plan.Installation = types.ObjectNull(installationTypes)
	// Save the remote identity before metadata validation can fail; a retry must retain ownership of the created integration.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{IntegrationID: plan.ID})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(populateModel(&plan, remote, definition, r.deployment)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_github_app_integration") {
		return
	}
	remote, err := r.getIntegration(ctx, data.ID.ValueString())
	if connect.CodeOf(err) == connect.CodeNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read GitHub App Integration", err.Error())
		return
	}
	definition, err := r.githubDefinition(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve GitHub App Definition", err.Error())
		return
	}
	resp.Diagnostics.Append(populateModel(&data, remote, definition, r.deployment)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{IntegrationID: data.ID})...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "updating", "ona_github_app_integration") {
		return
	}
	if !plan.AppID.Equal(state.AppID) {
		resp.Diagnostics.AddError("GitHub App Replacement Required", "Changing app_id requires replacing the integration.")
		return
	}
	update := &v1.UpdateIntegrationRequest{Id: state.ID.ValueString()}
	if !plan.Enabled.Equal(state.Enabled) {
		enabled := plan.Enabled.ValueBool()
		update.Enabled = &enabled
	}
	if !plan.CredentialsVersion.Equal(state.CredentialsVersion) {
		update.Auth = mutationAuth(ctx, plan, config, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	definition, err := r.githubDefinition(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve GitHub App Definition", err.Error())
		return
	}
	result, err := r.client.IntegrationService().UpdateIntegration(ctx, connect.NewRequest(update))
	if err != nil {
		resp.Diagnostics.AddError("Unable to Update GitHub App Integration", safeAPIError("updating the GitHub App integration", err).Error())
		return
	}
	if result == nil || result.Msg == nil || result.Msg.GetIntegration().GetId() != state.ID.ValueString() {
		resp.Diagnostics.AddError("Unable to Update GitHub App Integration", "Ona returned an empty or mismatched integration after the update.")
		return
	}
	resp.Diagnostics.Append(populateModel(&plan, result.Msg.GetIntegration(), definition, r.deployment)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{IntegrationID: plan.ID})...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_github_app_integration") {
		return
	}
	_, err := r.client.IntegrationService().DeleteIntegration(ctx, connect.NewRequest(&v1.DeleteIntegrationRequest{Id: state.ID.ValueString()}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		resp.Diagnostics.AddError("Unable to Delete GitHub App Integration", safeAPIError("deleting the Ona integration", err).Error())
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *Resource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"integration_id": identityschema.StringAttribute{RequiredForImport: true, Description: "Ona integration UUID."},
	}}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if req.Identity != nil && !req.Identity.Raw.IsNull() {
		var identity IdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		id = identity.IntegrationID.ValueString()
	}
	if _, err := uuid.Parse(id); err != nil {
		resp.Diagnostics.AddError("Invalid GitHub App Integration Import ID", "Import requires an Ona integration UUID, either as the string ID or as integration_id in a structured identity.")
		return
	}
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("integration_id"), req, resp)
}

func (r *Resource) getIntegration(ctx context.Context, id string) (*v1.Integration, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, fmt.Errorf("the integration ID must be a valid UUID")
	}
	result, err := r.client.IntegrationService().GetIntegration(ctx, connect.NewRequest(&v1.GetIntegrationRequest{Id: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("the requested integration was not found"))
		}
		return nil, safeAPIError("reading the GitHub App integration", err)
	}
	if result == nil || result.Msg == nil || result.Msg.GetIntegration().GetId() != id {
		return nil, fmt.Errorf("the Ona API returned an empty or mismatched integration")
	}
	return result.Msg.GetIntegration(), nil
}
