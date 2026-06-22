package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	onaclient "github.com/ona/terraform-provider-ona/internal/client"
)

func TestBuildInventoryEnvironmentClassLabels(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result []inventoryResource
	}

	tests := []struct {
		Name     string
		Input    snapshot
		Expected Expectation
	}{
		{
			Name: "prefixes_environment_classes_with_runner_name",
			Input: snapshot{
				orgID: "org-1",
				runners: []*onaclient.Runner{
					{RunnerID: "runner-1", Name: "Frankfurt"},
					{RunnerID: "runner-2", Name: "US East"},
				},
				environmentClasses: []*onaclient.EnvironmentClass{
					{ID: "class-1", RunnerID: "runner-1", DisplayName: "Large"},
					{ID: "class-2", RunnerID: "runner-2", DisplayName: "Large"},
				},
			},
			Expected: Expectation{
				Result: []inventoryResource{
					{
						Type:     "ona_runner",
						Address:  "ona_runner.frankfurt",
						UUID:     "runner-1",
						ImportID: "runner-1",
						Name:     "Frankfurt",
					},
					{
						Type:     "ona_runner",
						Address:  "ona_runner.us_east",
						UUID:     "runner-2",
						ImportID: "runner-2",
						Name:     "US East",
					},
					{
						Type:     "ona_runner_environment_class",
						Address:  "ona_runner_environment_class.frankfurt_large",
						UUID:     "class-1",
						ImportID: "class-1",
						Name:     "Large",
						References: map[string]string{
							"runner_id": "runner-1",
						},
					},
					{
						Type:     "ona_runner_environment_class",
						Address:  "ona_runner_environment_class.us_east_large",
						UUID:     "class-2",
						ImportID: "class-2",
						Name:     "Large",
						References: map[string]string{
							"runner_id": "runner-2",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Result: buildInventory(tc.Input).Resources}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("buildInventory() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildInventoryProjectReferencesEnvironmentClasses(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result []string
	}

	tests := []struct {
		Name     string
		Input    *onaclient.Project
		Expected Expectation
	}{
		{
			Name: "includes_allowed_and_prebuild_environment_classes",
			Input: &onaclient.Project{
				EnvironmentClasses: []*onaclient.ProjectEnvironmentClass{
					{EnvironmentClassID: "allowed-class"},
					{LocalRunner: true},
				},
				PrebuildConfiguration: &onaclient.ProjectPrebuildConfiguration{
					EnvironmentClassIDs: []string{"prebuild-class", "allowed-class"},
				},
			},
			Expected: Expectation{
				Result: []string{"allowed-class", "prebuild-class"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Result: projectReferenceIDs(tc.Input)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("projectReferenceIDs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildInventoryGroupReferencesServiceAccounts(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result []string
	}

	tests := []struct {
		Name     string
		Input    []*onaclient.GroupMembership
		Expected Expectation
	}{
		{
			Name: "includes_service_accounts_and_ignores_users",
			Input: []*onaclient.GroupMembership{
				{Subject: &onaclient.Subject{ID: "user-1", Principal: onaclient.PrincipalUser}},
				{Subject: &onaclient.Subject{ID: "service-account-1", Principal: onaclient.PrincipalServiceAccount}},
				{Subject: &onaclient.Subject{ID: "service-account-1", Principal: onaclient.PrincipalServiceAccount}},
			},
			Expected: Expectation{
				Result: []string{"service-account-1"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Result: groupMembershipReferenceIDs(tc.Input)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("groupMembershipReferenceIDs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMergeExternalResourcesUpgradesLegacyEnvironmentClasses(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result []inventoryResource
	}

	tests := []struct {
		Name      string
		Resources []inventoryResource
		External  []inventoryResource
		Expected  Expectation
	}{
		{
			Name: "replaces_legacy_external_and_generic_environment_classes",
			Resources: []inventoryResource{
				{Type: "external_environment_class", Address: "local.environment_classes.large", UUID: "class-1"},
				{Type: "ona_environment_class", Address: "ona_environment_class.large", UUID: "class-2", ImportID: "class-2"},
				{Type: "ona_project", Address: "ona_project.next", UUID: "project-1", ImportID: "project-1"},
			},
			External: []inventoryResource{
				{Type: "ona_runner_environment_class", Address: "ona_runner_environment_class.frankfurt_large", UUID: "class-1", ImportID: "class-1"},
				{Type: "ona_runner_environment_class", Address: "ona_runner_environment_class.frankfurt_regular", UUID: "class-2", ImportID: "class-2"},
			},
			Expected: Expectation{
				Result: []inventoryResource{
					{Type: "ona_project", Address: "ona_project.next", UUID: "project-1", ImportID: "project-1"},
					{Type: "ona_runner_environment_class", Address: "ona_runner_environment_class.frankfurt_large", UUID: "class-1", ImportID: "class-1"},
					{Type: "ona_runner_environment_class", Address: "ona_runner_environment_class.frankfurt_regular", UUID: "class-2", ImportID: "class-2"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Result: mergeExternalResources(tc.Resources, tc.External)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mergeExternalResources() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
