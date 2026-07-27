package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func rewriteGeneratedConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read generated config: %w", err)
	}
	file, diags := hclwrite.ParseConfig(data, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return fmt.Errorf("parse generated config: %s", diags.Error())
	}
	refs, err := identityReferenceMap(file.Body())
	if err != nil {
		return err
	}

	count := 0
	for _, block := range file.Body().Blocks() {
		if block.Type() == "resource" {
			count += rewriteBody(block.Body(), refs)
		}
	}
	logStepf("derived %d unambiguous resource identities and rewrote %d attributes", len(refs), count)
	if err := os.WriteFile(path, file.Bytes(), 0644); err != nil {
		return fmt.Errorf("write rewritten config: %w", err)
	}
	return nil
}

func identityReferenceMap(body *hclwrite.Body) (map[string]hcl.Traversal, error) {
	candidates := map[string][]hcl.Traversal{}
	for _, block := range body.Blocks() {
		if block.Type() != "import" {
			continue
		}
		attributes := block.Body().Attributes()
		toAttribute, hasTo := attributes["to"]
		identityAttribute, hasIdentity := attributes["identity"]
		if !hasTo || !hasIdentity {
			continue
		}

		toExpression, parseDiags := hclsyntax.ParseExpression(toAttribute.Expr().BuildTokens(nil).Bytes(), "", hcl.Pos{Line: 1, Column: 1})
		if parseDiags.HasErrors() {
			return nil, fmt.Errorf("parse generated import destination: %s", parseDiags.Error())
		}
		toTraversal, traversalDiags := hcl.AbsTraversalForExpr(toExpression)
		if traversalDiags.HasErrors() {
			return nil, fmt.Errorf("read generated import destination: %s", traversalDiags.Error())
		}

		identityExpression, parseDiags := hclsyntax.ParseExpression(identityAttribute.Expr().BuildTokens(nil).Bytes(), "", hcl.Pos{Line: 1, Column: 1})
		if parseDiags.HasErrors() {
			return nil, fmt.Errorf("parse generated import identity: %s", parseDiags.Error())
		}
		identity, valueDiags := identityExpression.Value(nil)
		if valueDiags.HasErrors() {
			return nil, fmt.Errorf("read generated import identity: %s", valueDiags.Error())
		}
		identityAttributeName, id, ok := singleStringIdentity(identity)
		if !ok {
			continue
		}
		if !identityMatchesResourceType(toTraversal.RootName(), identityAttributeName) {
			continue
		}

		toTraversal = append(toTraversal, hcl.TraverseAttr{Name: "id"})
		candidates[id] = append(candidates[id], toTraversal)
	}

	result := map[string]hcl.Traversal{}
	for id, traversals := range candidates {
		if len(traversals) == 1 {
			result[id] = traversals[0]
		}
	}
	return result, nil
}

func singleStringIdentity(identity cty.Value) (string, string, bool) {
	identityType := identity.Type()
	if !identity.IsKnown() || identity.IsNull() || (!identityType.IsObjectType() && !identityType.IsMapType()) {
		return "", "", false
	}

	var attributeName string
	var result string
	nonNullValues := 0
	for iterator := identity.ElementIterator(); iterator.Next(); {
		key, value := iterator.Element()
		if !value.IsKnown() {
			return "", "", false
		}
		if value.IsNull() {
			continue
		}
		nonNullValues++
		if value.Type() != cty.String {
			return "", "", false
		}
		attributeName = key.AsString()
		result = value.AsString()
	}
	if nonNullValues != 1 || result == "" {
		return "", "", false
	}
	return attributeName, result, true
}

func identityMatchesResourceType(resourceType, identityAttributeName string) bool {
	if identityAttributeName == "id" {
		return true
	}
	resourceName := strings.TrimPrefix(resourceType, "ona_")
	return resourceName != resourceType && identityAttributeName == resourceName+"_id"
}

