package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	onaclient "github.com/gitpod-io/terraform-provider-ona/internal/client"
)

func collect(ctx context.Context, api *onaclient.Client, cfg config) (snapshot, error) {
	orgID := strings.TrimSpace(cfg.orgID)
	if orgID == "" {
		logStepf("resolving organization from authenticated identity")
		resp, err := api.GetAuthenticatedIdentity(ctx)
		if err != nil {
			return snapshot{}, fmt.Errorf("get authenticated identity: %w", err)
		}
		orgID = resp.OrganizationID
	}
	if orgID == "" {
		return snapshot{}, fmt.Errorf("organization ID is empty")
	}
	cfg.orgID = orgID
	logStepf("using organization %s", orgID)

	logStepf("listing groups")
	groups, err := listGroups(ctx, api, orgID, cfg.includeSystemGroups)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d groups", len(groups))

	logStepf("listing group memberships")
	groupMemberships, err := listGroupMemberships(ctx, api, groups)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d group memberships", countGroupMemberships(groupMemberships))

	logStepf("listing teams")
	teams, err := listTeams(ctx, api, orgID)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d teams", len(teams))

	logStepf("listing security policies")
	securityPolicies, err := listSecurityPolicies(ctx, api, orgID)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d security policies", len(securityPolicies))

	logStepf("listing runners")
	runners, err := listRunners(ctx, api)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d runners", len(runners))

	logStepf("listing environment classes")
	environmentClasses, err := listEnvironmentClasses(ctx, api, runners)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d environment classes", len(environmentClasses))

	logStepf("listing projects")
	projects, err := listProjects(ctx, api, orgID)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d projects", len(projects))
	logStepf("listing project environment classes")
	if err := addProjectEnvironmentClasses(ctx, api, projects); err != nil {
		return snapshot{}, err
	}

	logStepf("listing service accounts")
	serviceAccounts, err := listServiceAccounts(ctx, cfg)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d service accounts", len(serviceAccounts))

	logStepf("listing automations")
	workflows, err := listWorkflows(ctx, api)
	if err != nil {
		return snapshot{}, err
	}
	logStepf("found %d automations", len(workflows))

	logStepf("reading organization policies")
	organizationPolicy, err := getOrganizationPolicies(ctx, api, orgID)
	if err != nil {
		return snapshot{}, err
	}

	return snapshot{
		orgID:              orgID,
		groups:             groups,
		groupMemberships:   groupMemberships,
		teams:              teams,
		securityPolicies:   securityPolicies,
		runners:            runners,
		environmentClasses: environmentClasses,
		projects:           projects,
		serviceAccounts:    serviceAccounts,
		workflows:          workflows,
		organizationPolicy: organizationPolicy,
	}, nil
}

func collectExternalReferences(ctx context.Context, api *onaclient.Client, cfg config, inv inventory) (inventory, error) {
	cfg.orgID = inv.OrganizationID
	logStepf("augmenting existing import map with referenced objects")
	runnerIDs := runnerIDsFromInventory(inv)
	environmentClasses, err := listEnvironmentClassesForRunnerIDs(ctx, api, runnerIDs)
	if err != nil {
		return inventory{}, err
	}
	missingEnvironmentClassIDs := missingEnvironmentClassIDs(inv, environmentClasses)
	for _, id := range missingEnvironmentClassIDs {
		environmentClass, err := getEnvironmentClass(ctx, api, id)
		if err != nil {
			return inventory{}, err
		}
		if environmentClass != nil {
			environmentClasses = append(environmentClasses, environmentClass)
		}
	}
	serviceAccounts, err := listServiceAccounts(ctx, cfg)
	if err != nil {
		return inventory{}, err
	}
	s := snapshot{
		orgID:              inv.OrganizationID,
		environmentClasses: environmentClasses,
		serviceAccounts:    serviceAccounts,
	}
	external := buildExternalInventory(s)
	inv.Resources = mergeExternalResources(inv.Resources, external.Resources)
	sortInventory(inv.Resources)
	return inv, nil
}

func listGroups(ctx context.Context, api *onaclient.Client, orgID string, includeSystem bool) ([]*onaclient.Group, error) {
	var result []*onaclient.Group
	err := eachPage(func(token string) (string, error) {
		resp, err := api.ListGroups(ctx, onaclient.ListGroupsRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
		})
		if err != nil {
			return "", fmt.Errorf("list groups: %w", err)
		}
		for _, g := range resp.Groups {
			if g.GetOrganizationId() != orgID {
				continue
			}
			if !includeSystem && (g.GetSystemManaged() || g.GetDirectShare()) {
				continue
			}
			result = append(result, g)
		}
		return resp.Pagination.GetNextToken(), nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].GetName() < result[j].GetName() })
	return result, err
}

