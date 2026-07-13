// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &AnnouncementBannerResource{}

func NewAnnouncementBannerListResource() list.ListResource {
	return &AnnouncementBannerResource{}
}

type announcementBannerListModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
}

func (r *AnnouncementBannerResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists the singleton Ona announcement banner when it is configured.",
		Attributes: map[string]listschema.Attribute{
			"organization_id": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Organization ID to query. Defaults to the authenticated organization.",
			},
		},
	}
}

func (r *AnnouncementBannerResource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_announcement_banner resources")))
			return
		}
		if !listutil.HasCapacity(req.Limit, 0) {
			return
		}

		var data announcementBannerListModel
		diags := req.Config.Get(ctx, &data)
		if !listutil.PushDiagnostics(push, diags) {
			return
		}

		organizationID := data.OrganizationID.ValueString()
		if organizationID == "" {
			var err error
			organizationID, err = listutil.AuthenticatedOrganizationID(ctx, r.client)
			if err != nil {
				push(listutil.Error("Unable to Determine Organization", err))
				return
			}
		}

		banner, err := r.getAnnouncementBanner(ctx, organizationID)
		if connect.CodeOf(err) == connect.CodeNotFound {
			return
		}
		if err != nil {
			push(listutil.Error("Unable to Read Ona Announcement Banner", err))
			return
		}

		item := req.NewListResult(ctx)
		item.DisplayName = organizationID
		item.Diagnostics.Append(item.Identity.Set(ctx, AnnouncementBannerIdentityModel{
			OrganizationID: types.StringValue(organizationID),
		})...)
		if req.IncludeResource && !item.Diagnostics.HasError() {
			var model AnnouncementBannerModel
			item.Diagnostics.Append(populateAnnouncementBannerModel(&model, banner, organizationID, AnnouncementBannerModel{})...)
			if !item.Diagnostics.HasError() {
				item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
			}
		}
		push(item)
	}
}
