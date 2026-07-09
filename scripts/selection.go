package main

import (
	"fmt"
	"sort"
	"strings"
)

type stringList []string

func (l *stringList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

func (l *stringList) String() string {
	if l == nil {
		return ""
	}
	return strings.Join(*l, ",")
}

func selectInventory(inv inventory, cfg config) (inventory, error) {
	if !isResourceSelectionConfigured(cfg) {
		inv.Resources = importableInventoryResources(inv.Resources)
		return inv, nil
	}
	typeSet, err := selectedResourceTypes(cfg.resourceTypes)
	if err != nil {
		return inventory{}, err
	}
	idSet := selectedResourceIDs(cfg.resourceIDs)

	var selected []inventoryResource
	for _, r := range inv.Resources {
		if isRemovedResource(r) {
			continue
		}
		if isExternalReferenceResource(r) {
			continue
		}
		if len(typeSet) > 0 {
			if _, ok := typeSet[r.Type]; !ok {
				continue
			}
		}
		if len(idSet) > 0 && !resourceMatchesID(r, idSet) {
			continue
		}
		selected = append(selected, r)
	}
	if countImportableResources(inventory{Resources: selected}) == 0 {
		return inventory{}, fmt.Errorf("resource selection matched no importable resources")
	}

	resources := append([]inventoryResource{}, selected...)
	referenceIDs := selectedReferenceIDs(selected)
	runnerIDs := selectedRunnerIDs(selected)
	for _, r := range inv.Resources {
		if !isSelectedReferenceResource(r, runnerIDs, referenceIDs) {
			continue
		}
		if containsInventoryResource(resources, r) {
			continue
		}
		resources = append(resources, r)
	}

	sortInventory(resources)
	return inventory{OrganizationID: inv.OrganizationID, Resources: resources}, nil
}

func isResourceSelectionConfigured(cfg config) bool {
	return len(cfg.resourceTypes) > 0 || len(cfg.resourceIDs) > 0
}

func selectedResourceTypes(values []string) (map[string]struct{}, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := map[string]struct{}{}
	var unknown []string
	for _, value := range values {
		typ, ok := normalizeResourceType(value)
		if !ok {
			unknown = append(unknown, value)
			continue
		}
		result[typ] = struct{}{}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown resource type %q; supported types: %s", strings.Join(unknown, ", "), strings.Join(supportedResourceTypes(), ", "))
	}
	return result, nil
}

func selectedResourceIDs(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func normalizeResourceType(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.TrimPrefix(value, "ona_")
	value = strings.TrimSuffix(value, "s")

	if value == "runner" {
		return "ona_runner", true
	}
	if value == "environment_class" {
		return "ona_environment_class", true
	}
	if value == "project" {
		return "ona_project", true
	}
	return "", false
}

func supportedResourceTypes() []string {
	return []string{
		"ona_environment_class",
		"ona_project",
		"ona_runner",
	}
}

func isExternalReferenceResource(r inventoryResource) bool {
	return r.Type == "external_environment_class" || r.Type == "external_service_account"
}

func isRemovedResource(r inventoryResource) bool {
	return r.Type == "ona_project_policy"
}

func importableInventoryResources(resources []inventoryResource) []inventoryResource {
	result := make([]inventoryResource, 0, len(resources))
	for _, r := range resources {
		if isRemovedResource(r) {
			continue
		}
		result = append(result, r)
	}
	return result
}

func isSelectedReferenceResource(r inventoryResource, runnerIDs, referenceIDs map[string]struct{}) bool {
	switch r.Type {
	case "ona_environment_class", "ona_runner_environment_class":
		if _, ok := referenceIDs[r.UUID]; ok && r.UUID != "" {
			return true
		}
		runnerID := r.References["runner_id"]
		if _, ok := runnerIDs[runnerID]; ok && runnerID != "" {
			return true
		}
		return false
	case "external_service_account":
		if _, ok := referenceIDs[r.UUID]; ok && r.UUID != "" {
			return true
		}
		return false
	default:
		return false
	}
}

func containsInventoryResource(resources []inventoryResource, resource inventoryResource) bool {
	key := externalResourceKey(resource)
	for _, existing := range resources {
		if externalResourceKey(existing) == key {
			return true
		}
	}
	return false
}

func selectedRunnerIDs(resources []inventoryResource) map[string]struct{} {
	result := map[string]struct{}{}
	for _, r := range resources {
		if r.Type == "ona_runner" && r.UUID != "" {
			result[r.UUID] = struct{}{}
		}
	}
	return result
}

func selectedReferenceIDs(resources []inventoryResource) map[string]struct{} {
	result := map[string]struct{}{}
	for _, r := range resources {
		for _, value := range r.References {
			if value != "" {
				result[value] = struct{}{}
			}
		}
		for _, value := range r.ReferenceIDs {
			if value != "" {
				result[value] = struct{}{}
			}
		}
	}
	return result
}

func resourceMatchesID(r inventoryResource, ids map[string]struct{}) bool {
	if _, ok := ids[r.UUID]; ok && r.UUID != "" {
		return true
	}
	if _, ok := ids[r.ImportID]; ok && r.ImportID != "" {
		return true
	}
	return false
}

func countImportableResources(inv inventory) int {
	count := 0
	for _, r := range inv.Resources {
		if r.ImportID != "" && r.SkipReason == "" && isProviderImportableResource(r) {
			count++
		}
	}
	return count
}

func isProviderImportableResource(r inventoryResource) bool {
	return r.Type == "ona_runner" || r.Type == "ona_environment_class" || r.Type == "ona_project"
}
