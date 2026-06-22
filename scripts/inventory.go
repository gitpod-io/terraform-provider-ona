package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	onaclient "github.com/ona/terraform-provider-ona/internal/client"
)

func buildInventory(s snapshot) inventory {
	labels := buildLabels(s)
	var resources []inventoryResource
	add := func(r inventoryResource) {
		resources = append(resources, r)
	}

	for _, g := range s.groups {
		add(inventoryResource{
			Type:         "ona_group",
			Address:      "ona_group." + labels.group[g.GetId()],
			UUID:         g.GetId(),
			ImportID:     g.GetId(),
			Name:         g.GetName(),
			ReferenceIDs: groupMembershipReferenceIDs(s.groupMemberships[g.GetId()]),
		})
	}
	for _, t := range s.teams {
		add(inventoryResource{Type: "ona_team", Address: "ona_team." + labels.team[t.GetId()], UUID: t.GetId(), ImportID: t.GetId(), Name: t.GetName()})
	}
	for _, p := range s.securityPolicies {
		name := p.GetId()
		if p.GetMetadata() != nil {
			name = p.GetMetadata().GetName()
		}
		add(inventoryResource{Type: "ona_security_policy", Address: "ona_security_policy." + labels.securityPolicy[p.GetId()], UUID: p.GetId(), ImportID: p.GetId(), Name: name})
	}
	for _, r := range s.runners {
		add(inventoryResource{Type: "ona_runner", Address: "ona_runner." + labels.runner[r.GetRunnerId()], UUID: r.GetRunnerId(), ImportID: r.GetRunnerId(), Name: r.GetName()})
	}
	addEnvironmentClassResources(add, s, labels)
	addExternalResources(add, s, labels)
	for _, p := range s.projects {
		add(inventoryResource{
			Type:         "ona_project",
			Address:      "ona_project." + labels.project[p.GetId()],
			UUID:         p.GetId(),
			ImportID:     p.GetId(),
			Name:         p.GetMetadata().GetName(),
			ReferenceIDs: projectReferenceIDs(p),
		})
	}
	if s.organizationPolicy != nil {
		add(inventoryResource{Type: "ona_organization_policies", Address: "ona_organization_policies.current", UUID: s.orgID, ImportID: "current", Name: "current"})
	}
	for _, w := range s.workflows {
		resource := inventoryResource{Type: "ona_automation", Address: "ona_automation." + labels.workflow[w.GetId()], UUID: w.GetId(), ImportID: w.GetId(), Name: w.GetMetadata().GetName()}
		if reason := unsupportedWorkflowReason(w); reason != "" {
			resource.SkipReason = reason
		}
		add(resource)
	}

	sortInventory(resources)
	return inventory{OrganizationID: s.orgID, Resources: resources}
}

func buildExternalInventory(s snapshot) inventory {
	labels := buildLabels(s)
	var resources []inventoryResource
	addEnvironmentClassResources(func(r inventoryResource) {
		resources = append(resources, r)
	}, s, labels)
	addExternalResources(func(r inventoryResource) {
		resources = append(resources, r)
	}, s, labels)
	sortInventory(resources)
	return inventory{OrganizationID: s.orgID, Resources: resources}
}

func addEnvironmentClassResources(add func(inventoryResource), s snapshot, labels labelSet) {
	for _, c := range s.environmentClasses {
		add(inventoryResource{
			Type:     "ona_runner_environment_class",
			Address:  "ona_runner_environment_class." + labels.environmentClass[c.GetId()],
			UUID:     c.GetId(),
			ImportID: c.GetId(),
			Name:     c.GetDisplayName(),
			References: map[string]string{
				"runner_id": c.GetRunnerId(),
			},
		})
	}
}

func addExternalResources(add func(inventoryResource), s snapshot, labels labelSet) {
	for _, a := range s.serviceAccounts {
		add(inventoryResource{
			Type:    "external_service_account",
			Address: "local.service_accounts." + labels.serviceAccount[a.GetId()],
			UUID:    a.GetId(),
			Name:    a.GetName(),
		})
	}
}

type labelSet struct {
	group            map[string]string
	team             map[string]string
	securityPolicy   map[string]string
	runner           map[string]string
	environmentClass map[string]string
	project          map[string]string
	serviceAccount   map[string]string
	workflow         map[string]string
}

