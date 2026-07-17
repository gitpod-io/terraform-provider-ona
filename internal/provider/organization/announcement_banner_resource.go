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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const announcementBannerResourceType = "ona_announcement_banner"

var _ resource.Resource = &AnnouncementBannerResource{}
var _ resource.ResourceWithConfigure = &AnnouncementBannerResource{}
var _ resource.ResourceWithImportState = &AnnouncementBannerResource{}
var _ resource.ResourceWithValidateConfig = &AnnouncementBannerResource{}

func NewAnnouncementBannerResource() resource.Resource {
	return &AnnouncementBannerResource{}
}

type AnnouncementBannerResource struct {
	clientHolder
}

type AnnouncementBannerModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Message types.String `tfsdk:"message"`
}

func (r *AnnouncementBannerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_announcement_banner"
}

func (r *AnnouncementBannerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Singleton Ona announcement banner shown on the organization home page. Organization scope is resolved from the configured provider token. Destroying this resource disables and clears the remote banner because the Ona API has no delete operation.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Authenticated organization ID for the managed announcement banner singleton.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": resourceschema.BoolAttribute{
				Required:            true,
				MarkdownDescription: "Whether the announcement banner is displayed to organization members.",
			},
			"message": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Announcement banner body. Supports basic Markdown and must be at most 1000 characters. Required when `enabled` is true.",
			},
		},
	}
}

func (r *AnnouncementBannerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *AnnouncementBannerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics.Append(validateAnnouncementBannerConfig(ctx, req.Config)...)
}

func (r *AnnouncementBannerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AnnouncementBannerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", announcementBannerResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}

	banner, err := r.updateAnnouncementBanner(ctx, authenticated.ID, data.Enabled.ValueBool(), data.Message.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Announcement Banner", "updating Ona announcement banner", err)
		return
	}

	data.ID = types.StringValue(authenticated.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planned := data
	data = AnnouncementBannerModel{}
	resp.Diagnostics.Append(populateAnnouncementBannerModel(&data, banner, authenticated.ID, planned)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnouncementBannerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AnnouncementBannerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", announcementBannerResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, data.ID, authenticated.ID, announcementBannerResourceType) {
		return
	}

	banner, err := r.getAnnouncementBanner(ctx, authenticated.ID)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Announcement Banner", "reading Ona announcement banner", err)
		return
	}

	prior := data
	data = AnnouncementBannerModel{}
	resp.Diagnostics.Append(populateAnnouncementBannerModel(&data, banner, authenticated.ID, prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnouncementBannerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AnnouncementBannerModel
	var prior AnnouncementBannerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating", announcementBannerResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, prior.ID, authenticated.ID, announcementBannerResourceType) {
		return
	}

	banner, err := r.updateAnnouncementBanner(ctx, authenticated.ID, data.Enabled.ValueBool(), data.Message.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Announcement Banner", "updating Ona announcement banner", err)
		return
	}

	planned := data
	data = AnnouncementBannerModel{}
	resp.Diagnostics.Append(populateAnnouncementBannerModel(&data, banner, authenticated.ID, planned)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AnnouncementBannerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AnnouncementBannerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", announcementBannerResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, data.ID, authenticated.ID, announcementBannerResourceType) {
		return
	}

	if _, err := r.updateAnnouncementBanner(ctx, authenticated.ID, false, ""); err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Announcement Banner", "disabling and clearing Ona announcement banner", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *AnnouncementBannerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !r.requireClient(&resp.Diagnostics, "importing", announcementBannerResourceType) {
		return
	}

	authenticated, err := r.authenticatedOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Resolve Ona Organization", err.Error())
		return
	}
	if !validSingletonImportID(req.ID, authenticated.ID) {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Use %q or the authenticated organization ID %q to import %s.", "current", authenticated.ID, announcementBannerResourceType))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(authenticated.ID))...)
}

func (r *AnnouncementBannerResource) getAnnouncementBanner(ctx context.Context, organizationID string) (*v1.AnnouncementBanner, error) {
	result, err := r.client.OrganizationService().GetAnnouncementBanner(ctx, connect.NewRequest(&v1.GetAnnouncementBannerRequest{
		OrganizationId: organizationID,
	}))
	if err != nil {
		return nil, fmt.Errorf("get announcement banner: %w", err)
	}
	return result.Msg.GetBanner(), nil
}

func (r *AnnouncementBannerResource) updateAnnouncementBanner(ctx context.Context, organizationID string, enabled bool, message string) (*v1.AnnouncementBanner, error) {
	result, err := r.client.OrganizationService().UpdateAnnouncementBanner(ctx, connect.NewRequest(&v1.UpdateAnnouncementBannerRequest{
		OrganizationId: organizationID,
		Enabled:        &enabled,
		Message:        &message,
	}))
	if err != nil {
		return nil, fmt.Errorf("update announcement banner: %w", err)
	}
	return result.Msg.GetBanner(), nil
}

func populateAnnouncementBannerModel(data *AnnouncementBannerModel, banner *v1.AnnouncementBanner, authenticatedID string, planned AnnouncementBannerModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if banner == nil {
		diags.AddError("Missing Announcement Banner", "The Ona API returned an empty announcement banner.")
		return diags
	}
	if banner.GetOrganizationId() != "" && banner.GetOrganizationId() != authenticatedID {
		diags.AddError(
			"Unexpected Announcement Banner Organization",
			fmt.Sprintf("The Ona API returned an announcement banner for organization %q, but the provider token is authenticated for organization %q.", banner.GetOrganizationId(), authenticatedID),
		)
		return diags
	}

	data.ID = types.StringValue(authenticatedID)
	data.Enabled = types.BoolValue(banner.GetEnabled())
	data.Message = types.StringValue(banner.GetMessage())
	if !planned.Message.IsNull() && !planned.Message.IsUnknown() && planned.Message.ValueString() == banner.GetMessage() {
		data.Message = planned.Message
	}
	return diags
}

func validateAnnouncementBannerConfig(ctx context.Context, cfg tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	var enabled types.Bool
	var message types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("enabled"), &enabled)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("message"), &message)...)
	if diags.HasError() {
		return diags
	}

	if !message.IsNull() && !message.IsUnknown() && utf8.RuneCountInString(message.ValueString()) > 1000 {
		diags.AddAttributeError(path.Root("message"), "Invalid Announcement Banner Message", "message must be at most 1000 characters.")
	}
	if enabled.IsNull() || enabled.IsUnknown() || !enabled.ValueBool() {
		return diags
	}
	if message.IsUnknown() {
		return diags
	}
	if message.IsNull() || strings.TrimSpace(message.ValueString()) == "" {
		diags.AddAttributeError(path.Root("message"), "Missing Announcement Banner Message", "Set a non-empty message when enabled is true.")
	}
	return diags
}

func validSingletonImportID(importID string, organizationID string) bool {
	return importID == "current" || importID == organizationID
}
