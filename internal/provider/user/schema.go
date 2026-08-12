// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"fmt"
	"net/mail"
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

type EmailSelectorValidator struct{}

func (EmailSelectorValidator) Description(context.Context) string {
	return "email must be a valid email address between 1 and 256 characters"
}

func (EmailSelectorValidator) MarkdownDescription(context.Context) string {
	return "Email must be a valid email address between 1 and 256 characters."
}

func (EmailSelectorValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	length := utf8.RuneCountInString(value)
	if length == 0 || length > 256 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Ona User Email", "email must contain between 1 and 256 characters.")
		return
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Name != "" || parsed.Address != value {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Ona User Email", "email must be a valid email address.")
	}
}

type LoginProviderValidator struct{}

func (LoginProviderValidator) Description(context.Context) string {
	return "login_provider must be custom, github, or google"
}

func (LoginProviderValidator) MarkdownDescription(context.Context) string {
	return "Login provider must be `custom`, `github`, or `google`."
}

func (LoginProviderValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value != "custom" && value != "github" && value != "google" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Ona Login Provider", fmt.Sprintf("Unsupported login_provider %q. Use custom, github, or google.", value))
	}
}

func userDataSourceSchema() datasourceschema.Schema {
	attributes := userAttributes(datasourceschema.StringAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: "Stable UUID of the existing Ona user. Specify this alone, or omit it and specify both `email` and `login_provider`.",
		Validators: []validator.String{
			UUIDStringValidator{},
		},
	})
	attributes["email"] = datasourceschema.StringAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: "Exact user email to look up, matched case-insensitively. Must be paired with `login_provider` when `user_id` is omitted.",
		Validators: []validator.String{
			EmailSelectorValidator{},
		},
	}
	attributes["login_provider"] = datasourceschema.StringAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: "Exact login provider to pair with `email`. Supported values are `github`, `google`, and `custom` (SSO).",
		Validators: []validator.String{
			LoginProviderValidator{},
		},
	}
	return datasourceschema.Schema{
		MarkdownDescription: "Fetches one Ona user by UUID or by an exact email and login-provider pair from the organization associated with the configured token. The user must be visible to that token; suspended or departed users can require organization-admin access.",
		Attributes:          attributes,
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
