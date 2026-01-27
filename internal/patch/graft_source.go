package patch

import (
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func resolveGraftTokens(overrideBlocks []*hclwrite.Block, existingBlocks map[string]*hclwrite.Block, existingLocals map[string]*hclwrite.Attribute) {
	for _, overrideBlock := range overrideBlocks {
		srcBody := overrideBlock.Body()
		for _, block := range srcBody.Blocks() {
			if block.Type() == "locals" {
				for name, attr := range block.Body().Attributes() {
					if targetAttr, ok := existingLocals[name]; ok {
						valTokens := attr.Expr().BuildTokens(nil)
						newTokens := replaceGraftSourceTokens(valTokens, targetAttr.Expr().BuildTokens(nil))
						if newTokens != nil {
							block.Body().SetAttributeRaw(name, newTokens)
						}
					}
				}
				continue
			}

			key := blockKey(block)
			if existingBlock := existingBlocks[key]; existingBlock != nil {
				resolveBodyGraftSource(block.Body(), existingBlock.Body())
			}
		}
	}
}

func resolveBodyGraftSource(body *hclwrite.Body, originalBody *hclwrite.Body) {
	for name, attr := range body.Attributes() {
		originalAttr := originalBody.GetAttribute(name)

		var replacement hclwrite.Tokens
		if originalAttr != nil {
			replacement = originalAttr.Expr().BuildTokens(nil)
		} else {
			replacement = hclwrite.Tokens{
				{
					Type:  hclsyntax.TokenIdent,
					Bytes: []byte("null"),
				},
			}
		}

		newTokens := replaceGraftSourceTokens(attr.Expr().BuildTokens(nil), replacement)
		if newTokens != nil {
			body.SetAttributeRaw(name, newTokens)
		}
	}

	// Simple recursion for nested blocks that are unique by type
	// If there are multiple blocks of the same type (like ingress), this naive matching
	// might be ambiguous, but for attributes like lifecycle it works well.
	for _, block := range body.Blocks() {
		// Try to find matching block in originalBody
		// We look for strict match on Type and Labels
		matchingBlock := findMatchingBlock(originalBody, block)
		if matchingBlock != nil {
			resolveBodyGraftSource(block.Body(), matchingBlock.Body())
		}
	}
}

func findMatchingBlock(body *hclwrite.Body, pattern *hclwrite.Block) *hclwrite.Block {
	for _, b := range body.Blocks() {
		if b.Type() == pattern.Type() {
			// Check labels
			if len(b.Labels()) != len(pattern.Labels()) {
				continue
			}
			match := true
			for i, l := range b.Labels() {
				if l != pattern.Labels()[i] {
					match = false
					break
				}
			}
			if match {
				return b
			}
		}
	}
	return nil
}

func replaceGraftSourceTokens(tokens hclwrite.Tokens, replacement hclwrite.Tokens) hclwrite.Tokens {
	if len(replacement) == 0 {
		replacement = hclwrite.Tokens{
			{
				Type:  hclsyntax.TokenIdent,
				Bytes: []byte("null"),
			},
		}
	}

	var newTokens hclwrite.Tokens
	changed := false

	for i := 0; i < len(tokens); i++ {
		// Check for graft.source sequence: IDENT("graft") DOT IDENT("source")
		if i+2 < len(tokens) &&
			tokens[i].Type == hclsyntax.TokenIdent && string(tokens[i].Bytes) == "graft" &&
			tokens[i+1].Type == hclsyntax.TokenDot &&
			tokens[i+2].Type == hclsyntax.TokenIdent && string(tokens[i+2].Bytes) == "source" {

			newTokens = append(newTokens, replacement...)
			i += 2 // skip existing graft.source
			changed = true
		} else {
			newTokens = append(newTokens, tokens[i])
		}
	}

	if changed {
		return newTokens
	}
	return nil
}
