// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func llmIntegrationResourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona runner LLM integration. Use this resource to configure BYOK access for LLM models on a customer-managed runner.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "LLM integration ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID this LLM integration belongs to. Changing this value replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"models": resourceschema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Supported LLM models for this integration. Supported values include `sonnet_3_7`, `sonnet_4`, `sonnet_4_extended`, `opus_4`, `openai_4o`, and `openai_auto`.",
			},
			"endpoint": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "LLM provider endpoint URL. The value must not include leading or trailing whitespace.",
			},
			"api_key": resourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
				MarkdownDescription: "LLM provider API key. This write-only value is sent to Ona but is not stored in Terraform plan or state. Required when creating an integration or rotating the API key.",
			},
			"api_key_version": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User-managed version marker for resubmitting `api_key` during rotation. Increment or otherwise change this value when supplying a new API key.",
			},
			"max_tokens": resourceschema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
				MarkdownDescription: "Maximum number of tokens a single LLM provider request may generate. Set to `0` to use the model default.",
			},
			"enabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the LLM integration is enabled. Defaults to `true`.",
			},
			"phase": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "LLM integration phase reported by Ona.",
			},
			"phase_reason": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Reason for the current LLM integration phase, when reported by Ona.",
			},
			"llm_provider": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "LLM provider family inferred by Ona from the configured models. Supported values are `anthropic` and `openai`.",
			},
		},
	}
}
