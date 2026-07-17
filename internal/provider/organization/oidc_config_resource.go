// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &OIDCConfigResource{}
var _ resource.ResourceWithConfigure = &OIDCConfigResource{}
var _ resource.ResourceWithIdentity = &OIDCConfigResource{}
var _ resource.ResourceWithImportState = &OIDCConfigResource{}
var _ resource.ResourceWithValidateConfig = &OIDCConfigResource{}

func NewOIDCConfigResource() resource.Resource {
	return &OIDCConfigResource{}
}

type OIDCConfigResource struct {
	clientHolder
}

type OIDCConfigModel struct {
	ID                types.String `tfsdk:"id"`
	CustomClaimFields types.Set    `tfsdk:"custom_claim_fields"`
}

func (r *OIDCConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_config"
}

func (r *OIDCConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyStringSet, diags := types.SetValue(types.StringType, nil)
	resp.Diagnostics.Append(diags...)
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Singleton Ona OIDC V3 token configuration for the organization associated with the configured provider token. Destroying this resource removes Terraform state only; it does not reset the remote organization setting.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform resource ID. This is the authenticated organization ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"custom_claim_fields": resourceschema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(emptyStringSet),
				MarkdownDescription: "Additional fields included in the OIDC V3 `sub` claim. Supported values are `creator_id`, `creator_principal`, `creator_email`, `creator_name`, `creator_idp`, `account_id`, `user_id`, `organization_id`, `project_id`, `runner_id`, `environment_id`, `email`, `name`, `idp`, `runner_name`, `service_account_id`, `environment_initializers.git.remote_uri`, `environment_initializers.git.upstream_remote_uri`, and `environment_initializers.context_url`. A field is included only for principal types that provide it.",
			},
		},
	}
}

func (r *OIDCConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *OIDCConfigResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data OIDCConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateOIDCConfigModel(ctx, data, &resp.Diagnostics)
}

func (r *OIDCConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OIDCConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", "ona_oidc_config") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	oidcConfig, diags := oidcConfigFromModel(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.OrganizationService().UpdateOIDCConfig(ctx, connect.NewRequest(&v1.UpdateOIDCConfigRequest{
		OrganizationId: organizationID,
		OidcConfig:     oidcConfig,
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona OIDC Config", "updating the Ona OIDC config", err)
		return
	}
	data.ID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, OIDCConfigIdentityModel{OrganizationID: data.ID})...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	planned := data
	populateOIDCConfigModel(ctx, &data, organizationID, result.Msg.GetOidcConfig(), planned, &resp.Diagnostics)
	preserveOIDCConfigPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OIDCConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OIDCConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", "ona_oidc_config") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	if !validateOIDCOrganizationScope(&resp.Diagnostics, data.ID, organizationID) {
		return
	}
	result, err := r.client.OrganizationService().GetOIDCConfig(ctx, connect.NewRequest(&v1.GetOIDCConfigRequest{
		OrganizationId: organizationID,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona OIDC Config", "reading the Ona OIDC config", err)
		return
	}
	prior := data
	data = OIDCConfigModel{}
	populateOIDCConfigModel(ctx, &data, organizationID, result.Msg.GetOidcConfig(), prior, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, OIDCConfigIdentityModel{OrganizationID: data.ID})...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OIDCConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OIDCConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior OIDCConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating", "ona_oidc_config") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	if !validateOIDCOrganizationScope(&resp.Diagnostics, prior.ID, organizationID) {
		return
	}
	oidcConfig, diags := oidcConfigFromModel(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.OrganizationService().UpdateOIDCConfig(ctx, connect.NewRequest(&v1.UpdateOIDCConfigRequest{
		OrganizationId: organizationID,
		OidcConfig:     oidcConfig,
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona OIDC Config", "updating the Ona OIDC config", err)
		return
	}
	planned := data
	populateOIDCConfigModel(ctx, &data, organizationID, result.Msg.GetOidcConfig(), planned, &resp.Diagnostics)
	preserveOIDCConfigPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, OIDCConfigIdentityModel{OrganizationID: data.ID})...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OIDCConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}

func (r *OIDCConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("organization_id"), req, resp)
		return
	}
	if !r.requireClient(&resp.Diagnostics, "importing", "ona_oidc_config") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	if req.ID != "current" && req.ID != organizationID {
		resp.Diagnostics.AddError(
			"Invalid Ona OIDC Config Import ID",
			fmt.Sprintf("Import ona_oidc_config with \"current\" or the authenticated organization ID %q.", organizationID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(organizationID))...)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, OIDCConfigIdentityModel{OrganizationID: types.StringValue(organizationID)})...)
}

func validateOIDCOrganizationScope(diags *diag.Diagnostics, stateID types.String, organizationID string) bool {
	if stateID.IsNull() || stateID.IsUnknown() || stateID.ValueString() == "" || stateID.ValueString() == organizationID {
		return true
	}
	diags.AddError(
		"Ona OIDC Config Organization Mismatch",
		fmt.Sprintf("This ona_oidc_config state belongs to organization %q, but the configured provider token is authenticated for organization %q.", stateID.ValueString(), organizationID),
	)
	return false
}
