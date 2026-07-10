// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package warmpool

import (
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func warmPoolResourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona warm pool. A warm pool keeps pre-created environments available for one project and environment class. The project must have prebuilds enabled for the environment class before the warm pool can be created.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Warm pool ID. Use this value as the Terraform import ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project ID this warm pool belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_class_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Environment class ID whose instances are warmed.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"min_size": resourceschema.Int32Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(defaultWarmPoolMinSize),
				MarkdownDescription: "Minimum number of warm instances to maintain. Must be between 0 and 20 and no larger than `max_size`.",
			},
			"max_size": resourceschema.Int32Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(defaultWarmPoolMaxSize),
				MarkdownDescription: "Maximum number of warm instances to maintain. Must be between 1 and 20 and no smaller than `min_size`.",
			},
			"organization_id": computedResourceString("Organization ID that owns the warm pool."),
			"runner_id":       computedResourceString("Runner ID that manages this warm pool."),
			"created_at":      computedResourceString("Time when the warm pool was created."),
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the warm pool was last updated.",
			},
			"snapshot_id": computedResourceString("Prebuild snapshot ID currently assigned to this warm pool."),
		},
	}
}

func warmPoolDataSourceSchema() datasourceschema.Schema {
	attributes := warmPoolDataSourceAttributes(datasourceschema.StringAttribute{
		Required:            true,
		MarkdownDescription: "Warm pool ID.",
	})

	return datasourceschema.Schema{
		MarkdownDescription: "Fetches an Ona warm pool by ID.",
		Attributes:          attributes,
	}
}

func warmPoolCollectionDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Fetches Ona warm pools.",
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform data source ID.",
			},
			"project_ids": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Project IDs to filter warm pools by.",
			},
			"environment_class_ids": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Environment class IDs to filter warm pools by.",
			},
			"page_size": datasourceschema.Int32Attribute{
				Optional:            true,
				MarkdownDescription: "API page size to use while listing warm pools. Defaults to 100.",
			},
			"warm_pools": datasourceschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Ona warm pools matching the filters.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: warmPoolDataSourceAttributes(datasourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Warm pool ID.",
					}),
				},
			},
		},
	}
}

func warmPoolDataSourceAttributes(warmPoolID datasourceschema.StringAttribute) map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"id":                   computedDataSourceString("Terraform data source ID. This is the same value as `warm_pool_id`."),
		"warm_pool_id":         warmPoolID,
		"project_id":           computedDataSourceString("Project ID this warm pool belongs to."),
		"environment_class_id": computedDataSourceString("Environment class ID whose instances are warmed."),
		"min_size":             computedDataSourceInt32("Minimum number of warm instances to maintain."),
		"max_size":             computedDataSourceInt32("Maximum number of warm instances to maintain."),
		"organization_id":      computedDataSourceString("Organization ID that owns the warm pool."),
		"runner_id":            computedDataSourceString("Runner ID that manages this warm pool."),
		"created_at":           computedDataSourceString("Time when the warm pool was created."),
		"updated_at":           computedDataSourceString("Time when the warm pool was last updated."),
		"snapshot_id":          computedDataSourceString("Prebuild snapshot ID currently assigned to this warm pool."),
	}
}

func computedResourceString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

func computedDataSourceString(description string) datasourceschema.StringAttribute {
	return datasourceschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: description,
	}
}

func computedDataSourceInt32(description string) datasourceschema.Int32Attribute {
	return datasourceschema.Int32Attribute{
		Computed:            true,
		MarkdownDescription: description,
	}
}