func splitGeneratedConfig(path, dir string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rewritten generated config: %w", err)
	}
	file, diags := hclwrite.ParseConfig(data, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse rewritten generated config: %s", diags.Error())
	}

	grouped := map[string][]byte{}
	counts := map[string]int{}
	for _, block := range file.Body().Blocks() {
		name := outputFileForBlock(block)
		grouped[name] = append(grouped[name], block.BuildTokens(nil).Bytes()...)
		grouped[name] = append(grouped[name], '\n')
		counts[name]++
	}

	var names []string
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)

	var paths []string
	for _, name := range names {
		target := filepath.Join(dir, name)
		if err := os.WriteFile(target, grouped[name], 0644); err != nil {
			return nil, fmt.Errorf("write %s: %w", target, err)
		}
		logStepf("wrote %s with %d blocks", target, counts[name])
		paths = append(paths, target)
	}
	if err := os.WriteFile(filepath.Join(dir, "generated.rewritten.tf.txt"), data, 0644); err != nil {
		return nil, fmt.Errorf("write rewritten generated config copy: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return nil, fmt.Errorf("remove intermediate generated config: %w", err)
	}
	return paths, nil
}

func outputFileForBlock(block *hclwrite.Block) string {
	if block.Type() == "import" {
		return "imports.tf"
	}
	if block.Type() != "resource" || len(block.Labels()) == 0 {
		return "generated_misc.tf"
	}
	resourceType := block.Labels()[0]
	switch resourceType {
	case "ona_automation":
		return "automations.tf"
	case "ona_environment_class":
		return "environment_classes.tf"
	case "ona_group":
		return "groups.tf"
	case "ona_organization_policies":
		return "organization_policies.tf"
	case "ona_project":
		return "projects.tf"
	case "ona_runner":
		return "runners.tf"
	case "ona_security_policy":
		return "security_policies.tf"
	case "ona_team":
		return "teams.tf"
	}
	if strings.HasPrefix(resourceType, "ona_") {
		return strings.TrimPrefix(resourceType, "ona_") + ".tf"
	}
	return "generated_misc.tf"
}

func rewriteBody(body *hclwrite.Body, refs map[string]hcl.Traversal) int {
	count := 0
	for name, attr := range body.Attributes() {
		tokens, ok := rewrittenAttributeTokens(attr, refs)
		if ok {
			body.SetAttributeRaw(name, tokens)
			count++
		}
	}
	for _, block := range body.Blocks() {
		count += rewriteBody(block.Body(), refs)
	}
	return count
}

func rewrittenAttributeTokens(attr *hclwrite.Attribute, refs map[string]hcl.Traversal) (hclwrite.Tokens, bool) {
	src := attr.Expr().BuildTokens(nil).Bytes()
	expr, diags := hclsyntax.ParseExpression(src, "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, false
	}
	value, diags := expr.Value(nil)
	if diags.HasErrors() || !value.IsWhollyKnown() {
		return nil, false
	}
	if value.Type() == cty.String {
		ref, ok := refs[value.AsString()]
		if !ok {
			return nil, false
		}
		return hclwrite.TokensForTraversal(ref), true
	}
	if !value.CanIterateElements() {
		return nil, false
	}
	var elems []hclwrite.Tokens
	changed := false
	for iterator := value.ElementIterator(); iterator.Next(); {
		_, item := iterator.Element()
		if item.Type() == cty.String {
			if ref, ok := refs[item.AsString()]; ok {
				elems = append(elems, hclwrite.TokensForTraversal(ref))
				changed = true
				continue
			}
		}
		elems = append(elems, hclwrite.TokensForValue(item))
	}
	if !changed {
		return nil, false
	}
	return tokensForMultilineTuple(elems), true
}

func tokensForMultilineTuple(elems []hclwrite.Tokens) hclwrite.Tokens {
	if len(elems) == 0 {
		return hclwrite.TokensForTuple(elems)
	}
	var tokens hclwrite.Tokens
	tokens = append(tokens, token(hclsyntax.TokenOBrack, "["))
	tokens = append(tokens, token(hclsyntax.TokenNewline, "\n"))
	for _, elem := range elems {
		if len(elem) > 0 {
			elem[0].SpacesBefore += 2
		}
		tokens = append(tokens, elem...)
		tokens = append(tokens, token(hclsyntax.TokenComma, ","))
		tokens = append(tokens, token(hclsyntax.TokenNewline, "\n"))
	}
	tokens = append(tokens, token(hclsyntax.TokenCBrack, "]"))
	return tokens
}

func token(typ hclsyntax.TokenType, bytes string) *hclwrite.Token {
	return &hclwrite.Token{
		Type:         typ,
		Bytes:        []byte(bytes),
		SpacesBefore: 0,
	}
}
