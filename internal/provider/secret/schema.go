// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona secret. The plaintext `value` is write-only and is never stored in Terraform plan or state. Rotate a secret by changing both `value` and `value_version`.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Secret ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"scope": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Secret scope. Supported values are `organization`, `project`, `user`, and `service_account`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization ID inferred from the authenticated provider identity for organization-scoped secrets.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project ID. Required when `scope` is `project`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "User ID. When `scope` is `user` and this is omitted, Ona infers the authenticated user identity.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Service account ID. Required when `scope` is `service_account`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Secret name. Must be 3 to 127 characters and contain only letters, digits, and underscores.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": resourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
				MarkdownDescription: "Plaintext secret value. This write-only value is sent to Ona but is not stored in Terraform plan or state. Required on create and when changing `value_version`.",
			},
			"value_version": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User-managed rotation marker for resubmitting `value`. Changing `value` alone does not rotate the secret; change this marker with the value.",
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the secret was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the secret was last updated.",
			},
			"creator": resourceschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created the secret.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
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
			},
			"environment_variable": resourceschema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Set to `true` to mount this secret as an environment variable named after `name`. This is one of the mutually exclusive mount modes.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"file_path": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Absolute file path where the secret is mounted. This is one of the mutually exclusive mount modes.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"container_registry_basic_auth_host": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Container registry host for Docker basic authentication. The `value` should match the Ona API payload; use `base64encode(\"${var.registry_username}:${var.registry_password}\")` for username/password credentials.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"api_only": resourceschema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Set to `true` for a secret available only to API/programmatic consumers. It is not automatically injected into environments. This is one of the mutually exclusive mount modes.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]resourceschema.Block{
			"credential_proxy": resourceschema.ListNestedBlock{
				MarkdownDescription: "Credential proxy injection settings. Set no more than one block. Changes require replacement.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: resourceschema.NestedBlockObject{
					Attributes: map[string]resourceschema.Attribute{
						"target_hosts": resourceschema.SetAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Target hosts to intercept, such as `github.com` or `*.github.com`. Empty entries are ignored when constructing the API request.",
							PlanModifiers: []planmodifier.Set{
								setplanmodifier.RequiresReplace(),
							},
						},
						"header": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "HTTP header name to inject.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
			},
		},
	}
}
