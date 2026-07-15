// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package integration

import (
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() resourceschema.Schema {
	return resourceschema.Schema{
		MarkdownDescription: "Ona organization integration. Use a definition ID from `ona_integration_definitions` for a built-in integration, or omit it to configure a custom MCP integration. Integration writes require organization integration permissions. Removing this resource deletes the remote integration.",
		Attributes: map[string]resourceschema.Attribute{
			"id":              stableComputedString("Integration ID. Use this value as the Terraform import ID."),
			"organization_id": stableComputedString("Organization ID that owns the integration."),
			"integration_definition_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Global integration definition ID. Omit this value for a custom MCP integration. Changing it replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Runner ID to which this integration is restricted. Changing it replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the integration is enabled. Defaults to `false`.",
			},
			"capabilities": resourceCapabilitiesAttribute(),
			"auth":         resourceAuthAttribute(),
			"credentials":  resourceCredentialsAttribute(),
			"host": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Integration host. Definition-backed integrations inherit it when omitted; custom integrations derive it from the MCP URL when omitted. Changing it replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Integration display name. Required for custom integrations and inherited for definition-backed integrations. Changing it replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Custom integration description. Definition-backed integrations inherit their description. Changing it replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"icon_url": stableComputedString("Integration icon URL resolved from the selected definition, when present."),
			"categories": resourceschema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Integration categories. Supported values are `source_control`, `communication`, `project_management`, `observability`, `data_analytics`, `knowledge`, `mcp`, `automation_triggers`, and `ai`. Definition-backed integrations inherit categories when omitted. Clearing custom categories replaces the integration; clearing definition-backed categories is not representable and returns a plan diagnostic.",
			},
			"external_installation": resourceschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Provider-side app installation associated with this integration, when known.",
				Attributes: map[string]resourceschema.Attribute{
					"id":           stableComputedString("Provider-assigned installation ID."),
					"account_name": stableComputedString("Provider account or organization name."),
					"account_type": stableComputedString("Provider account type."),
				},
			},
		},
	}
}

func resourceCapabilitiesAttribute() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: "Effective integration capabilities. Omitted values may be inherited from the selected definition. Capabilities are immutable for custom integrations.",
		Attributes: map[string]resourceschema.Attribute{
			"mcp": resourceschema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Model Context Protocol capability.",
				Attributes: map[string]resourceschema.Attribute{
					"url": resourceschema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Remote MCP server URL. Custom integrations require a public HTTPS URL.",
					},
				},
			},
			"context_parsing":    markerResourceAttribute("Context parsing capability."),
			"source_code_access": markerResourceAttribute("Source code access capability."),
			"login":              markerResourceAttribute("Login capability."),
			"agent_client": resourceschema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Agent-client capability.",
				Attributes: map[string]resourceschema.Attribute{
					"severity_threshold": resourceschema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Minimum incident severity that triggers an agent session. Supported values are `SEV1`, `SEV2`, and `SEV3`.",
					},
					"default_project_id": resourceschema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Default Ona project ID when no project can be resolved from event context.",
					},
				},
			},
			"scm_pr_events": markerResourceAttribute("SCM pull-request event capability."),
		},
	}
}

func markerResourceAttribute(description string) resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: description,
		Attributes:          map[string]resourceschema.Attribute{},
	}
}

func resourceAuthAttribute() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: "Effective authentication configuration. Omitted values may be inherited from the selected definition. Authentication is immutable for custom integrations.",
		Attributes: map[string]resourceschema.Attribute{
			"requires_auth": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the integration requires authentication. Definition-backed integrations inherit this value and cannot override it.",
			},
			"api_key": markerResourceAttribute("API-key authentication support marker. This block does not contain the key value."),
			"oauth": resourceschema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "OAuth configuration.",
				Attributes:          resourceOAuthAttributes(),
			},
			"proprietary_app": resourceschema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Provider-specific application configuration.",
				Attributes:          resourceProprietaryAppAttributes(),
			},
		},
	}
}

func resourceOAuthAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"auth_url":              optionalComputedResourceString("OAuth authorization endpoint URL."),
		"token_url":             optionalComputedResourceString("OAuth token endpoint URL."),
		"scopes":                optionalComputedResourceStringSet("OAuth scopes."),
		"client_id":             optionalComputedResourceString("OAuth client ID."),
		"client_secret_version": optionalResourceString("User-managed marker for resubmitting or rotating `client_secret`."),
		"redirect_url":          optionalComputedResourceString("OAuth redirect URL."),
		"dynamic_registration": resourceschema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Whether RFC 7591 dynamic client registration is enabled.",
		},
		"auth_params": optionalComputedResourceStringMap("Additional OAuth authorization query parameters."),
	}
}

func resourceProprietaryAppAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"client_id":              optionalComputedResourceString("Provider application client ID."),
		"client_secret_version":  optionalResourceString("User-managed marker for resubmitting or rotating `client_secret`."),
		"webhook_secret_version": optionalResourceString("User-managed marker for resubmitting or rotating `webhook_secret`."),
		"auth_params":            optionalComputedResourceStringMap("Additional provider authorization parameters."),
		"app_scopes":             optionalComputedResourceStringSet("Provider application OAuth scopes."),
		"token_url":              optionalComputedResourceString("Provider application token endpoint URL."),
		"app_id":                 optionalComputedResourceString("Provider-assigned application ID."),
		"private_key_version":    optionalResourceString("User-managed marker for resubmitting or rotating `private_key`."),
		"app_slug":               optionalComputedResourceString("Provider-assigned application slug."),
		"api_key_version":        optionalResourceString("User-managed marker for resubmitting or rotating `api_key`."),
	}
}

