// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona webhook for receiving SCM events. Webhooks must be created with a user or administrator credential because the Ona API does not allow service accounts to create them. Changing `secret_version` rotates the generated signing secret and immediately invalidates its previous value; retrieve the current value with the `ona_webhook_secret` ephemeral resource. Removing this resource deletes the webhook and converts triggers on bound workflows to manual triggers.",
		Attributes: map[string]resourceschema.Attribute{
			"id": computedStableString("Webhook ID. Use this value as the Terraform import ID."),
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Webhook display name. Must be between 1 and 80 characters.",
			},
			"description": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description of the webhook's purpose. Must not exceed 500 characters.",
			},
			"type": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Webhook scope type. Supported values are `repository` and `organization`. Changes require replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scm_provider": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SCM provider. Supported values are `github`, `gitlab`, and `bitbucket`. Changes require replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"repository_scopes": resourceschema.SetNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Repositories handled by a `repository` webhook. Configure between 1 and 100 entries. Do not set this for an `organization` webhook.",
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"host": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "SCM host, such as `github.com` or `gitlab.com`.",
						},
						"owner": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Repository owner or organization.",
						},
						"name": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Repository name.",
						},
					},
				},
			},
			"organization_scope": resourceschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "SCM organization handled by an `organization` webhook. Do not set this for a `repository` webhook.",
				Attributes: map[string]resourceschema.Attribute{
					"host": resourceschema.StringAttribute{
						Required:            true,
						MarkdownDescription: "SCM host, such as `github.com` or `gitlab.com`.",
					},
					"name": resourceschema.StringAttribute{
						Required:            true,
						MarkdownDescription: "SCM organization or group name.",
					},
				},
			},
			"secret_version": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User-managed rotation marker. Changing this value rotates the webhook signing secret during apply. The secret itself is never stored in this resource's state.",
			},
			"organization_id": computedStableString("Organization ID that owns the webhook."),
			"url":             computedStableString("Generated webhook endpoint URL."),
			"bound_workflow_count": resourceschema.Int32Attribute{
				Computed:            true,
				MarkdownDescription: "Number of workflows currently bound to the webhook.",
			},
			"last_triggered_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the webhook was last triggered.",
			},
			"creator": resourceschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created the webhook.",
				Attributes: map[string]resourceschema.Attribute{
					"id":        computedStableString("Creator subject ID."),
					"principal": computedStableString("Creator principal type."),
				},
			},
			"created_at": computedStableString("Time when the webhook was created."),
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the webhook was last updated.",
			},
		},
	}
}

func computedStableString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}
