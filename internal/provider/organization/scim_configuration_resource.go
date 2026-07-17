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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SCIMConfigurationResource{}
var _ resource.ResourceWithConfigure = &SCIMConfigurationResource{}
var _ resource.ResourceWithIdentity = &SCIMConfigurationResource{}
var _ resource.ResourceWithImportState = &SCIMConfigurationResource{}
var _ resource.ResourceWithValidateConfig = &SCIMConfigurationResource{}

func NewSCIMConfigurationResource() resource.Resource {
	return &SCIMConfigurationResource{}
}

type SCIMConfigurationResource struct {
	clientHolder
}

type SCIMConfigurationModel struct {
	ID                                 types.String `tfsdk:"id"`
	SSOConfigurationID                 types.String `tfsdk:"sso_configuration_id"`
	Name                               types.String `tfsdk:"name"`
	Enabled                            types.Bool   `tfsdk:"enabled"`
	AllowUnverifiedEmailAccountLinking types.Bool   `tfsdk:"allow_unverified_email_account_linking"`
	TokenExpiresIn                     types.String `tfsdk:"token_expires_in"`
	TokenExpiresAt                     types.String `tfsdk:"token_expires_at"`
	CreatedAt                          types.String `tfsdk:"created_at"`
}

func (r *SCIMConfigurationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim_configuration"
}

func (r *SCIMConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona SCIM provisioning configuration for the organization associated with the configured provider token. This resource does not expose the one-time SCIM bearer token in Terraform state.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SCIM configuration ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sso_configuration_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SSO configuration ID linked to SCIM provisioning.",
			},
			"name": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(defaultSCIMName),
				MarkdownDescription: "Human-readable SCIM configuration name.",
			},
			"enabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether SCIM provisioning is active.",
			},
			"allow_unverified_email_account_linking": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether SCIM may link provisioned users to existing accounts when the identity provider does not mark email addresses as verified.",
			},
			"token_expires_in": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Initial SCIM token lifetime using a number followed by a time unit, such as `24h` or `8760h`. Minimum is `24h`; maximum is `17520h`. Changing this value replaces the resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token_expires_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the current SCIM token expires.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the SCIM configuration was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SCIMConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *SCIMConfigurationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	validateSCIMConfigurationConfig(ctx, req.Config, &resp.Diagnostics)
}

func (r *SCIMConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SCIMConfigurationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", "ona_scim_configuration") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	createReq, diags := createSCIMConfigurationRequest(ctx, organizationID, data, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.OrganizationService().CreateSCIMConfiguration(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona SCIM Configuration", "creating the Ona SCIM configuration", err)
		return
	}
	scim := result.Msg.GetScimConfiguration()
	if scim == nil || scim.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona SCIM Configuration", "The Ona API returned an empty SCIM configuration.")
		return
	}

	data.ID = types.StringValue(scim.GetId())
	resp.Diagnostics.Append(resp.Identity.Set(ctx, SCIMConfigurationIdentityModel{ID: data.ID})...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if isKnownBool(data.Enabled) && !data.Enabled.ValueBool() {
		enabled := false
		updateResult, err := r.client.OrganizationService().UpdateSCIMConfiguration(ctx, connect.NewRequest(&v1.UpdateSCIMConfigurationRequest{
			ScimConfigurationId: data.ID.ValueString(),
			Enabled:             &enabled,
		}))
		if err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Disable Created Ona SCIM Configuration", "updating the created Ona SCIM configuration", err)
			return
		}
		scim = updateResult.Msg.GetScimConfiguration()
	}
	if scim == nil {
		scim, err = r.getSCIMConfiguration(ctx, data.ID.ValueString())
		if err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona SCIM Configuration", "reading the created Ona SCIM configuration", err)
			return
		}
	}
	if scim == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if !validateSCIMOrganizationScope(&resp.Diagnostics, scim, organizationID) {
		return
	}
	planned := data
	populateSCIMConfigurationModel(&data, scim, planned)
	preserveSCIMConfigurationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCIMConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SCIMConfigurationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", "ona_scim_configuration") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	scim, err := r.getSCIMConfiguration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona SCIM Configuration", "reading the Ona SCIM configuration", err)
		return
	}
	if scim == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if !validateSCIMOrganizationScope(&resp.Diagnostics, scim, organizationID) {
		return
	}
	prior := data
	data = SCIMConfigurationModel{}
	populateSCIMConfigurationModel(&data, scim, prior)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, SCIMConfigurationIdentityModel{ID: data.ID})...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCIMConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SCIMConfigurationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating", "ona_scim_configuration") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	updateReq, diags := updateSCIMConfigurationRequestFromConfig(ctx, data, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.OrganizationService().UpdateSCIMConfiguration(ctx, connect.NewRequest(updateReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona SCIM Configuration", "updating the Ona SCIM configuration", err)
		return
	}
	scim := result.Msg.GetScimConfiguration()
	if scim == nil {
		scim, err = r.getSCIMConfiguration(ctx, data.ID.ValueString())
		if err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona SCIM Configuration", "reading the updated Ona SCIM configuration", err)
			return
		}
	}
	if scim == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if !validateSCIMOrganizationScope(&resp.Diagnostics, scim, organizationID) {
		return
	}
	planned := data
	populateSCIMConfigurationModel(&data, scim, planned)
	preserveSCIMConfigurationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, SCIMConfigurationIdentityModel{ID: data.ID})...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCIMConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SCIMConfigurationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", "ona_scim_configuration") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	_, err := r.client.OrganizationService().DeleteSCIMConfiguration(ctx, connect.NewRequest(&v1.DeleteSCIMConfigurationRequest{
		ScimConfigurationId: data.ID.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona SCIM Configuration", "deleting the Ona SCIM configuration", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *SCIMConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}

func (r *SCIMConfigurationResource) getSCIMConfiguration(ctx context.Context, id string) (*v1.SCIMConfiguration, error) {
	if id == "" {
		return nil, fmt.Errorf("SCIM configuration ID is empty")
	}
	result, err := r.client.OrganizationService().GetSCIMConfiguration(ctx, connect.NewRequest(&v1.GetSCIMConfigurationRequest{
		ScimConfigurationId: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get SCIM configuration: %w", err)
	}
	return result.Msg.GetScimConfiguration(), nil
}

func validateSCIMOrganizationScope(diags *diag.Diagnostics, scim *v1.SCIMConfiguration, organizationID string) bool {
	if scim.GetOrganizationId() == "" || scim.GetOrganizationId() == organizationID {
		return true
	}
	diags.AddError(
		"Ona SCIM Configuration Organization Mismatch",
		fmt.Sprintf("The SCIM configuration %q belongs to organization %q, but the configured provider token is authenticated for organization %q.", scim.GetId(), scim.GetOrganizationId(), organizationID),
	)
	return false
}
