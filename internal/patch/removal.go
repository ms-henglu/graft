package patch

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

func applyRemovals(modulePath string, overrideBlocks []*hclwrite.Block) error {
	graftBlocks := make(map[string]*hclwrite.Block)
	for _, block := range overrideBlocks {
		graftBlock := filterBlockWithType(block, "_graft")
		if graftBlock == nil {
			continue
		}
		graftBlocks[blockKey(block)] = graftBlock
		block.Body().RemoveBlock(graftBlock)
	}

	entries, err := filepath.Glob(filepath.Join(modulePath, "*.tf"))
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry, "_graft_") {
			continue
		}

		content, err := os.ReadFile(entry)
		if err != nil {
			return err
		}

		f, diags := hclwrite.ParseConfig(content, entry, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			// Skip malformed files or files hclwrite can't parse
			continue
		}

		isFileChanged := false
		for _, block := range f.Body().Blocks() {
			graftBlock, ok := graftBlocks[blockKey(block)]
			if !ok {
				continue
			}

			removals := parseRemovals(graftBlock)
			if len(removals) == 0 {
				continue
			}

			// Check for "self" removal
			isSelfRemoval := false
			for _, r := range removals {
				if r == "self" {
					isSelfRemoval = true
					break
				}
			}

			if isSelfRemoval {
				f.Body().RemoveBlock(block)
				isFileChanged = true
				continue
			}

			// Handle granular removals
			hasChanges := false
			for _, r := range removals {
				if removePath(block, r) {
					hasChanges = true
				}
			}
			if hasChanges {
				isFileChanged = true
			}
		}

		if isFileChanged {
			if err := os.WriteFile(entry, f.Bytes(), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

func removePath(body *hclwrite.Block, path string) bool {
	if !strings.Contains(path, ".") {
		attr := body.Body().GetAttribute(path)
		if attr != nil {
			body.Body().RemoveAttribute(path)
			return true
		}

		var toRemove []*hclwrite.Block
		for _, b := range body.Body().Blocks() {
			if b.Type() == path {
				toRemove = append(toRemove, b)
				continue
			}
			// Special handling for dynamic blocks by label
			if b.Type() == "dynamic" && len(b.Labels()) > 0 && b.Labels()[0] == path {
				toRemove = append(toRemove, b)
			}
		}
		for _, b := range toRemove {
			body.Body().RemoveBlock(b)
		}
		return len(toRemove) > 0
	}

	blockType := strings.SplitN(path, ".", 2)[0]
	nestedPath := path[len(blockType)+1:]
	changed := false
	for _, b := range body.Body().Blocks() {
		if b.Type() == blockType {
			changed = changed || removePath(b, nestedPath)
		}
	}
	return changed
}

func parseRemovals(b *hclwrite.Block) []string {
	attr := b.Body().GetAttribute("remove")
	if attr == nil {
		return nil
	}

	// Convert tokens to bytes to be parsed as expression
	exprBytes := attr.Expr().BuildTokens(nil).Bytes()

	// Parse expression using HCL syntax
	expr, diags := hclsyntax.ParseExpression(exprBytes, "remove_attr", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil
	}

	// Evaluate expression
	val, diags := expr.Value(nil)
	if diags.HasErrors() {
		return nil
	}

	var res []string
	if val.Type().IsTupleType() || val.Type().IsListType() {
		it := val.ElementIterator()
		for it.Next() {
			_, v := it.Element()
			if v.Type() == cty.String {
				res = append(res, v.AsString())
			}
		}
		return res
	}

	if err := gocty.FromCtyValue(val, &res); err != nil {
		return nil
	}
	return res
}
