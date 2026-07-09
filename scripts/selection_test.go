package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestStringListSet(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result []string
		Err    string
	}

	tests := []struct {
		Name     string
		Values   []string
		Expected Expectation
	}{
		{
			Name:   "single_value",
			Values: []string{"ona_project"},
			Expected: Expectation{
				Result: []string{"ona_project"},
			},
		},
		{
			Name:   "comma_separated_values_are_trimmed",
			Values: []string{" ona_project,ona_group ,, ona_team "},
			Expected: Expectation{
				Result: []string{"ona_project", "ona_group", "ona_team"},
			},
		},
		{
			Name:   "repeated_values_are_appended",
			Values: []string{"ona_project", "ona_group,ona_team"},
			Expected: Expectation{
				Result: []string{"ona_project", "ona_group", "ona_team"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			var list stringList
			for _, value := range tc.Values {
				if err := list.Set(value); err != nil {
					got.Err = err.Error()
					break
				}
			}
			got.Result = []string(list)

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("stringList.Set() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectInventory(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result inventory
		Err    string
	}

	tests := []struct {
		Name     string
		Config   config
		Expected Expectation
	}{
		{
			Name: "no_selection_drops_removed_resources_from_stale_inventory",
			Expected: Expectation{
				Result: testImportableInventory(),
			},
		},
		{
			Name: "resource_type_project_selects_project_and_references",
			Config: config{
				resourceTypes: stringList{"project"},
			},
			Expected: Expectation{
				Result: inventory{
					OrganizationID: "org-1",
					Resources: []inventoryResource{
						testServiceAccountResource(),
						testEnvironmentClassResource("class-1", "runner-1"),
						testEnvironmentClassResource("class-2", "runner-2"),
						testProjectResource("project-1", "one", "class-1"),
						testProjectResource("project-2", "two", "class-2"),
					},
				},
			},
		},
		{
			Name: "resource_type_group_is_not_supported_until_provider_registers_resource",
			Config: config{
				resourceTypes: stringList{"group"},
			},
			Expected: Expectation{
				Err: `unknown resource type "group"; supported types: ona_environment_class, ona_project, ona_runner`,
			},
		},
		{
			Name: "resource_type_and_id_selects_runner_intersection",
			Config: config{
				resourceTypes: stringList{"ona_runner"},
				resourceIDs:   stringList{"runner-2"},
			},
			Expected: Expectation{
				Result: inventory{
					OrganizationID: "org-1",
					Resources: []inventoryResource{
						testEnvironmentClassResource("class-2", "runner-2"),
						testRunnerResource("runner-2", "us_east"),
					},
				},
			},
		},
		{
			Name: "removed_resource_id_returns_error",
			Config: config{
				resourceIDs: stringList{"project-1/group-1"},
			},
			Expected: Expectation{
				Err: "resource selection matched no importable resources",
			},
		},
		{
			Name: "selected_runner_includes_system_environment_classes_for_that_runner",
			Config: config{
				resourceTypes: stringList{"runner"},
				resourceIDs:   stringList{"runner-1"},
			},
			Expected: Expectation{
				Result: inventory{
					OrganizationID: "org-1",
					Resources: []inventoryResource{
						testEnvironmentClassResource("class-1", "runner-1"),
						testRunnerResource("runner-1", "frankfurt"),
					},
				},
			},
		},
		{
			Name: "unknown_resource_type_returns_supported_types",
			Config: config{
				resourceTypes: stringList{"workspace"},
			},
			Expected: Expectation{
				Err: `unknown resource type "workspace"; supported types: ona_environment_class, ona_project, ona_runner`,
			},
		},
		{
			Name: "unmatched_id_returns_error",
			Config: config{
				resourceIDs: stringList{"missing"},
			},
			Expected: Expectation{
				Err: "resource selection matched no importable resources",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, err := selectInventory(testInventory(), tc.Config)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("selectInventory() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func testImportableInventory() inventory {
	return inventory{
		OrganizationID: "org-1",
		Resources: []inventoryResource{
			testEnvironmentClassResource("class-1", "runner-1"),
			testEnvironmentClassResource("class-2", "runner-2"),
			{
				Type:     "ona_group",
				Address:  "ona_group.one",
				UUID:     "group-1",
				ImportID: "group-1",
				Name:     "one",
			},
			testProjectResource("project-1", "one", "class-1"),
			testProjectResource("project-2", "two", "class-2"),
			testRunnerResource("runner-1", "frankfurt"),
			testRunnerResource("runner-2", "us_east"),
			testServiceAccountResource(),
		},
	}
}

func testInventory() inventory {
	return inventory{
		OrganizationID: "org-1",
		Resources: []inventoryResource{
			testEnvironmentClassResource("class-1", "runner-1"),
			testEnvironmentClassResource("class-2", "runner-2"),
			{
				Type:     "ona_group",
				Address:  "ona_group.one",
				UUID:     "group-1",
				ImportID: "group-1",
				Name:     "one",
			},
			testProjectResource("project-1", "one", "class-1"),
			testProjectResource("project-2", "two", "class-2"),
			testProjectPolicyResource(),
			testRunnerResource("runner-1", "frankfurt"),
			testRunnerResource("runner-2", "us_east"),
			testServiceAccountResource(),
		},
	}
}

func testProjectResource(id, name, environmentClassID string) inventoryResource {
	return inventoryResource{
		Type:     "ona_project",
		Address:  "ona_project." + name,
		UUID:     id,
		ImportID: id,
		Name:     name,
		ReferenceIDs: []string{
			environmentClassID,
			"service-account-1",
		},
	}
}

func testProjectPolicyResource() inventoryResource {
	return inventoryResource{
		Type:     "ona_project_policy",
		Address:  "ona_project_policy.one",
		ImportID: "project-1/group-1",
		Name:     "one",
		References: map[string]string{
			"group_id":   "group-1",
			"project_id": "project-1",
		},
	}
}

func testRunnerResource(id, name string) inventoryResource {
	return inventoryResource{
		Type:     "ona_runner",
		Address:  "ona_runner." + name,
		UUID:     id,
		ImportID: id,
		Name:     name,
	}
}

func testEnvironmentClassResource(id, runnerID string) inventoryResource {
	return inventoryResource{
		Type:     "ona_environment_class",
		Address:  "ona_environment_class." + id,
		UUID:     id,
		ImportID: id,
		Name:     id,
		References: map[string]string{
			"runner_id": runnerID,
		},
	}
}

func testServiceAccountResource() inventoryResource {
	return inventoryResource{
		Type:    "external_service_account",
		Address: "local.service_accounts.bot",
		UUID:    "service-account-1",
		Name:    "bot",
	}
}
