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
		MarkdownDescription: "Ona runner registration. Use this resource to create a remote runner record, then deploy the runner with the generated setup output for the selected provider.",
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
				MarkdownDescription: "Ona runner ID. Use this value when configuring runner environment classes, SCM integrations, and runner token flows.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner display name shown in Ona.",
			},
			"runner_provider": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Cloud provider for the runner. Supported values are `aws_ec2` and `gcp`. Changing this value replaces the runner.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner kind assigned by the Ona API from the selected provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloudformation_template_url": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "CloudFormation template URL for AWS EC2 runner setup. This is populated only for `aws_ec2` runners and is null for GCP runners.",
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the runner was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"creator": resourceCreatorSchema(),
		},
		Blocks: map[string]resourceschema.Block{
			"configuration": resourceConfigurationSchema(),
		},
	}
}

func resourceConfigurationSchema() resourceschema.SingleNestedBlock {
	return resourceschema.SingleNestedBlock{
		MarkdownDescription: "Runner configuration applied to the remote runner. Some fields are provider defaults and are preserved in Terraform state after creation.",
		Attributes: map[string]resourceschema.Attribute{
			"region": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Cloud region for the runner. Required for `aws_ec2` runners and omitted for providers that do not use this setting. Changing this value replaces the runner.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"release_channel": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Runner release channel. Supported values are `stable` and `latest`. Defaults to the provider value `stable`.",
				Default:             stringdefault.StaticString("stable"),
			},
			"auto_update": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the runner should automatically update itself. Defaults to the provider value `true`.",
				Default:             booldefault.StaticBool(true),
			},
			"devcontainer_image_cache_enabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the shared devcontainer image build cache is enabled for this runner. Defaults to the provider value `true`.",
				Default:             booldefault.StaticBool(true),
			},
			"log_level": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Runner log level. Supported values are `debug`, `info`, `warn`, and `error`. Defaults to the provider value `info`.",
				Default:             stringdefault.StaticString("info"),
			},
		},
		Blocks: map[string]resourceschema.Block{
			"metrics": resourceschema.SingleNestedBlock{
				MarkdownDescription: "Metrics delivery configuration. Use `managed_metrics_enabled` for Ona-managed metrics, or configure the custom remote-write pipeline fields.",
				Attributes: map[string]resourceschema.Attribute{
					"enabled": resourceschema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: "Whether the runner sends metrics to the configured custom remote-write pipeline. Defaults to `false`.",
					},
					"url": resourceschema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Remote-write URL for a custom metrics pipeline.",
					},
					"username": resourceschema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Username for authenticating to the custom metrics pipeline.",
					},
					"password": resourceschema.StringAttribute{
						Optional:            true,
						Sensitive:           true,
						WriteOnly:           true,
						MarkdownDescription: "Password or token for authenticating to the custom metrics pipeline. This value is sensitive and write-only, so Terraform does not store it in plan or state.",
					},
					"password_version": resourceschema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "User-managed version marker for resubmitting or rotating `password`. Change this value when supplying a new password or token.",
					},
					"managed_metrics_enabled": resourceschema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: "Whether the runner sends metrics through Ona's managed metrics pipeline. Defaults to `false`.",
					},
				},
			},
			"update_window": resourceschema.SingleNestedBlock{
				MarkdownDescription: "Daily UTC window during which runner auto-updates may run.",
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
