// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

//go:build generate

package tools

// Format Terraform code for use in documentation and local development.
// If you do not have Terraform installed, you can remove the formatting command, but it is suggested
// to ensure the Terraform examples and dev workspaces are formatted properly.
//go:generate terraform fmt -recursive ../examples/ ../dev/local-devloop/

// Generate documentation.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-dir .. -provider-name ona