func resourceCredentialsAttribute() resourceschema.SingleNestedAttribute {
	return resourceschema.SingleNestedAttribute{
		Optional:            true,
		Sensitive:           true,
		WriteOnly:           true,
		MarkdownDescription: "Write-only integration credentials. Values are sent to Ona but are never stored in Terraform plan or state. Pair each value with its version marker under `auth` to rotate it intentionally.",
		Attributes: map[string]resourceschema.Attribute{
			"oauth_client_secret":        credentialResourceString("OAuth client secret."),
			"proprietary_client_secret":  credentialResourceString("Provider application client secret."),
			"proprietary_webhook_secret": credentialResourceString("Secret used to verify provider webhook signatures."),
			"proprietary_private_key":    credentialResourceString("PEM-encoded private key for application authentication."),
			"proprietary_api_key":        credentialResourceString("Provider API key used for outbound updates."),
		},
	}
}

func definitionsDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{
		MarkdownDescription: "Fetches all integration definitions visible to the configured provider token. Definitions are global templates managed by Ona; Terraform can discover but not mutate them. Experimental definitions are filtered by backend visibility rules.",
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform data source ID. Always `integration_definitions`.",
			},
			"definitions": datasourceschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Visible integration definitions sorted by definition ID. Credential values are omitted.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: definitionDataSourceAttributes(),
				},
			},
		},
	}
}

func definitionDataSourceAttributes() map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"id":           computedDataSourceString("Integration definition ID."),
		"name":         computedDataSourceString("Integration definition name."),
		"description":  computedDataSourceString("Integration definition description."),
		"icon_url":     computedDataSourceString("Integration definition icon URL."),
		"host":         computedDataSourceString("Integration host."),
		"experimental": computedDataSourceBool("Whether the definition is experimental."),
		"categories": datasourceschema.SetAttribute{
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: "Integration categories.",
		},
		"capabilities": dataSourceCapabilitiesAttribute(),
		"auth":         dataSourceAuthAttribute(),
	}
}

func dataSourceCapabilitiesAttribute() datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Capabilities supported by this definition.",
		Attributes: map[string]datasourceschema.Attribute{
			"mcp": datasourceschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]datasourceschema.Attribute{
					"url": computedDataSourceString("Remote MCP server URL."),
				},
			},
			"context_parsing":    markerDataSourceAttribute("Whether context parsing is supported."),
			"source_code_access": markerDataSourceAttribute("Whether source code access is supported."),
			"login":              markerDataSourceAttribute("Whether login is supported."),
			"agent_client": datasourceschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]datasourceschema.Attribute{
					"severity_threshold": computedDataSourceString("Minimum incident severity."),
					"default_project_id": computedDataSourceString("Default Ona project ID."),
				},
			},
			"scm_pr_events": markerDataSourceAttribute("Whether SCM pull-request events are supported."),
		},
	}
}

func markerDataSourceAttribute(description string) datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: description,
		Attributes:          map[string]datasourceschema.Attribute{},
	}
}

func dataSourceAuthAttribute() datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Non-secret authentication metadata. Credential values are never returned.",
		Attributes: map[string]datasourceschema.Attribute{
			"requires_auth": computedDataSourceBool("Whether authentication is required."),
			"api_key":       markerDataSourceAttribute("Whether API-key authentication is supported."),
			"oauth": datasourceschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]datasourceschema.Attribute{
					"auth_url":             computedDataSourceString("OAuth authorization endpoint URL."),
					"token_url":            computedDataSourceString("OAuth token endpoint URL."),
					"scopes":               computedDataSourceStringSet("OAuth scopes."),
					"client_id":            computedDataSourceString("OAuth client ID."),
					"redirect_url":         computedDataSourceString("OAuth redirect URL."),
					"dynamic_registration": computedDataSourceBool("Whether dynamic client registration is enabled."),
					"auth_params":          computedDataSourceStringMap("Additional OAuth authorization parameters."),
				},
			},
			"proprietary_app": datasourceschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]datasourceschema.Attribute{
					"client_id":   computedDataSourceString("Provider application client ID."),
					"auth_params": computedDataSourceStringMap("Additional provider authorization parameters."),
					"app_scopes":  computedDataSourceStringSet("Provider application scopes."),
					"token_url":   computedDataSourceString("Provider application token endpoint URL."),
					"app_id":      computedDataSourceString("Provider-assigned application ID."),
					"app_slug":    computedDataSourceString("Provider-assigned application slug."),
				},
			},
		},
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

func optionalComputedResourceString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: description}
}

func optionalResourceString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{Optional: true, MarkdownDescription: description}
}

func credentialResourceString(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Optional:            true,
		Sensitive:           true,
		WriteOnly:           true,
		MarkdownDescription: description,
	}
}

func optionalComputedResourceStringSet(description string) resourceschema.SetAttribute {
	return resourceschema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, MarkdownDescription: description}
}

func optionalComputedResourceStringMap(description string) resourceschema.MapAttribute {
	return resourceschema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, MarkdownDescription: description}
}

func computedDataSourceString(description string) datasourceschema.StringAttribute {
	return datasourceschema.StringAttribute{Computed: true, MarkdownDescription: description}
}

func computedDataSourceBool(description string) datasourceschema.BoolAttribute {
	return datasourceschema.BoolAttribute{Computed: true, MarkdownDescription: description}
}

func computedDataSourceStringSet(description string) datasourceschema.SetAttribute {
	return datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: description}
}

func computedDataSourceStringMap(description string) datasourceschema.MapAttribute {
	return datasourceschema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: description}
}
