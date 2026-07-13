// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SSOConfigurationResource{}
var _ resource.ResourceWithConfigure = &SSOConfigurationResource{}
var _ resource.ResourceWithImportState = &SSOConfigurationResource{}
var _ resource.ResourceWithValidateConfig = &SSOConfigurationResource{}

func NewSSOConfigurationResource() resource.Resource {
	return &SSOConfigurationResource{}
}

type SSOConfigurationResource struct {
	clientHolder
}

type SSOConfigurationModel struct {
	ID                  types.String `tfsdk:"id"`
	ClientID            types.String `tfsdk:"client_id"`
	ClientSecret        types.String `tfsdk:"client_secret"`
	ClientSecretVersion types.String `tfsdk:"client_secret_version"`
	IssuerURL           types.String `tfsdk:"issuer_url"`
	DisplayName         types.String `tfsdk:"display_name"`
	EmailDomains        types.Set    `tfsdk:"email_domains"`
	AdditionalScopes    types.Set    `tfsdk:"additional_scopes"`
	ClaimsExpression    types.String `tfsdk:"claims_expression"`
	State               types.String `tfsdk:"state"`
	ProviderType        types.String `tfsdk:"provider_type"`
}

func (r *SSOConfigurationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso_configuration"
}

func (r *SSOConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Custom Ona OIDC SSO configuration for the organization associated with the configured provider token.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SSO configuration ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "OIDC application client ID from the identity provider.",
			},
			"client_secret": resourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
				MarkdownDescription: "OIDC application client secret. This write-only value is sent to Ona but not stored in Terraform plan or state.",
			},
			"client_secret_version": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User-managed version marker for resubmitting `client_secret` during rotation.",
			},
			"issuer_url": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "OIDC issuer URL for the identity provider.",
			},
			"display_name": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Display name shown for this SSO configuration. Maximum length is 128 characters.",
			},
			"email_domains": resourceschema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Email domains allowed to sign in through this SSO configuration. The Ona API cannot clear all domains through Terraform; omit this attribute to leave it unmanaged.",
			},
			"additional_scopes": resourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Additional OIDC scopes requested during sign-in. An explicitly empty set clears all additional scopes.",
			},
			"claims_expression": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional CEL expression evaluated against OIDC token claims during login. Set an empty string to clear the expression.",
			},
			"state": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(ssoStateActive),
				MarkdownDescription: "SSO configuration state. Supported values are `active` and `inactive`.",
			},
			"provider_type": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SSO provider type. Terraform-managed configurations are `custom`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SSOConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *SSOConfigurationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data SSOConfigurationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateSSOConfigurationModel(ctx, data, &resp.Diagnostics)
}

func (r *SSOConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SSOConfigurationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secret := readSSOClientSecret(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", "ona_sso_configuration") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}

	createReq, diags := createSSOConfigurationRequest(ctx, organizationID, data, secret)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.OrganizationService().CreateSSOConfiguration(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona SSO Configuration", "creating the Ona SSO configuration", err)
		return
	}
	sso := result.Msg.GetSsoConfiguration()
	if sso == nil || sso.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona SSO Configuration", "The Ona API returned an empty SSO configuration.")
		return
	}

	data.ID = types.StringValue(sso.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.State.ValueString() == ssoStateInactive {
		inactive := v1.SSOConfigurationState_SSO_CONFIGURATION_STATE_INACTIVE
		if _, err := r.client.OrganizationService().UpdateSSOConfiguration(ctx, connect.NewRequest(&v1.UpdateSSOConfigurationRequest{
			SsoConfigurationId: data.ID.ValueString(),
			State:              &inactive,
		})); err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Deactivate Created Ona SSO Configuration", "updating the created Ona SSO configuration state", err)
			return
		}
	}

	sso, err = r.getSSOConfiguration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona SSO Configuration", "reading the created Ona SSO configuration", err)
		return
	}
	if sso == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if !validateSSOOrganizationScope(&resp.Diagnostics, sso, organizationID) || !validateSSOProviderType(&resp.Diagnostics, sso) {
		return
	}
	planned := data
	populateSSOConfigurationModel(ctx, &data, sso, planned, &resp.Diagnostics)
	preserveSSOConfigurationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSOConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SSOConfigurationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", "ona_sso_configuration") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	sso, err := r.getSSOConfiguration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona SSO Configuration", "reading the Ona SSO configuration", err)
		return
	}
	if sso == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if !validateSSOOrganizationScope(&resp.Diagnostics, sso, organizationID) || !validateSSOProviderType(&resp.Diagnostics, sso) {
		return
	}
	prior := data
	data = SSOConfigurationModel{}
	populateSSOConfigurationModel(ctx, &data, sso, prior, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSOConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SSOConfigurationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior SSOConfigurationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secret := readSSOClientSecret(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating", "ona_sso_configuration") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	updateReq, diags := updateSSOConfigurationRequestFromConfig(ctx, data, prior, req.Config, secret)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.OrganizationService().UpdateSSOConfiguration(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona SSO Configuration", "updating the Ona SSO configuration", err)
		return
	}
	sso, err := r.getSSOConfiguration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona SSO Configuration", "reading the updated Ona SSO configuration", err)
		return
	}
	if sso == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	if !validateSSOOrganizationScope(&resp.Diagnostics, sso, organizationID) || !validateSSOProviderType(&resp.Diagnostics, sso) {
		return
	}
	planned := data
	populateSSOConfigurationModel(ctx, &data, sso, planned, &resp.Diagnostics)
	preserveSSOConfigurationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SSOConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SSOConfigurationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", "ona_sso_configuration") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	_, err := r.client.OrganizationService().DeleteSSOConfiguration(ctx, connect.NewRequest(&v1.DeleteSSOConfigurationRequest{
		SsoConfigurationId: data.ID.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona SSO Configuration", "deleting the Ona SSO configuration", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *SSOConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SSOConfigurationResource) getSSOConfiguration(ctx context.Context, id string) (*v1.SSOConfiguration, error) {
	if id == "" {
		return nil, fmt.Errorf("sso configuration ID is empty")
	}
	result, err := r.client.OrganizationService().GetSSOConfiguration(ctx, connect.NewRequest(&v1.GetSSOConfigurationRequest{
		SsoConfigurationId: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get SSO configuration: %w", err)
	}
	return result.Msg.GetSsoConfiguration(), nil
}

func validateSSOOrganizationScope(diags *diag.Diagnostics, sso *v1.SSOConfiguration, organizationID string) bool {
	if sso.GetOrganizationId() == "" || sso.GetOrganizationId() == organizationID {
		return true
	}
	diags.AddError(
		"Ona SSO Configuration Organization Mismatch",
		fmt.Sprintf("The SSO configuration %q belongs to organization %q, but the configured provider token is authenticated for organization %q.", sso.GetId(), sso.GetOrganizationId(), organizationID),
	)
	return false
}

func validateSSOProviderType(diags *diag.Diagnostics, sso *v1.SSOConfiguration) bool {
	if sso.GetProviderType() == v1.SSOConfiguration_PROVIDER_TYPE_CUSTOM {
		return true
	}
	diags.AddError(
		"Unsupported Ona SSO Configuration Provider Type",
		fmt.Sprintf("The SSO configuration %q has provider type %q. ona_sso_configuration manages only custom SSO configurations.", sso.GetId(), ssoProviderTypeToString(sso.GetProviderType())),
	)
	return false
}