func buildLabels(s snapshot) labelSet {
	l := labelSet{
		group:            map[string]string{},
		team:             map[string]string{},
		securityPolicy:   map[string]string{},
		runner:           map[string]string{},
		environmentClass: map[string]string{},
		project:          map[string]string{},
		serviceAccount:   map[string]string{},
		workflow:         map[string]string{},
	}
	used := map[string]map[string]struct{}{}
	makeLabel := func(kind, preferred, id string) string {
		if used[kind] == nil {
			used[kind] = map[string]struct{}{}
		}
		base := terraformLabel(preferred)
		if base == "" {
			base = "r_" + strings.ReplaceAll(id[:minInt(len(id), 8)], "-", "_")
		}
		candidate := base
		for i := 2; ; i++ {
			if _, ok := used[kind][candidate]; !ok {
				used[kind][candidate] = struct{}{}
				return candidate
			}
			candidate = fmt.Sprintf("%s_%d", base, i)
		}
	}

	for _, g := range s.groups {
		l.group[g.GetId()] = makeLabel("group", g.GetName(), g.GetId())
	}
	for _, t := range s.teams {
		l.team[t.GetId()] = makeLabel("team", t.GetName(), t.GetId())
	}
	for _, p := range s.securityPolicies {
		name := p.GetId()
		if p.GetMetadata() != nil {
			name = p.GetMetadata().GetName()
		}
		l.securityPolicy[p.GetId()] = makeLabel("security_policy", name, p.GetId())
	}
	for _, r := range s.runners {
		l.runner[r.GetRunnerId()] = makeLabel("runner", r.GetName(), r.GetRunnerId())
	}
	runnerNames := runnerNamesByID(s.runners)
	for _, c := range s.environmentClasses {
		l.environmentClass[c.GetId()] = makeLabel("environment_class", environmentClassPreferredLabel(c, runnerNames), c.GetId())
	}
	for _, p := range s.projects {
		name := p.GetId()
		if p.GetMetadata() != nil {
			name = p.GetMetadata().GetName()
		}
		l.project[p.GetId()] = makeLabel("project", name, p.GetId())
	}
	for _, a := range s.serviceAccounts {
		l.serviceAccount[a.GetId()] = makeLabel("service_account", a.GetName(), a.GetId())
	}
	for _, w := range s.workflows {
		name := w.GetId()
		if w.GetMetadata() != nil {
			name = w.GetMetadata().GetName()
		}
		l.workflow[w.GetId()] = makeLabel("automation", name, w.GetId())
	}
	return l
}

func runnerNamesByID(runners []*onaclient.Runner) map[string]string {
	result := map[string]string{}
	for _, r := range runners {
		if r.GetRunnerId() != "" {
			result[r.GetRunnerId()] = r.GetName()
		}
	}
	return result
}

func environmentClassPreferredLabel(c *onaclient.EnvironmentClass, runnerNames map[string]string) string {
	runnerName := strings.TrimSpace(runnerNames[c.GetRunnerId()])
	displayName := strings.TrimSpace(c.GetDisplayName())
	if runnerName == "" {
		return displayName
	}
	if displayName == "" {
		return runnerName
	}
	return runnerName + "_" + displayName
}

func sortInventory(resources []inventoryResource) {
	sort.SliceStable(resources, func(i, j int) bool {
		if resources[i].Type != resources[j].Type {
			return resources[i].Type < resources[j].Type
		}
		return resources[i].Address < resources[j].Address
	})
}

func missingEnvironmentClassIDs(inv inventory, fetched []*onaclient.EnvironmentClass) []string {
	known := map[string]struct{}{}
	for _, class := range fetched {
		if class.GetId() != "" {
			known[class.GetId()] = struct{}{}
		}
	}
	for _, resource := range inv.Resources {
		if (resource.Type == "ona_runner_environment_class" || resource.Type == "ona_environment_class" || resource.Type == "external_environment_class") && resource.UUID != "" {
			known[resource.UUID] = struct{}{}
		}
	}

	var missing []string
	seen := map[string]struct{}{}
	for _, resource := range inv.Resources {
		for _, id := range resource.ReferenceIDs {
			if id == "" {
				continue
			}
			if _, ok := known[id]; ok {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}

func mergeExternalResources(resources, external []inventoryResource) []inventoryResource {
	merged := make([]inventoryResource, 0, len(resources)+len(external))
	for _, resource := range resources {
		if resource.Type == "external_environment_class" || resource.Type == "ona_environment_class" {
			continue
		}
		merged = append(merged, resource)
	}
	seen := map[string]struct{}{}
	for _, resource := range merged {
		seen[externalResourceKey(resource)] = struct{}{}
	}
	for _, resource := range external {
		key := externalResourceKey(resource)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, resource)
	}
	return merged
}

func externalResourceKey(resource inventoryResource) string {
	if resource.UUID != "" {
		return resource.Type + "\x00" + resource.UUID
	}
	return resource.Type + "\x00" + resource.Address
}

func runnerIDsFromInventory(inv inventory) []string {
	var result []string
	for _, r := range inv.Resources {
		if r.Type == "ona_runner" && r.UUID != "" {
			result = append(result, r.UUID)
		}
	}
	return result
}

func unsupportedWorkflowReason(w *onaclient.Workflow) string {
	if w.GetSpec().GetReport() != nil {
		return "reports are not supported by ona_automation"
	}
	for _, trigger := range w.GetSpec().GetTriggers() {
		if trigger.GetIncident() != nil {
			return "incident triggers are not supported by ona_automation"
		}
		if trigger.GetContext().GetRepositories() != nil {
			return "repository trigger contexts are not supported by ona_automation"
		}
	}
	return ""
}

func projectReferenceIDs(project *onaclient.Project) []string {
	if project == nil {
		return nil
	}
	var result []string
	for _, class := range project.GetEnvironmentClasses() {
		result = append(result, class.GetEnvironmentClassId())
	}
	result = append(result, project.GetPrebuildConfiguration().GetEnvironmentClassIds()...)
	if executor := project.GetPrebuildConfiguration().GetExecutor(); executor != nil {
		result = append(result, executor.GetId())
	}
	return compactStrings(result)
}

func groupMembershipReferenceIDs(members []*onaclient.GroupMembership) []string {
	var result []string
	for _, member := range members {
		subject := member.GetSubject()
		if subject.GetPrincipal() == onaclient.PrincipalServiceAccount {
			result = append(result, subject.GetId())
		}
	}
	return compactStrings(result)
}

func compactStrings(values []string) []string {
	var result []string
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

var invalidLabelChar = regexp.MustCompile(`[^a-z0-9_]+`)

func terraformLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = invalidLabelChar.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return ""
	}
	if value[0] >= '0' && value[0] <= '9' {
		value = "r_" + value
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
