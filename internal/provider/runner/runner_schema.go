// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona runner.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform resource ID. This is the same value as `runner_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner display name.",
			},
			"runner_provider": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner provider. Supported values are `aws_ec2` and `gcp`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"runner_manager_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Runner manager ID for managed runners.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner kind deduced by the API from the provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloudformation_template_url": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "CloudFormation template URL for AWS EC2 runner setup. This is null for non-AWS runners.",
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the runner was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the runner was last updated.",
			},
			"status":  resourceStatusSchema(),
			"creator": resourceCreatorSchema(),
		},
		Blocks: map[string]resourceschema.Block{
			"configuration": resourceConfigurationSchema(),
		},
	}
}

func resourceConfigurationSchema() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "Runner configuration.",
		Attributes: map[string]resourceschema.Attribute{
			"region": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Region hint for remote runners. Required for `aws_ec2` runners.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"release_channel": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Runner release channel. Supported values are `stable` and `latest`. Defaults to `stable`.",
				Default:             stringdefault.StaticString("stable"),
			},
			"auto_update": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the runner should automatically update itself. Defaults to `true`.",
				Default:             booldefault.StaticBool(true),
			},
			"devcontainer_image_cache_enabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the shared devcontainer build cache is enabled for this runner. Defaults to `true`.",
				Default:             booldefault.StaticBool(true),
			},
			"log_level": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Runner log level. Supported values are `debug`, `info`, `warn`, and `error`. Defaults to `info`.",
				Default:             stringdefault.StaticString("info"),
			},
		},
		Blocks: map[string]resourceschema.Block{
			"update_window": resourceschema.SingleNestedBlock{
				MarkdownDescription: "Daily UTC window during which auto-updates may run.",
				Attributes: map[string]resourceschema.Attribute{
					"start": resourceschema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Start time in `HH:00` UTC format. Required when `update_window` is set.",
					},
					"end": resourceschema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "End time in `HH:00` UTC format. If omitted, the API defaults to two hours after `start`.",
					},
				},
			},
		},
	}
}

func resourceStatusSchema() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Runner status reported by the runner.",
		Attributes: map[string]resourceschema.Attribute{
			"phase": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner phase.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"region": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Actual region reported by the runner.",
			},
			"message": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner status message.",
			},
			"version": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner version.",
			},
			"log_url": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner log URL.",
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the runner status was last updated.",
			},
			"system_details": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner system details.",
			},
			"support_bundle_url": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner support bundle URL.",
			},
		},
	}
}

func resourceCreatorSchema() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Identity that created the runner.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creator subject ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"principal": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creator principal type.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
