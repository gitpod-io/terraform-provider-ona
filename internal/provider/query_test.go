// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// QueryTestCase creates the shared hermetic Terraform 1.14+ test case used by
// resource-specific Query tests. Callers supply their fake API server and
// Query step while this helper provides the common provider configuration.
func QueryTestCase(host string, step testresource.TestStep) testresource.TestCase {
	return testresource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		PreCheck:                 func() {},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []testresource.TestStep{
			{
				Config: QueryProviderConfig(host),
			},
			step,
		},
	}
}

// QueryProviderConfig returns provider configuration for a fake Ona API.
func QueryProviderConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}
`, host)
}