func listGroupMemberships(ctx context.Context, api *onaclient.Client, groups []*onaclient.Group) (map[string][]*onaclient.GroupMembership, error) {
	result := map[string][]*onaclient.GroupMembership{}
	for _, group := range groups {
		groupID := group.GetId()
		if groupID == "" {
			continue
		}
		err := eachPage(func(token string) (string, error) {
			resp, err := api.ListMemberships(ctx, onaclient.ListMembershipsRequest{
				Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
				GroupID:    groupID,
			})
			if err != nil {
				return "", fmt.Errorf("list memberships for group %s: %w", groupID, err)
			}
			result[groupID] = append(result[groupID], resp.Members...)
			return resp.Pagination.GetNextToken(), nil
		})
		if err != nil {
			return nil, err
		}
		sort.Slice(result[groupID], func(i, j int) bool {
			return result[groupID][i].GetSubject().GetId() < result[groupID][j].GetSubject().GetId()
		})
	}
	return result, nil
}

func countGroupMemberships(memberships map[string][]*onaclient.GroupMembership) int {
	var count int
	for _, members := range memberships {
		count += len(members)
	}
	return count
}

func listTeams(ctx context.Context, api *onaclient.Client, orgID string) ([]*onaclient.Team, error) {
	var result []*onaclient.Team
	err := eachPage(func(token string) (string, error) {
		resp, err := api.ListTeams(ctx, onaclient.ListTeamsRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
		})
		if err != nil {
			return "", fmt.Errorf("list teams: %w", err)
		}
		for _, t := range resp.Teams {
			if t.GetOrganizationId() == orgID {
				result = append(result, t)
			}
		}
		return resp.Pagination.GetNextToken(), nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].GetName() < result[j].GetName() })
	return result, err
}

func listSecurityPolicies(ctx context.Context, api *onaclient.Client, orgID string) ([]*onaclient.SecurityPolicy, error) {
	var result []*onaclient.SecurityPolicy
	err := eachPage(func(token string) (string, error) {
		resp, err := api.ListSecurityPolicies(ctx, onaclient.ListSecurityPoliciesRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
			Filter:     &onaclient.ListSecurityPoliciesFilter{OrganizationID: orgID},
		})
		if err != nil {
			return "", fmt.Errorf("list security policies: %w", err)
		}
		result = append(result, resp.SecurityPolicies...)
		return resp.Pagination.GetNextToken(), nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].GetMetadata().GetName() < result[j].GetMetadata().GetName() })
	return result, err
}

func listRunners(ctx context.Context, api *onaclient.Client) ([]*onaclient.Runner, error) {
	var result []*onaclient.Runner
	err := eachPage(func(token string) (string, error) {
		resp, err := api.ListRunners(ctx, onaclient.ListRunnersRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
		})
		if err != nil {
			return "", fmt.Errorf("list runners: %w", err)
		}
		result = append(result, resp.Runners...)
		return resp.Pagination.GetNextToken(), nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].GetName() < result[j].GetName() })
	return result, err
}

func listEnvironmentClasses(ctx context.Context, api *onaclient.Client, runners []*onaclient.Runner) ([]*onaclient.EnvironmentClass, error) {
	runnerIDs := make([]string, 0, len(runners))
	for _, r := range runners {
		if r.GetRunnerId() != "" {
			runnerIDs = append(runnerIDs, r.GetRunnerId())
		}
	}
	return listEnvironmentClassesForRunnerIDs(ctx, api, runnerIDs)
}

func listEnvironmentClassesForRunnerIDs(ctx context.Context, api *onaclient.Client, runnerIDs []string) ([]*onaclient.EnvironmentClass, error) {
	var result []*onaclient.EnvironmentClass
	for _, batch := range chunks(runnerIDs, 25) {
		err := eachPage(func(token string) (string, error) {
			resp, err := api.ListEnvironmentClasses(ctx, onaclient.ListEnvironmentClassesRequest{
				Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
				Filter:     &onaclient.ListEnvironmentClassesFilter{RunnerIDs: batch},
			})
			if err != nil {
				return "", fmt.Errorf("list environment classes: %w", err)
			}
			result = append(result, resp.EnvironmentClasses...)
			return resp.Pagination.GetNextToken(), nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GetDisplayName() < result[j].GetDisplayName() })
	return result, nil
}

func getEnvironmentClass(ctx context.Context, api *onaclient.Client, id string) (*onaclient.EnvironmentClass, error) {
	resp, err := api.GetEnvironmentClass(ctx, onaclient.GetEnvironmentClassRequest{
		EnvironmentClassID: id,
	})
	if err != nil {
		if onaclient.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get environment class %s: %w", id, err)
	}
	return resp.EnvironmentClass, nil
}

func listProjects(ctx context.Context, api *onaclient.Client, orgID string) ([]*onaclient.Project, error) {
	var result []*onaclient.Project
	err := eachPage(func(token string) (string, error) {
		resp, err := api.ListProjects(ctx, onaclient.ListProjectsRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
		})
		if err != nil {
			return "", fmt.Errorf("list projects: %w", err)
		}
		for _, p := range resp.Projects {
			if p.GetMetadata().GetOrganizationId() == orgID {
				result = append(result, p)
			}
		}
		return resp.Pagination.GetNextToken(), nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].GetMetadata().GetName() < result[j].GetMetadata().GetName() })
	return result, err
}

