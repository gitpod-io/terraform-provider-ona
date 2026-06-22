package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func rewriteGeneratedConfig(path string, inv inventory) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read generated config: %w", err)
	}
	file, diags := hclwrite.ParseConfig(data, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return fmt.Errorf("parse generated config: %s", diags.Error())
	}
	refs := referenceMap(inv)
	count := rewriteBody(file.Body(), refs)
	logStepf("rewrote %d generated attributes to references", count)
	if err := os.WriteFile(path, file.Bytes(), 0644); err != nil {
		return fmt.Errorf("write rewritten config: %w", err)
	}
	return nil
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
		logStepf("wrote %s with %d resource blocks", target, counts[name])
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
	if block.Type() != "resource" || len(block.Labels()) == 0 {
		return "generated_misc.tf"
	}
	switch block.Labels()[0] {
	case "ona_automation":
		return "automations.tf"
	case "ona_runner_environment_class":
		return "runner_environment_classes.tf"
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
	default:
		return "generated_misc.tf"
	}
}

func referenceMap(inv inventory) map[string]hcl.Traversal {
	result := map[string]hcl.Traversal{}
	for _, r := range inv.Resources {
		if r.UUID == "" || r.SkipReason != "" || r.Type == "ona_organization_policies" {
			continue
		}
		traversal, diags := hclsyntax.ParseTraversalAbs([]byte(r.Address+".id"), "", hcl.Pos{Line: 1, Column: 1})
		if !diags.HasErrors() {
			result[r.UUID] = traversal
		}
	}
	return result
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
	for it := value.ElementIterator(); it.Next(); {
		_, item := it.Element()
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
