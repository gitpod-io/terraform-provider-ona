// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/zclconf/go-cty/cty"
)

type localImportLoopListBlock struct {
	TypeName        string
	IncludeResource bool
}

func TestLocalImportLoopListsEveryRegisteredResource(t *testing.T) {
	t.Parallel()

	queryPath := filepath.Join("..", "..", "dev", "local-importloop", "query.tfquery.hcl")
	querySource, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("read local import loop query: %v", err)
	}

	queryFile, diags := hclsyntax.ParseConfig(querySource, queryPath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		t.Fatalf("parse local import loop query: %s", diags.Error())
	}
	queryBody, ok := queryFile.Body.(*hclsyntax.Body)
	if !ok {
		t.Fatalf("parse local import loop query: unexpected body type %T", queryFile.Body)
	}

	var actual []localImportLoopListBlock
	for _, block := range queryBody.Blocks {
		if block.Type != "list" {
			continue
		}
		if len(block.Labels) != 2 {
			t.Fatalf("list block at %s has %d labels, want 2", block.TypeRange.String(), len(block.Labels))
		}
		includeResource, ok := block.Body.Attributes["include_resource"]
		if !ok {
			t.Fatalf("list %q has no include_resource attribute", block.Labels[0])
		}
		includeResourceValue, includeResourceDiags := includeResource.Expr.Value(nil)
		if includeResourceDiags.HasErrors() {
			t.Fatalf("evaluate list %q include_resource: %s", block.Labels[0], includeResourceDiags.Error())
		}
		if !includeResourceValue.IsKnown() || includeResourceValue.IsNull() || !includeResourceValue.Type().Equals(cty.Bool) {
			t.Fatalf("list %q include_resource must be a known boolean", block.Labels[0])
		}
		actual = append(actual, localImportLoopListBlock{
			TypeName:        block.Labels[0],
			IncludeResource: includeResourceValue.True(),
		})
	}

	ctx := t.Context()
	provider := &OnaProvider{}
	expected := make([]localImportLoopListBlock, 0, len(provider.ListResources(ctx)))
	for _, newListResource := range provider.ListResources(ctx) {
		var resp resource.MetadataResponse
		newListResource().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "ona"}, &resp)
		if resp.TypeName == "" {
			t.Fatal("registered list resource returned an empty type name")
		}
		expected = append(expected, localImportLoopListBlock{
			TypeName:        resp.TypeName,
			IncludeResource: true,
		})
	}

	sort.Slice(actual, func(i, j int) bool { return actual[i].TypeName < actual[j].TypeName })
	sort.Slice(expected, func(i, j int) bool { return expected[i].TypeName < expected[j].TypeName })
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("local import loop list resources mismatch (-want +got):\n%s", diff)
	}
}
