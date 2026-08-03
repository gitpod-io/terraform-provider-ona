// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func dataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Fetches an Ona project by ID and exposes the same fields as the `ona_project` managed resource.",
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform data source ID. This is the same value as `project_id`.",
			},
			"project_id": datasourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Ona project ID to look up.",
			},
			"name": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project display name shown in Ona.",
			},
			"repository_clone_url": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Git repository clone URL.",
			},
			"branch": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Git branch name Ona uses when creating environments and prebuilds.",
			},
			"insights_enabled": datasourceschema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether Ona Insights is enabled for the project.",
			},
			"devcontainer_file_path": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Path to the devcontainer file, relative to the repository root.",
			},
			"automations_file_path": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Path to the automations file, relative to the repository root.",
			},
			"created_at": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the project was created.",
			},
			"creator":                projectDataSourceSubjectAttribute("Identity that created the project."),
			"environment_class":      projectDataSourceEnvironmentClassesAttribute(),
			"prebuild_configuration": projectDataSourcePrebuildAttribute(),
		},
	}
}

func projectDataSourceEnvironmentClassesAttribute() datasourceschema.ListNestedAttribute {
	return datasourceschema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Environment classes available to this project, in priority order.",
		NestedObject: datasourceschema.NestedAttributeObject{
			Attributes: map[string]datasourceschema.Attribute{
				"environment_class_id": datasourceschema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Runner environment class ID available to this project. Null for a local-runner entry.",
				},
				"local_runner": datasourceschema.BoolAttribute{
					Computed:            true,
					MarkdownDescription: "Whether this entry represents the user's local runner.",
				},
				"order": datasourceschema.Int64Attribute{
					Computed:            true,
					MarkdownDescription: "Priority order for this environment class entry. Lower values are preferred first.",
				},
			},
		},
	}
}

func projectDataSourcePrebuildAttribute() datasourceschema.ListNestedAttribute {
	return datasourceschema.ListNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Prebuild configuration for the project.",
		NestedObject: datasourceschema.NestedAttributeObject{
			Attributes: map[string]datasourceschema.Attribute{
				"enabled": datasourceschema.BoolAttribute{
					Computed:            true,
					MarkdownDescription: "Whether prebuilds are enabled for this project.",
				},
				"environment_class_ids": datasourceschema.SetAttribute{
					Computed:            true,
					ElementType:         types.StringType,
					MarkdownDescription: "Environment class IDs for which prebuilds are created.",
				},
				"timeout": datasourceschema.StringAttribute{
					Computed:            true,
					MarkdownDescription: "Maximum duration allowed for a prebuild to complete, as a Go duration string.",
				},
				"enable_jetbrains_warmup": datasourceschema.BoolAttribute{
					Computed:            true,
					MarkdownDescription: "Whether JetBrains IDE warmup runs during prebuilds.",
				},
				"daily_schedule": datasourceschema.ListNestedAttribute{
					Computed:            true,
					MarkdownDescription: "Daily UTC prebuild schedule.",
					NestedObject: datasourceschema.NestedAttributeObject{
						Attributes: map[string]datasourceschema.Attribute{
							"hour_utc": datasourceschema.Int64Attribute{
								Computed:            true,
								MarkdownDescription: "UTC hour of day when the prebuild starts.",
							},
						},
					},
				},
				"executor": datasourceschema.ListNestedAttribute{
					Computed:            true,
					MarkdownDescription: "Subject whose SCM credentials are used to run prebuilds.",
					NestedObject: datasourceschema.NestedAttributeObject{
						Attributes: map[string]datasourceschema.Attribute{
							"id": datasourceschema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Executor subject ID.",
							},
							"principal": datasourceschema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Executor principal type.",
							},
						},
					},
				},
			},
		},
	}
}

func projectDataSourceSubjectAttribute(description string) datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subject ID.",
			},
			"principal": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Subject principal type.",
			},
		},
	}
}