func addProjectEnvironmentClasses(ctx context.Context, api *onaclient.Client, projects []*onaclient.Project) error {
	for _, project := range projects {
		classes, err := listProjectEnvironmentClasses(ctx, api, project.GetId())
		if err != nil {
			return err
		}
		project.EnvironmentClasses = classes
	}
	return nil
}

func listProjectEnvironmentClasses(ctx context.Context, api *onaclient.Client, projectID string) ([]*onaclient.ProjectEnvironmentClass, error) {
	var result []*onaclient.ProjectEnvironmentClass
	err := eachPage(func(token string) (string, error) {
		resp, err := api.ListProjectEnvironmentClasses(ctx, onaclient.ListProjectEnvironmentClassesRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
			ProjectID:  projectID,
		})
		if err != nil {
			return "", fmt.Errorf("list project environment classes: %w", err)
		}
		result = append(result, resp.ProjectEnvironmentClasses...)
		return resp.Pagination.GetNextToken(), nil
	})
	return result, err
}

func ensureProjectEnvironmentClassReferenceIDs(ctx context.Context, api *onaclient.Client, inv inventory) (inventory, bool, error) {
	changed := false
	for i := range inv.Resources {
		resource := &inv.Resources[i]
		if resource.Type != "ona_project" || resource.UUID == "" {
			continue
		}
		classes, err := listProjectEnvironmentClasses(ctx, api, resource.UUID)
		if err != nil {
			return inventory{}, false, err
		}
		before := len(resource.ReferenceIDs)
		for _, class := range classes {
			resource.ReferenceIDs = append(resource.ReferenceIDs, class.GetEnvironmentClassId())
		}
		resource.ReferenceIDs = compactStrings(resource.ReferenceIDs)
		if len(resource.ReferenceIDs) != before {
			changed = true
		}
	}
	return inv, changed, nil
}

func listServiceAccounts(ctx context.Context, cfg config) ([]*onaclient.ServiceAccount, error) {
	client, err := onaclient.New(onaclient.Config{
		Host:      cfg.host,
		Token:     cfg.token,
		UserAgent: onaclient.UserAgent + " terraform-import",
	})
	if err != nil {
		return nil, err
	}
	var result []*onaclient.ServiceAccount
	err = eachPage(func(token string) (string, error) {
		resp, err := client.ListServiceAccounts(ctx, onaclient.ListServiceAccountsRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
			Filter:     &onaclient.ListServiceAccountsFilter{IncludeSuspended: false},
		})
		if err != nil {
			return "", fmt.Errorf("list service accounts: %w", err)
		}
		for _, account := range resp.ServiceAccounts {
			if account.GetOrganizationId() == cfg.orgID || cfg.orgID == "" {
				result = append(result, account)
			}
		}
		return resp.Pagination.GetNextToken(), nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].GetName() < result[j].GetName() })
	return result, err
}

func listWorkflows(ctx context.Context, api *onaclient.Client) ([]*onaclient.Workflow, error) {
	var result []*onaclient.Workflow
	err := eachPage(func(token string) (string, error) {
		resp, err := api.ListWorkflows(ctx, onaclient.ListWorkflowsRequest{
			Pagination: &onaclient.PaginationRequest{PageSize: 100, Token: token},
		})
		if err != nil {
			return "", fmt.Errorf("list automations: %w", err)
		}
		result = append(result, resp.Workflows...)
		return resp.Pagination.GetNextToken(), nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].GetMetadata().GetName() < result[j].GetMetadata().GetName() })
	return result, err
}

func getOrganizationPolicies(ctx context.Context, api *onaclient.Client, orgID string) (*onaclient.OrganizationPolicies, error) {
	resp, err := api.GetOrganizationPolicies(ctx, onaclient.GetOrganizationPoliciesRequest{OrganizationID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get organization policies: %w", err)
	}
	return resp.Policies, nil
}

func eachPage(fetch func(token string) (next string, err error)) error {
	for token := ""; ; {
		next, err := fetch(token)
		if err != nil {
			return err
		}
		if next == "" {
			return nil
		}
		token = next
	}
}

func chunks(values []string, size int) [][]string {
	if size <= 0 || len(values) == 0 {
		return nil
	}
	var result [][]string
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		result = append(result, values[start:end])
	}
	return result
}
