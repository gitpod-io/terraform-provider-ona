package main

import onaclient "github.com/gitpod-io/terraform-provider-ona/internal/client"

const providerSource = "registry.terraform.io/gitpod-io/ona"
const importMapFileName = "import-map.json"

type config struct {
	host                 string
	token                string
	orgID                string
	workdir              string
	providerDir          string
	terraform            string
	resourceTypes        stringList
	resourceIDs          stringList
	terraformParallelism int
	apiBaseURL           string
	includeSystemGroups  bool
	skipTerraform        bool
	skipValidate         bool
	refreshImportMap     bool
}

type inventory struct {
	OrganizationID string              `json:"organization_id"`
	Resources      []inventoryResource `json:"resources"`
}

type inventoryResource struct {
	Type         string            `json:"type"`
	Address      string            `json:"address"`
	UUID         string            `json:"uuid,omitempty"`
	ImportID     string            `json:"import_id"`
	Name         string            `json:"name,omitempty"`
	References   map[string]string `json:"references,omitempty"`
	ReferenceIDs []string          `json:"reference_ids,omitempty"`
	SkipReason   string            `json:"skip_reason,omitempty"`
}

type snapshot struct {
	orgID              string
	groups             []*onaclient.Group
	groupMemberships   map[string][]*onaclient.GroupMembership
	teams              []*onaclient.Team
	securityPolicies   []*onaclient.SecurityPolicy
	runners            []*onaclient.Runner
	environmentClasses []*onaclient.EnvironmentClass
	projects           []*onaclient.Project
	serviceAccounts    []*onaclient.ServiceAccount
	workflows          []*onaclient.Workflow
	organizationPolicy *onaclient.OrganizationPolicies
}
