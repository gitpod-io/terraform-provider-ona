// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"unicode/utf8"

	"github.com/google/uuid"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type UUIDStringValidator struct{}

func (UUIDStringValidator) Description(context.Context) string {
	return "value must be a UUID"
}

func (UUIDStringValidator) MarkdownDescription(context.Context) string {
	return "Value must be a UUID."
}

func (UUIDStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := uuid.Parse(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid UUID", "Value must be a valid UUID.")
	}
}

type SearchStringValidator struct{}

func (SearchStringValidator) Description(context.Context) string {
	return "search must not exceed 256 characters"
}

func (SearchStringValidator) MarkdownDescription(context.Context) string {
	return "Search must not exceed 256 characters."
}

func (SearchStringValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if utf8.RuneCountInString(req.ConfigValue.ValueString()) > 256 {
		resp.Diagnostics.AddAttributeError(req.Path, "User Search Is Too Long", "search must not exceed 256 characters.")
	}
}

func userDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Fetches one Ona user by UUID from the organization associated with the configured token. The user must be visible to that token; suspended or departed users can require organization-admin access.",
		Attributes: userAttributes(datasourceschema.StringAttribute{
			Required:            true,
			MarkdownDescription: "UUID of the existing Ona user to look up.",
			Validators: []validator.String{
				UUIDStringValidator{},
			},
		}),
	}
}

func userCollectionDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Lists Ona users visible to the configured token in its organization. Suspended or departed users can require organization-admin access.",
		Attributes: map[string]datasourceschema.Attribute{
			"search": datasourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Case-insensitive substring search over user names and email addresses. A UUID search matches that user ID exactly. Blank input is ignored.",
				Validators: []validator.String{
					SearchStringValidator{},
				},
			},
			"statuses": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "User statuses to include. Supported values are `active`, `suspended`, and `left`.",
			},
			"roles": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Organization roles to include. Supported values are `admin` and `member`.",
			},
			"user_ids": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "UUIDs of users to include. The Ona API accepts at most 25 values.",
			},
			"users": datasourceschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching users sorted by user ID for deterministic Terraform state.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: userAttributes(datasourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Stable UUID of the Ona user.",
					}),
				},
			},
		},
	}
}

func userAttributes(userID datasourceschema.StringAttribute) map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"id": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Terraform data source ID. This is the same value as `user_id`.",
		},
		"user_id": userID,
		"name": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "User display name.",
		},
		"email": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "User email address, or null when unavailable.",
		},
		"status": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "User status: `active`, `suspended`, or `left`.",
		},
		"role": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Organization role: `admin` or `member`.",
		},
		"member_since": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Time the backend user record was created, formatted as RFC 3339. This may not be the most recent rejoin time.",
		},
		"login_provider": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Login provider reported for the user, or null when unavailable.",
		},
	}
}
