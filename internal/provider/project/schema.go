// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona project backed by a Git repository. Use this resource to define the repository, branch, environment classes, and optional prebuild settings that Ona should use for environments created from the project.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID that owns the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project display name shown in Ona.",
			},
			"repository_clone_url": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Git repository clone URL. Use an HTTP(S) or SSH clone URL, such as `https://github.com/ona/example.git` or `git@github.com:ona/example.git`.",
			},
			"branch": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Git branch name Ona should use when creating environments and prebuilds.",
			},
			"devcontainer_file_path": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Path to the devcontainer file, relative to the repository root. Omit to let Ona use its default discovery behavior.",
			},
			"automations_file_path": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Path to the automations file, relative to the repository root. Omit to let Ona use its default discovery behavior.",
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the project was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the project was last updated.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"creator": subjectAttribute("Identity that created the project."),
		},
		Blocks: map[string]resourceschema.Block{
			"environment_class":      environmentClassBlock(),
			"prebuild_configuration": prebuildConfigurationBlock(),
		},
	}
}

func environmentClassBlock() resourceschema.ListNestedBlock {
	return resourceschema.ListNestedBlock{
		MarkdownDescription: "Environment classes available to this project, in priority order. Configure at least one block. Each block must set exactly one of `environment_class_id` or `local_runner = true`.",
		NestedObject: resourceschema.NestedBlockObject{
			Attributes: map[string]resourceschema.Attribute{
				"environment_class_id": resourceschema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Runner environment class ID to make available to this project. Omit when `local_runner` is true.",
				},
				"local_runner": resourceschema.BoolAttribute{
					Optional:            true,
					MarkdownDescription: "Whether this entry represents the user's local runner. Set to `true` instead of `environment_class_id` for local-runner projects.",
				},
				"order": resourceschema.Int64Attribute{
					Required:            true,
					MarkdownDescription: "Priority order for this environment class entry. Lower values are preferred first and values must be unique within the project.",
				},
			},
		},
	}
}

func prebuildConfigurationBlock() resourceschema.ListNestedBlock {
	return resourceschema.ListNestedBlock{
		MarkdownDescription: "Prebuild configuration for the project. Set no more than one block. Omitting the block disables Terraform management of prebuild settings. Warm pools for prebuilt environment classes are managed separately with `ona_warm_pool` resources.",
		NestedObject: resourceschema.NestedBlockObject{
			Attributes: map[string]resourceschema.Attribute{
				"enabled": resourceschema.BoolAttribute{
					Optional:            true,
					Computed:            true,
					Default:             booldefault.StaticBool(true),
					MarkdownDescription: "Whether prebuilds are enabled for this project. Defaults to the provider value `true` when the prebuild block is present.",
				},
				"environment_class_ids": resourceschema.SetAttribute{
					Optional:            true,
					ElementType:         types.StringType,
					MarkdownDescription: "Environment class IDs for which prebuilds should be created. Omit to let Ona use project defaults.",
				},
				"timeout": resourceschema.StringAttribute{
					Optional:            true,
					Computed:            true,
					Default:             stringdefault.StaticString("1h"),
					MarkdownDescription: "Maximum duration allowed for a prebuild to complete, as a Go duration string. Must be between `5m` and `2h`; defaults to the provider value `1h`.",
				},
				"enable_jetbrains_warmup": resourceschema.BoolAttribute{
					Optional:            true,
					Computed:            true,
					Default:             booldefault.StaticBool(false),
					MarkdownDescription: "Whether JetBrains IDE warmup runs during prebuilds. Defaults to the provider value `false`.",
				},
			},
			Blocks: map[string]resourceschema.Block{
				"daily_schedule": resourceschema.ListNestedBlock{
					MarkdownDescription: "Daily UTC prebuild schedule. Set no more than one block.",
					NestedObject: resourceschema.NestedBlockObject{
						Attributes: map[string]resourceschema.Attribute{
							"hour_utc": resourceschema.Int64Attribute{
								Required:            true,
								MarkdownDescription: "UTC hour of day, from 0 through 23, when the prebuild should start.",
							},
						},
					},
				},
				"executor": resourceschema.ListNestedBlock{
					MarkdownDescription: "Subject whose SCM credentials are used to run prebuilds. Set no more than one block. Omit to use Ona defaults.",
					NestedObject: resourceschema.NestedBlockObject{
						Attributes: map[string]resourceschema.Attribute{
							"id": resourceschema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Executor subject ID.",
							},
							"principal": resourceschema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Executor principal type. Supported values are `user` and `service_account`.",
							},
						},
					},
				},
			},
		},
	}
}

func subjectAttribute(description string) resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subject ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"principal": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subject principal type.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
