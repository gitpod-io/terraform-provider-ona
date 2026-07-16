// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package workflow

import (
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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
		MarkdownDescription: "Persistent Ona workflow automation. Creating workflows requires a permitted user credential; the Ona API rejects workflow creation by service accounts. A caller changing workflow triggers or actions must own the current user executor or set the executor to themselves or a service account. Reports, report steps, legacy pull-request triggers without a webhook or integration, and workflow-level agent/Codex settings cannot be imported or managed. Removing this resource uses graceful deletion: Ona immediately deletes idle workflows, but cancels active executions and finishes their cleanup asynchronously.",
		Attributes: map[string]resourceschema.Attribute{
			"id": stableComputedString("Workflow ID. Use this value as the Terraform import ID."),
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Workflow display name. Must be between 1 and 80 characters.",
			},
			"description": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional workflow description. Must not exceed 500 characters. Set an empty string to clear it.",
			},
			"triggers": resourceschema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Ordered workflow triggers. Configure between 1 and 10 entries.",
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: triggerResourceAttributes(),
				},
			},
			"action": resourceschema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Workflow action and its ordered execution steps.",
				Attributes:          actionResourceAttributes(),
			},
			"executor": resourceschema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Identity that executes the workflow. Omit to use the creating user. A user executor must be the API caller; a service-account executor may be selected by ID. Removing this block retains the resolved remote executor because the API has no clear operation.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]resourceschema.Attribute{
					"id": resourceschema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Executor UUID.",
					},
					"principal": resourceschema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Executor principal type. Supported values are `user` and `service_account`.",
					},
				},
			},
			"disabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether automatic and manual workflow starts are disabled. Defaults to `false`.",
			},
			"webhook_url": stableComputedString("Generated workflow webhook URL. The signing secret is not read or stored."),
			"creator":     computedSubjectResourceAttribute("Identity that created the workflow."),
			"created_at":  stableComputedString("Time when the workflow was created, in RFC 3339 format."),
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the workflow was last updated, in RFC 3339 format.",
			},
		},
	}
}

func triggerResourceAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"manual": resourceschema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Manual trigger.",
			Attributes:          map[string]resourceschema.Attribute{},
		},
		"time": resourceschema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Time-based trigger.",
			Attributes: map[string]resourceschema.Attribute{
				"cron_expression": resourceschema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Five-field cron expression or supported cron descriptor. Must be between 1 and 100 characters.",
				},
			},
		},
		"pull_request": resourceschema.SingleNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Pull-request trigger. Configure at least one event and at least one webhook or integration ID.",
			Attributes: map[string]resourceschema.Attribute{
				"events": resourceschema.SetAttribute{
					Required:            true,
					ElementType:         types.StringType,
					MarkdownDescription: "Pull-request events: `opened`, `updated`, `approved`, `merged`, `closed`, `ready_for_review`, or `review_requested`.",
				},
				"webhook_id": resourceschema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "UUID of a reusable webhook that activates this trigger.",
				},
				"integration_id": resourceschema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "UUID of an integration that supplies events to this trigger.",
				},
			},
		},
		"context": resourceschema.SingleNestedAttribute{
			Required:            true,
			MarkdownDescription: "Execution context. Configure exactly one context variant.",
			Attributes: map[string]resourceschema.Attribute{
				"projects": resourceschema.SingleNestedAttribute{
					Optional:            true,
					MarkdownDescription: "Project environments in which the workflow runs.",
					Attributes: map[string]resourceschema.Attribute{
						"project_ids": resourceschema.SetAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Project UUIDs. The API accepts up to 500 entries.",
						},
					},
				},
				"repositories": resourceschema.SingleNestedAttribute{
					Optional:            true,
					MarkdownDescription: "Repository environments in which the workflow runs.",
					Attributes: map[string]resourceschema.Attribute{
						"repository_urls": resourceschema.SetAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Explicit repository URLs. Configure 1 to 500 values, or use repository_selector.",
						},
						"repository_selector": resourceschema.SingleNestedAttribute{
							Optional:            true,
							MarkdownDescription: "SCM repository search selector. Do not combine with repository_urls.",
							Attributes: map[string]resourceschema.Attribute{
								"repo_search_string": resourceschema.StringAttribute{
									Required:            true,
									MarkdownDescription: "SCM-specific repository search string.",
								},
								"scm_host": resourceschema.StringAttribute{
									Required:            true,
									MarkdownDescription: "SCM host, such as `github.com` or `gitlab.com`.",
								},
							},
						},
						"environment_class_id": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Environment class UUID used for repository environments.",
						},
					},
				},
				"agent": resourceschema.SingleNestedAttribute{
					Optional:            true,
					MarkdownDescription: "Agent-managed execution context.",
					Attributes: map[string]resourceschema.Attribute{
						"prompt": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Agent context prompt. Must be between 1 and 20,000 characters.",
						},
					},
				},
				"from_trigger": resourceschema.SingleNestedAttribute{
					Optional:            true,
					MarkdownDescription: "Use context from the pull-request event. Valid only for pull_request triggers.",
					Attributes:          map[string]resourceschema.Attribute{},
				},
			},
		},
	}
}

func actionResourceAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"limits": resourceschema.SingleNestedAttribute{
			Required:            true,
			MarkdownDescription: "Action execution limits. Organization tiering may impose lower server-side caps.",
			Attributes: map[string]resourceschema.Attribute{
				"max_parallel": resourceschema.Int32Attribute{
					Required:            true,
					MarkdownDescription: "Maximum concurrent actions, from 1 through 25. Must not exceed max_total.",
				},
				"max_total": resourceschema.Int32Attribute{
					Required:            true,
					MarkdownDescription: "Maximum total actions, from 1 through 100.",
				},
				"max_time": resourceschema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Optional maximum time per execution action, as a Go duration string such as `30m` or `2h`.",
					PlanModifiers: []planmodifier.String{
						durationSemanticEqualityModifier{},
					},
				},
			},
		},
		"steps": resourceschema.ListNestedAttribute{
			Required:            true,
			MarkdownDescription: "Ordered action steps. Configure between 1 and 50 entries.",
			NestedObject: resourceschema.NestedAttributeObject{
				Attributes: map[string]resourceschema.Attribute{
					"task": resourceschema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Shell task step.",
						Attributes: map[string]resourceschema.Attribute{
							"command": resourceschema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Command to run. Must be between 1 and 20,000 characters.",
							},
						},
					},
					"agent": resourceschema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Agent prompt step.",
						Attributes: map[string]resourceschema.Attribute{
							"prompt": resourceschema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Prompt for the agent. Must be between 1 and 20,000 characters.",
							},
						},
					},
					"pull_request": resourceschema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Pull-request creation step.",
						Attributes: map[string]resourceschema.Attribute{
							"title": resourceschema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Pull-request title. Must be between 1 and 500 characters.",
							},
							"description": resourceschema.StringAttribute{
								Optional:            true,
								Computed:            true,
								Default:             stringdefault.StaticString(""),
								MarkdownDescription: "Pull-request description. Must not exceed 20,000 characters.",
							},
							"branch": resourceschema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Pull-request branch. Must be between 1 and 255 characters.",
							},
							"draft": resourceschema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								Default:             booldefault.StaticBool(false),
								MarkdownDescription: "Whether to create the pull request as a draft. Defaults to `false`.",
							},
						},
					},
				},
			},
		},
	}
}

func collectionDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Lists persistent Ona automations visible to the configured provider token. System-managed automations, such as Insights automations, are excluded by the Ona API.",
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform data source ID. Always `automations`.",
			},
			"automation_ids": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Automation UUIDs to include. The API accepts at most 25 values.",
			},
			"search": datasourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Case-insensitive search across automation name, description, and ID. Must not exceed 256 characters.",
			},
			"creator_ids": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Creator user UUIDs to include. The API accepts at most 25 values.",
			},
			"status_phases": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Latest execution phases to include: `pending`, `running`, `stopping`, `stopped`, `deleting`, `deleted`, or `completed`. Do not combine with has_failed_execution_since.",
			},
			"has_failed_execution_since": datasourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "RFC 3339 timestamp. Includes automations with a failed execution at or after this time. Do not combine with status_phases.",
			},
			"disabled": datasourceschema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by disabled state. Omit to include enabled and disabled automations.",
			},
			"automations": datasourceschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching automation summaries sorted by automation ID.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: summaryDataSourceAttributes(),
				},
			},
		},
	}
}

func summaryDataSourceAttributes() map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"id":          computedDataSourceString("Automation ID."),
		"name":        computedDataSourceString("Automation display name."),
		"description": computedDataSourceString("Automation description."),
		"disabled": datasourceschema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether automation starts are disabled.",
		},
		"deleting": datasourceschema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Whether graceful deletion is in progress.",
		},
		"executor":    computedSubjectDataSourceAttribute("Identity that executes the automation."),
		"creator":     computedSubjectDataSourceAttribute("Identity that created the automation."),
		"created_at":  computedDataSourceString("Time when the automation was created, in RFC 3339 format."),
		"updated_at":  computedDataSourceString("Time when the automation was last updated, in RFC 3339 format."),
		"webhook_url": computedDataSourceString("Generated automation webhook URL."),
	}
}

func stableComputedString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

func computedSubjectResourceAttribute(description string) resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]resourceschema.Attribute{
			"id":        stableComputedString("Subject UUID."),
			"principal": stableComputedString("Subject principal type."),
		},
	}
}

func computedSubjectDataSourceAttribute(description string) datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		Attributes: map[string]datasourceschema.Attribute{
			"id":        computedDataSourceString("Subject UUID."),
			"principal": computedDataSourceString("Subject principal type."),
		},
	}
}

func computedDataSourceString(description string) datasourceschema.StringAttribute {
	return datasourceschema.StringAttribute{Computed: true, MarkdownDescription: description}
}
