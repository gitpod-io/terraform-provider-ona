// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const termsOfServiceResourceType = "ona_terms_of_service"

var _ resource.Resource = &TermsOfServiceResource{}
var _ resource.ResourceWithConfigure = &TermsOfServiceResource{}
var _ resource.ResourceWithImportState = &TermsOfServiceResource{}
var _ resource.ResourceWithValidateConfig = &TermsOfServiceResource{}

func NewTermsOfServiceResource() resource.Resource {
	return &TermsOfServiceResource{}
}

type TermsOfServiceResource struct {
	clientHolder
}

type TermsOfServiceModel struct {
	ID                            types.String `tfsdk:"id"`
	Enabled                       types.Bool   `tfsdk:"enabled"`
	Markdown                      types.String `tfsdk:"markdown"`
	CurrentVersionID              types.String `tfsdk:"current_version_id"`
	CurrentVersion                types.Int32  `tfsdk:"current_version"`
	CurrentVersionCreatedAt       types.String `tfsdk:"current_version_created_at"`
	CurrentVersionCreatedByUserID types.String `tfsdk:"current_version_created_by_user_id"`
}

func (r *TermsOfServiceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_terms_of_service"
}

func (r *TermsOfServiceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Singleton Ona organization Terms of Service configuration and current Markdown version. Organization scope is resolved from the configured provider token. Destroying this resource disables the Terms of Service requirement but does not delete immutable version history because the Ona API has no delete operation. Publishing or changing Markdown currently requires a user personal access token because Ona records the publishing user on each immutable version.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Authenticated organization ID for the managed Terms of Service singleton.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": resourceschema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether organization members must accept the current Terms of Service version.",
			},
			"markdown": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Desired Markdown body for the current Terms of Service version. When configured and changed, Ona publishes a new immutable version. When omitted, Terraform manages only `enabled` and preserves the API's current Markdown in state after read or import. Must be at most 32000 characters.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"current_version_id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the current immutable Terms of Service version, if one exists.",
				PlanModifiers: []planmodifier.String{
					useStateForUnknownUnlessMarkdownChangesString(),
				},
			},
			"current_version": resourceschema.Int32Attribute{
				Computed:            true,
				MarkdownDescription: "Current Terms of Service version number, if one exists.",
				PlanModifiers: []planmodifier.Int32{
					useStateForUnknownUnlessMarkdownChangesInt32(),
				},
			},
			"current_version_created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp for the current Terms of Service version, if one exists.",
				PlanModifiers: []planmodifier.String{
					useStateForUnknownUnlessMarkdownChangesString(),
				},
			},
			"current_version_created_by_user_id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User ID recorded by Ona as publisher of the current Terms of Service version.",
				PlanModifiers: []planmodifier.String{
					useStateForUnknownUnlessMarkdownChangesString(),
				},
			},
		},
	}
}

func (r *TermsOfServiceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *TermsOfServiceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics.Append(validateTermsOfServiceConfig(ctx, req.Config)...)
}

