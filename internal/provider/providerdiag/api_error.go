// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package providerdiag

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func AddAPIError(diags *diag.Diagnostics, summary string, operation string, err error) {
	diags.AddError(summary, APIErrorDetail(operation, err))
}

func APIErrorDetail(operation string, err error) string {
	if err == nil {
		return "Ona returned an empty error."
	}

	detail := actionForCode(connect.CodeOf(err), operation)
	if detail == "" {
		detail = fmt.Sprintf("Ona could not complete the request while %s.", operation)
	}
	return fmt.Sprintf("%s\n\nAPI error: %s", detail, err)
}

func actionForCode(code connect.Code, operation string) string {
	switch code {
	case connect.CodeUnauthenticated:
		return fmt.Sprintf("Ona rejected the API token while %s. Set the provider `token` argument or `ONA_TOKEN` to a valid personal access token or service-account token.", operation)
	case connect.CodePermissionDenied:
		return fmt.Sprintf("The configured Ona API token does not have permission to complete the request while %s. Use a token whose subject has access to the organization and resource being managed.", operation)
	case connect.CodeNotFound:
		return fmt.Sprintf("Ona could not find the requested resource while %s. Verify the configured ID, organization, and token scope. If the resource was deleted outside Terraform, run `terraform plan` again so Terraform can reconcile state.", operation)
	case connect.CodeInvalidArgument:
		return fmt.Sprintf("Ona rejected the request while %s because one or more Terraform arguments are invalid. Check the attributes named in the API error and update the Terraform configuration.", operation)
	case connect.CodeFailedPrecondition:
		return fmt.Sprintf("Ona cannot complete the request while %s because the remote resource is not in the required state yet. Complete the prerequisite described in the API error, then rerun Terraform.", operation)
	case connect.CodeResourceExhausted:
		return fmt.Sprintf("Ona rate-limited or quota-limited the request while %s. Wait and rerun Terraform; if the problem persists, reduce parallelism or contact Ona support.", operation)
	case connect.CodeUnavailable:
		return fmt.Sprintf("Ona was temporarily unavailable while %s. Rerun Terraform after the service recovers.", operation)
	case connect.CodeDeadlineExceeded:
		return fmt.Sprintf("The Ona API request timed out while %s. Rerun Terraform; if the problem persists, reduce Terraform parallelism or contact Ona support.", operation)
	default:
		return ""
	}
}
