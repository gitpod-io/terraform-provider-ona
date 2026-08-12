// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ list.ListResource = &Resource{}

func NewListResource() list.ListResource {
	return &Resource{}
}

type listModel struct {
	Search        types.String `tfsdk:"search"`
	Command       types.String `tfsdk:"command"`
	CommandPrefix types.String `tfsdk:"command_prefix"`
}

func (r *Resource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		MarkdownDescription: "Lists importable organization-level Ona skills. Template/skill hybrids are excluded because `ona_skill` cannot manage them.",
		Attributes: map[string]listschema.Attribute{
			"search": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Case-insensitive search across skill name, description, and command.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 256),
				},
			},
			"command": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Exact slash-command name to match.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 50),
					stringvalidator.RegexMatches(commandPattern, "command must contain only ASCII letters, digits, underscores, and hyphens"),
					stringvalidator.ConflictsWith(path.MatchRoot("command_prefix")),
				},
			},
			"command_prefix": listschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Slash-command prefix to match.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 50),
					stringvalidator.RegexMatches(commandPattern, "command_prefix must contain only ASCII letters, digits, underscores, and hyphens"),
					stringvalidator.ConflictsWith(path.MatchRoot("command")),
				},
			},
		},
	}
}

func (r *Resource) List(ctx context.Context, req list.ListRequest, resp *list.ListResultsStream) {
	resp.Results = func(push func(list.ListResult) bool) {
		if r.client == nil {
			push(listutil.Error("Ona API Client Is Not Configured", fmt.Errorf("set the provider token argument or ONA_TOKEN before listing ona_skill resources")))
			return
		}
		var config listModel
		if !listutil.PushDiagnostics(push, req.Config.Get(ctx, &config)) {
			return
		}
		configDiags := validateListConfig(config)
		if !listutil.PushDiagnostics(push, configDiags) {
			return
		}
		organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, r.client)
		if err != nil {
			var diags diag.Diagnostics
			providerdiag.AddAPIError(&diags, "Unable to Resolve Ona Organization", "getting the authenticated organization while listing ona_skill resources", err)
			push(list.ListResult{Diagnostics: diags})
			return
		}

		var token string
		var emitted int64
		for listutil.HasCapacity(req.Limit, emitted) {
			apiReq := listPromptsRequest(config, req.IncludeResource, listutil.PageSize(req.Limit, emitted), token)
			result, err := r.client.AgentService().ListPrompts(ctx, connect.NewRequest(apiReq))
			if err != nil {
				push(listutil.Error("Unable to List Ona Skills", fmt.Errorf("list prompts: %w", err)))
				return
			}
			if result == nil || result.Msg == nil {
				push(listutil.Error("Unable to List Ona Skills", fmt.Errorf("the Ona API returned an empty response")))
				return
			}
			for _, remote := range result.Msg.GetPrompts() {
				if !listutil.HasCapacity(req.Limit, emitted) {
					return
				}
				if remote.GetSpec() != nil && remote.GetSpec().GetIsSkill() && remote.GetSpec().GetIsTemplate() {
					continue
				}
				model, mappingDiags := modelFromPrompt(remote, remote.GetId(), organizationID, req.IncludeResource)
				if mappingDiags.HasError() {
					push(list.ListResult{Diagnostics: mappingDiags})
					return
				}
				item := req.NewListResult(ctx)
				item.DisplayName = skillDisplayName(remote)
				item.Diagnostics.Append(item.Identity.Set(ctx, IdentityModel{SkillID: model.ID})...)
				if req.IncludeResource && !item.Diagnostics.HasError() {
					item.Diagnostics.Append(item.Resource.Set(ctx, &model)...)
				}
				if !push(item) || item.Diagnostics.HasError() {
					return
				}
				emitted++
			}
			nextToken := result.Msg.GetPagination().GetNextToken()
			if nextToken == "" {
				return
			}
			if nextToken == token {
				push(listutil.Error("Unable to List Ona Skills", fmt.Errorf("the Ona API returned the same pagination token twice")))
				return
			}
			token = nextToken
		}
	}
}

func listPromptsRequest(config listModel, includeResource bool, pageSize int32, token string) *v1.ListPromptsRequest {
	filter := &v1.ListPromptsRequest_Filter{
		IsSkill:              true,
		ExcludePromptContent: !includeResource,
	}
	if !config.Search.IsNull() && !config.Search.IsUnknown() {
		filter.Search = config.Search.ValueString()
	}
	if !config.Command.IsNull() && !config.Command.IsUnknown() {
		filter.Command = config.Command.ValueString()
		filter.IsCommand = true
	}
	if !config.CommandPrefix.IsNull() && !config.CommandPrefix.IsUnknown() {
		filter.CommandPrefix = config.CommandPrefix.ValueString()
		filter.IsCommand = true
	}
	return &v1.ListPromptsRequest{
		Pagination: &v1.PaginationRequest{PageSize: pageSize, Token: token},
		Filter:     filter,
	}
}

func validateListConfig(config listModel) diag.Diagnostics {
	var diags diag.Diagnostics
	validateOptionalListString(config.Search, path.Root("search"), "Skill Search", 1, 256, false, &diags)
	validateOptionalListString(config.Command, path.Root("command"), "Skill Command", 1, 50, true, &diags)
	validateOptionalListString(config.CommandPrefix, path.Root("command_prefix"), "Skill Command Prefix", 1, 50, true, &diags)
	if !config.Command.IsNull() && !config.Command.IsUnknown() && !config.CommandPrefix.IsNull() && !config.CommandPrefix.IsUnknown() {
		diags.AddError("Conflicting Skill Command Filters", "Configure only one of command or command_prefix.")
	}
	return diags
}

func validateOptionalListString(value types.String, attrPath path.Path, label string, minLength, maxLength int, validateCommand bool, diags *diag.Diagnostics) {
	if value.IsNull() {
		return
	}
	if value.IsUnknown() {
		diags.AddAttributeError(attrPath, "Unknown "+label, fmt.Sprintf("%s must be known before querying skills.", attrPath.String()))
		return
	}
	validateStringValue(value.ValueString(), attrPath, label, minLength, maxLength, diags)
	if validateCommand && !commandPattern.MatchString(value.ValueString()) {
		diags.AddAttributeError(attrPath, "Invalid "+label, fmt.Sprintf("%s must contain only ASCII letters, digits, underscores, and hyphens.", attrPath.String()))
	}
}

var displayNameInvalidCharacters = regexp.MustCompile(`[^a-z0-9_]+`)

func skillDisplayName(remote *v1.Prompt) string {
	name := ""
	if remote != nil && remote.GetMetadata() != nil {
		name = strings.ToLower(strings.TrimSpace(remote.GetMetadata().GetName()))
	}
	name = displayNameInvalidCharacters.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" || name[0] >= '0' && name[0] <= '9' {
		name = "skill_" + name
	}
	id := "unknown"
	if remote != nil && remote.GetId() != "" {
		id = strings.ReplaceAll(remote.GetId(), "-", "_")
	}
	return strings.TrimSuffix(name, "_") + "_" + id
}