func (r *TermsOfServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TermsOfServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", termsOfServiceResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}

	markdown, diags := configuredMarkdown(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if markdown != nil && authenticated.Principal != v1.Principal_PRINCIPAL_USER {
		addTermsPublishingUserPATDiagnostic(&resp.Diagnostics)
		return
	}

	terms, err := r.updateTermsOfService(ctx, authenticated.ID, &data.Enabled, markdown)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Terms of Service", "updating Ona terms of service", err)
		return
	}

	data.ID = types.StringValue(authenticated.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), data.ID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planned := data
	data = TermsOfServiceModel{}
	resp.Diagnostics.Append(populateTermsOfServiceModel(&data, terms, authenticated.ID, planned)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TermsOfServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TermsOfServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", termsOfServiceResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, data.ID, authenticated.ID, termsOfServiceResourceType) {
		return
	}

	terms, err := r.getTermsOfService(ctx, authenticated.ID)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Terms of Service", "reading Ona terms of service", err)
		return
	}

	prior := data
	data = TermsOfServiceModel{}
	resp.Diagnostics.Append(populateTermsOfServiceModel(&data, terms, authenticated.ID, prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TermsOfServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TermsOfServiceModel
	var prior TermsOfServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating", termsOfServiceResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, prior.ID, authenticated.ID, termsOfServiceResourceType) {
		return
	}

	markdown, diags := configuredMarkdown(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if markdown != nil {
		if !prior.Markdown.IsNull() && !prior.Markdown.IsUnknown() && *markdown == prior.Markdown.ValueString() {
			markdown = nil
		} else if authenticated.Principal != v1.Principal_PRINCIPAL_USER {
			addTermsPublishingUserPATDiagnostic(&resp.Diagnostics)
			return
		}
	}

	var enabled *types.Bool
	if prior.Enabled.IsNull() || prior.Enabled.IsUnknown() || data.Enabled.ValueBool() != prior.Enabled.ValueBool() {
		enabled = &data.Enabled
	}

	terms, err := r.updateTermsOfService(ctx, authenticated.ID, enabled, markdown)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Terms of Service", "updating Ona terms of service", err)
		return
	}

	planned := data
	data = TermsOfServiceModel{}
	resp.Diagnostics.Append(populateTermsOfServiceModel(&data, terms, authenticated.ID, planned)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TermsOfServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TermsOfServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", termsOfServiceResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, data.ID, authenticated.ID, termsOfServiceResourceType) {
		return
	}

	enabled := types.BoolValue(false)
	if _, err := r.updateTermsOfService(ctx, authenticated.ID, &enabled, nil); err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Terms of Service", "disabling Ona terms of service", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *TermsOfServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !r.requireClient(&resp.Diagnostics, "importing", termsOfServiceResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !validSingletonImportID(req.ID, authenticated.ID) {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Use %q or the authenticated organization ID %q to import %s.", "current", authenticated.ID, termsOfServiceResourceType))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(authenticated.ID))...)
}

func (r *TermsOfServiceResource) getTermsOfService(ctx context.Context, organizationID string) (*v1.TermsOfService, error) {
	result, err := r.client.OrganizationService().GetTermsOfService(ctx, connect.NewRequest(&v1.GetTermsOfServiceRequest{
		OrganizationId: organizationID,
	}))
	if err != nil {
		return nil, fmt.Errorf("get terms of service: %w", err)
	}
	return result.Msg.GetTermsOfService(), nil
}

func (r *TermsOfServiceResource) updateTermsOfService(ctx context.Context, organizationID string, enabled *types.Bool, markdown *string) (*v1.TermsOfService, error) {
	updateReq := &v1.UpdateTermsOfServiceRequest{
		OrganizationId: organizationID,
		Markdown:       markdown,
	}
	if enabled != nil && !enabled.IsNull() && !enabled.IsUnknown() {
		value := enabled.ValueBool()
		updateReq.Enabled = &value
	}

	result, err := r.client.OrganizationService().UpdateTermsOfService(ctx, connect.NewRequest(updateReq))
	if err != nil {
		return nil, fmt.Errorf("update terms of service: %w", err)
	}
	return result.Msg.GetTermsOfService(), nil
}

func populateTermsOfServiceModel(data *TermsOfServiceModel, terms *v1.TermsOfService, authenticatedID string, planned TermsOfServiceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if terms == nil {
		diags.AddError("Missing Terms of Service", "The Ona API returned an empty terms of service object.")
		return diags
	}
	if terms.GetOrganizationId() != "" && terms.GetOrganizationId() != authenticatedID {
		diags.AddError(
			"Unexpected Terms of Service Organization",
			fmt.Sprintf("The Ona API returned terms of service for organization %q, but the provider token is authenticated for organization %q.", terms.GetOrganizationId(), authenticatedID),
		)
		return diags
	}

	data.ID = types.StringValue(authenticatedID)
	data.Enabled = types.BoolValue(terms.GetEnabled())

	version := terms.GetCurrentVersion()
	if version == nil {
		data.Markdown = types.StringNull()
		data.CurrentVersionID = types.StringNull()
		data.CurrentVersion = types.Int32Null()
		data.CurrentVersionCreatedAt = types.StringNull()
		data.CurrentVersionCreatedByUserID = types.StringNull()
		return diags
	}

	data.Markdown = types.StringValue(version.GetMarkdown())
	if !planned.Markdown.IsNull() && !planned.Markdown.IsUnknown() && planned.Markdown.ValueString() == version.GetMarkdown() {
		data.Markdown = planned.Markdown
	}
	data.CurrentVersionID = types.StringValue(version.GetId())
	data.CurrentVersion = types.Int32Value(version.GetVersion())
	data.CurrentVersionCreatedAt = timestampRFC3339(version.GetCreatedAt())
	data.CurrentVersionCreatedByUserID = types.StringValue(version.GetCreatedByUserId())
	return diags
}

