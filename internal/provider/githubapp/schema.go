// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package githubapp

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manages an organization-owned GitHub App integration. Create and credential rotation require the complete write-only credential bundle and a positive credentials_version. Import and refresh use saved non-secret metadata. Deleting this resource removes the Ona integration; it does not uninstall or delete the GitHub App.",
		Attributes: map[string]schema.Attribute{
			"id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Ona integration UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"app_id":  schema.StringAttribute{Required: true, MarkdownDescription: "GitHub App ID as a canonical positive decimal integer. Changing it replaces the integration.", Validators: []validator.String{appIDValidator{}}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Whether the integration is enabled. Defaults to true."},
			"credentials": schema.SingleNestedAttribute{
				Optional: true, WriteOnly: true, Sensitive: true,
				MarkdownDescription: "Full GitHub App credential bundle. Read only from configuration, never stored in plan or state. Required on creation and whenever credentials_version changes. An empty object is treated as omitted for import and unchanged credentials. Changing only these values does not trigger rotation.",
				Attributes: map[string]schema.Attribute{
					"private_key":    credentialAttribute("PEM-encoded GitHub App private key."),
					"client_secret":  credentialAttribute("GitHub App OAuth client secret."),
					"webhook_secret": credentialAttribute("GitHub App webhook secret."),
				},
			},
			"credentials_version": schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, MarkdownDescription: "Positive local rotation marker. Change it with the complete credentials bundle to rotate credentials in place. Omit when importing; Ona cannot recover this marker or detect secret drift."},
			"app_slug":            schema.StringAttribute{Computed: true, MarkdownDescription: "Saved GitHub App slug, refreshed during credential rotation."},
			"client_id":           schema.StringAttribute{Computed: true, MarkdownDescription: "Saved GitHub App OAuth client ID."},
			"setup": schema.SingleNestedAttribute{Computed: true, MarkdownDescription: "Stable setup URLs. Complete App installation in the Ona dashboard using an authenticated browser.", Attributes: map[string]schema.Attribute{
				"callback_url":     schema.StringAttribute{Computed: true, MarkdownDescription: "Canonical OAuth callback URL from the GitHub definition."},
				"webhook_url":      schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub App webhook endpoint at the canonical callback origin."},
				"installation_url": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization integrations dashboard for the configured Ona deployment."},
			}},
			"installation": schema.SingleNestedAttribute{Computed: true, MarkdownDescription: "Saved GitHub installation metadata, or null while installation is pending. Refresh does not validate installation access with GitHub.", Attributes: map[string]schema.Attribute{
				"id":           schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub installation ID."},
				"account_name": schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub account or organization login."},
				"account_type": schema.StringAttribute{Computed: true, MarkdownDescription: "GitHub account kind."},
			}},
		},
	}
}

func credentialAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: description}
}

type appIDValidator struct{}

func (appIDValidator) Description(context.Context) string {
	return "Must be a canonical positive int64 decimal string."
}
func (v appIDValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (appIDValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !validAppID(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid GitHub App ID", "Use a positive decimal integer without whitespace, signs, or leading zeros, at most 9223372036854775807.")
	}
}

func validAppID(value string) bool {
	id, err := strconv.ParseInt(value, 10, 64)
	return err == nil && id > 0 && strconv.FormatInt(id, 10) == value
}