func validateTermsOfServiceConfig(ctx context.Context, cfg tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	var enabled types.Bool
	var markdown types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("enabled"), &enabled)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("markdown"), &markdown)...)
	if diags.HasError() {
		return diags
	}

	if !markdown.IsNull() && !markdown.IsUnknown() && utf8.RuneCountInString(markdown.ValueString()) > 32000 {
		diags.AddAttributeError(path.Root("markdown"), "Invalid Terms of Service Markdown", "markdown must be at most 32000 characters.")
	}
	if enabled.IsNull() || enabled.IsUnknown() || !enabled.ValueBool() {
		return diags
	}
	if markdown.IsUnknown() {
		return diags
	}
	if markdown.IsNull() || strings.TrimSpace(markdown.ValueString()) == "" {
		diags.AddAttributeError(path.Root("markdown"), "Missing Terms of Service Markdown", "Set non-empty markdown when enabled is true.")
	}
	return diags
}

func configuredMarkdown(ctx context.Context, cfg tfsdk.Config) (*string, diag.Diagnostics) {
	var diags diag.Diagnostics
	var markdown types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("markdown"), &markdown)...)
	if diags.HasError() || markdown.IsNull() || markdown.IsUnknown() {
		return nil, diags
	}
	value := markdown.ValueString()
	return &value, diags
}

func addTermsPublishingUserPATDiagnostic(diags *diag.Diagnostics) {
	diags.AddError(
		"Terms of Service Publishing Requires User Token",
		"The configured Ona token is not a user personal access token. Publishing or changing ona_terms_of_service.markdown currently requires a user PAT because Ona records a user ID as the publisher of each immutable Terms of Service version.",
	)
}

func useStateForUnknownUnlessMarkdownChangesString() planmodifier.String {
	return stateForUnknownUnlessMarkdownChangesString{}
}

type stateForUnknownUnlessMarkdownChangesString struct{}

func (m stateForUnknownUnlessMarkdownChangesString) Description(ctx context.Context) string {
	return "Uses prior state for unknown values unless Terms of Service Markdown is changing."
}

func (m stateForUnknownUnlessMarkdownChangesString) MarkdownDescription(ctx context.Context) string {
	return "Uses prior state for unknown values unless Terms of Service Markdown is changing."
}

func (m stateForUnknownUnlessMarkdownChangesString) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || !req.PlanValue.IsUnknown() || termsMarkdownConfiguredChange(ctx, req.Config, req.State, &resp.Diagnostics) {
		return
	}
	resp.PlanValue = req.StateValue
}

func useStateForUnknownUnlessMarkdownChangesInt32() planmodifier.Int32 {
	return stateForUnknownUnlessMarkdownChangesInt32{}
}

type stateForUnknownUnlessMarkdownChangesInt32 struct{}

func (m stateForUnknownUnlessMarkdownChangesInt32) Description(ctx context.Context) string {
	return "Uses prior state for unknown values unless Terms of Service Markdown is changing."
}

func (m stateForUnknownUnlessMarkdownChangesInt32) MarkdownDescription(ctx context.Context) string {
	return "Uses prior state for unknown values unless Terms of Service Markdown is changing."
}

func (m stateForUnknownUnlessMarkdownChangesInt32) PlanModifyInt32(ctx context.Context, req planmodifier.Int32Request, resp *planmodifier.Int32Response) {
	if req.State.Raw.IsNull() || !req.PlanValue.IsUnknown() || termsMarkdownConfiguredChange(ctx, req.Config, req.State, &resp.Diagnostics) {
		return
	}
	resp.PlanValue = req.StateValue
}

func termsMarkdownConfiguredChange(ctx context.Context, cfg tfsdk.Config, state tfsdk.State, diags *diag.Diagnostics) bool {
	var configured types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("markdown"), &configured)...)
	if diags.HasError() || configured.IsNull() || configured.IsUnknown() {
		return false
	}

	var prior types.String
	diags.Append(state.GetAttribute(ctx, path.Root("markdown"), &prior)...)
	if diags.HasError() || prior.IsNull() || prior.IsUnknown() {
		return true
	}
	return configured.ValueString() != prior.ValueString()
}
