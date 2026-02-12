package absorb

import (
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/ms-henglu/graft/internal/utils"
	"github.com/zclconf/go-cty/cty"
)

type IndexedDriftChange struct {
	ResourceKey  string
	ProviderName string
	ResourceType string
	Changes      []DriftChange
}

func (c *IndexedDriftChange) ToBlock(schema *tfjson.SchemaBlock) *hclwrite.Block {
	if len(c.Changes) == 0 {
		return nil
	}

	first := c.Changes[0]
	indexRef := first.indexRef()

	// Pre-process each change: filter computed attrs and compute deep diff.
	// For block-type keys, save the full (pre-deepDiff) values since dynamic
	// blocks need the complete block content to replace the original static blocks.
	var processed []DriftChange

	for _, change := range c.Changes {
		result := filterComputedAttrs(change.ChangedAttrs, schema, "")
		attrs, _ := result.(map[string]interface{})
		if len(attrs) == 0 {
			continue
		}

		// Save full block values before deep diff
		fullBlockVals := make(map[string]interface{})
		for key, val := range attrs {
			if shouldRenderAsBlock(schema, key) {
				fullBlockVals[key] = utils.DeepCopyValue(val)
			}
		}

		if change.BeforeAttrs != nil && schema != nil {
			attrs = deepDiffBlock(change.BeforeAttrs, attrs, schema)
			if len(attrs) == 0 {
				continue
			}
		}

		change.FullBlockValues = fullBlockVals
		change.ChangedAttrs = attrs
		processed = append(processed, change)
	}
	if len(processed) == 0 {
		return nil
	}

	// Collect all attribute keys across all instances
	allKeys := make(map[string]bool)
	for _, pc := range processed {
		for key := range pc.ChangedAttrs {
			allKeys[key] = true
		}
	}

	// Separate block-type and plain attribute keys
	var blockKeys []string
	var attrKeys []string
	for _, key := range utils.SortedKeys(allKeys) {
		if shouldRenderAsBlock(schema, key) {
			blockKeys = append(blockKeys, key)
		} else {
			attrKeys = append(attrKeys, key)
		}
	}

	resBlock := hclwrite.NewBlock("resource", []string{first.ResourceType, first.ResourceName})
	resBody := resBlock.Body()

	// Render attribute drift using lookup()
	for _, key := range attrKeys {
		tokens := buildLookupTokens(processed, key, indexRef)
		resBody.SetAttributeRaw(key, tokens)
	}

	// Render block drift using dynamic blocks with per-instance lookup()
	var removals []string
	for _, key := range blockKeys {
		dynBlock := buildDynamicBlock(key, processed, indexRef, schema)
		if dynBlock != nil {
			resBody.AppendBlock(dynBlock)
			removals = append(removals, key)
		}
	}

	if len(removals) > 0 {
		sort.Strings(removals)
		graftBlock := resBody.AppendNewBlock("_graft", nil)
		graftBody := graftBlock.Body()
		var vals []cty.Value
		for _, r := range removals {
			vals = append(vals, cty.StringVal(r))
		}
		graftBody.SetAttributeValue("remove", cty.ListVal(vals))
	}

	return resBlock
}

// buildDynamicBlock generates a dynamic block for per-instance block drift:
//
//	dynamic "block_name" {
//	    for_each = lookup({
//	        "0" = [{ attr = val1 }]
//	        "1" = [{ attr = val2 }]
//	    }, count.index, [])
//	    content {
//	        attr = block_name.value.attr
//	    }
//	}
func buildDynamicBlock(blockName string, changes []DriftChange, indexRef string, schema *tfjson.SchemaBlock) *hclwrite.Block {
	// Filter changes that have drift for this block
	var relevant []DriftChange
	for _, change := range changes {
		if _, exists := change.ChangedAttrs[blockName]; exists {
			relevant = append(relevant, change)
		}
	}
	if len(relevant) == 0 {
		return nil
	}

	nestedSchema := nestedBlockSchema(schema, blockName)

	// Build recursive base map with all content keys across all instances
	var vals []interface{}
	for _, change := range relevant {
		val, ok := change.FullBlockValues[blockName]
		if !ok {
			val = change.ChangedAttrs[blockName]
		}
		if val != nil {
			vals = append(vals, val)
		}
	}
	base := buildBlockBase(vals, nestedSchema)
	dynBlock := hclwrite.NewBlock("dynamic", []string{blockName})
	dynBody := dynBlock.Body()

	// for_each = lookup({...}, indexRef, [])
	lookupMap := make(map[string]interface{})
	for _, change := range relevant {
		val, ok := change.FullBlockValues[blockName]
		if !ok {
			val = change.ChangedAttrs[blockName]
		}
		if val == nil {
			continue
		}
		lookupMap[change.indexKey()] = normalizeForDynamic(val, nestedSchema, base)
	}
	dynBody.SetAttributeRaw("for_each", buildLookupExprTokens(lookupMap, indexRef, "[]"))

	// content { ... }
	contentBlock := buildContentBlock(blockName, utils.SortedKeys(base), nestedSchema)
	dynBody.AppendBlock(contentBlock)

	return dynBlock
}

// buildBlockBase recursively builds a base map from multiple block values.
// For attribute keys, the value is nil. For block-type keys, the value is
// a nested base map built from all instances' nested block content.
func buildBlockBase(vals []interface{}, schema *tfjson.SchemaBlock) map[string]interface{} {
	base := make(map[string]interface{})
	nestedVals := make(map[string][]interface{})

	for _, val := range vals {
		var maps []map[string]interface{}
		switch v := val.(type) {
		case map[string]interface{}:
			maps = append(maps, v)
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					maps = append(maps, m)
				}
			}
		}
		for _, m := range maps {
			for k, v := range m {
				if _, exists := base[k]; !exists {
					base[k] = nil
				}
				if schema != nil && shouldRenderAsBlock(schema, k) && v != nil {
					nestedVals[k] = append(nestedVals[k], v)
				}
			}
		}
	}

	for k, nv := range nestedVals {
		nestedSchema := nestedBlockSchema(schema, k)
		base[k] = buildBlockBase(nv, nestedSchema)
	}

	return base
}

// normalizeForDynamic prepares a block value for use in a dynamic block's
// lookup map. Single blocks (maps) are wrapped in a single-element array.
// Multiple blocks (arrays) are kept as-is. Nested blocks within maps are
// recursively normalized. The base map defines the expected object shape;
// actual values from val are merged into a copy of base, ensuring consistent
// object shapes across all instances. For block-type keys, base values are
// nested base maps that propagate to recursive calls.
func normalizeForDynamic(val interface{}, schema *tfjson.SchemaBlock, base map[string]interface{}) interface{} {
	// normalizeMap merges actual values from m into a copy of b, filtering
	// empty strings and recursively normalizing nested blocks.
	normalizeMap := func(m map[string]interface{}, b map[string]interface{}) map[string]interface{} {
		result := make(map[string]interface{})
		for k := range b {
			result[k] = nil
		}
		for k, mv := range m {
			if mv == nil {
				continue
			}
			if str, ok := mv.(string); ok && str == "" {
				continue
			}
			if schema != nil && shouldRenderAsBlock(schema, k) {
				nestedSchema := nestedBlockSchema(schema, k)
				var nestedBase map[string]interface{}
				if nb, ok := b[k].(map[string]interface{}); ok {
					nestedBase = nb
				}
				result[k] = normalizeForDynamic(mv, nestedSchema, nestedBase)
			} else {
				result[k] = mv
			}
		}
		return result
	}

	switch v := val.(type) {
	case map[string]interface{}:
		return []interface{}{normalizeMap(v, base)}
	case []interface{}:
		elementBase := buildBlockBase(v, schema)
		result := make([]interface{}, len(v))
		for i, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				result[i] = normalizeMap(m, elementBase)
			} else {
				result[i] = item
			}
		}
		return result
	default:
		return val
	}
}

// buildContentBlock generates the content block for a dynamic block, with
// attribute references to blockName.value.attr for plain attributes and
// nested dynamic blocks for block-type attributes.
func buildContentBlock(blockName string, contentKeys []string, schema *tfjson.SchemaBlock) *hclwrite.Block {
	contentBlock := hclwrite.NewBlock("content", nil)
	contentBody := contentBlock.Body()

	for _, key := range contentKeys {
		if schema != nil && shouldRenderAsBlock(schema, key) {
			// Nested block — generate nested dynamic block
			nestedDyn := buildNestedDynamicBlock(blockName, key, schema)
			if nestedDyn != nil {
				contentBody.AppendBlock(nestedDyn)
			}
		} else {
			// Simple attribute — reference blockName.value.key
			contentBody.SetAttributeRaw(key, hclwrite.Tokens{
				{Type: hclsyntax.TokenIdent, Bytes: []byte(blockName)},
				{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
				{Type: hclsyntax.TokenIdent, Bytes: []byte("value")},
				{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
				{Type: hclsyntax.TokenIdent, Bytes: []byte(key)},
			})
		}
	}

	return contentBlock
}

// buildNestedDynamicBlock generates a nested dynamic block within a content block:
//
//	dynamic "nested_block" {
//	    for_each = try(parent.value.nested_block, [])
//	    content {
//	        attr = nested_block.value.attr
//	    }
//	}
func buildNestedDynamicBlock(parentBlockName, nestedBlockName string, parentSchema *tfjson.SchemaBlock) *hclwrite.Block {
	dynBlock := hclwrite.NewBlock("dynamic", []string{nestedBlockName})
	dynBody := dynBlock.Body()

	// for_each = try(parentBlockName.value.nestedBlockName, [])
	forEachTokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenIdent, Bytes: []byte("try")},
		{Type: hclsyntax.TokenOParen, Bytes: []byte("(")},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(parentBlockName)},
		{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		{Type: hclsyntax.TokenIdent, Bytes: []byte("value")},
		{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(nestedBlockName)},
		{Type: hclsyntax.TokenComma, Bytes: []byte(",")},
		{Type: hclsyntax.TokenIdent, Bytes: []byte(" ")},
		{Type: hclsyntax.TokenOBrack, Bytes: []byte("[")},
		{Type: hclsyntax.TokenCBrack, Bytes: []byte("]")},
		{Type: hclsyntax.TokenCParen, Bytes: []byte(")")},
	}
	dynBody.SetAttributeRaw("for_each", forEachTokens)

	// content { ... }
	nestedSchema := nestedBlockSchema(parentSchema, nestedBlockName)
	contentBlock := hclwrite.NewBlock("content", nil)
	contentBody := contentBlock.Body()

	if nestedSchema != nil {
		// Add attribute references from schema (excluding computed-only)
		if nestedSchema.Attributes != nil {
			var attrKeys []string
			for key, attr := range nestedSchema.Attributes {
				if !isComputedOnly(attr) {
					attrKeys = append(attrKeys, key)
				}
			}
			sort.Strings(attrKeys)
			for _, key := range attrKeys {
				contentBody.SetAttributeRaw(key, hclwrite.Tokens{
					{Type: hclsyntax.TokenIdent, Bytes: []byte(nestedBlockName)},
					{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
					{Type: hclsyntax.TokenIdent, Bytes: []byte("value")},
					{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
					{Type: hclsyntax.TokenIdent, Bytes: []byte(key)},
				})
			}
		}

		// Recurse for deeper nested blocks
		if nestedSchema.NestedBlocks != nil {
			for _, key := range utils.SortedKeys(nestedSchema.NestedBlocks) {
				deeperDyn := buildNestedDynamicBlock(nestedBlockName, key, nestedSchema)
				if deeperDyn != nil {
					contentBody.AppendBlock(deeperDyn)
				}
			}
		}
	}

	dynBody.AppendBlock(contentBlock)
	return dynBlock
}

// buildLookupTokens generates HCL tokens for:
//
//	lookup({ idx1 = val1, idx2 = val2 }, count.index, graft.source)
//
// or for for_each:
//
//	lookup({ "key1" = val1, "key2" = val2 }, each.key, graft.source)
func buildLookupTokens(changes []DriftChange, key string, indexRef string) hclwrite.Tokens {
	// Build the lookup map: index -> attribute value
	lookupMap := make(map[string]interface{})
	for _, change := range changes {
		val, exists := change.ChangedAttrs[key]
		if !exists {
			continue
		}
		lookupMap[change.indexKey()] = val
	}

	return buildLookupExprTokens(lookupMap, indexRef, "graft.source")
}

// buildLookupExprTokens generates HCL tokens for a lookup expression:
//
//	lookup(MAP, indexRef, defaultExpr)
//
// The map is converted to a cty value and rendered as an HCL literal.
// defaultExpr is a raw HCL expression string (e.g. "[]" or "graft.source").
// It is split on "." to produce dot-separated identifier tokens, so it must
// not contain dots within string literals or complex sub-expressions.
func buildLookupExprTokens(lookupMap map[string]interface{}, indexRef string, defaultExpr string) hclwrite.Tokens {
	mapTokens := hclwrite.NewExpressionLiteral(utils.ToCtyValue(lookupMap)).BuildTokens(nil)

	var tokens hclwrite.Tokens

	// lookup(
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("lookup")},
		&hclwrite.Token{Type: hclsyntax.TokenOParen, Bytes: []byte("(")},
	)
	tokens = append(tokens, mapTokens...)

	// , indexRef
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenComma, Bytes: []byte(",")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(" ")},
	)
	parts := strings.Split(indexRef, ".")
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(parts[0])},
		&hclwrite.Token{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(parts[1])},
	)

	// , defaultExpr)
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenComma, Bytes: []byte(",")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(" ")},
	)
	defaultParts := strings.Split(defaultExpr, ".")
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(defaultParts[0])},
	)
	for _, p := range defaultParts[1:] {
		tokens = append(tokens,
			&hclwrite.Token{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
			&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(p)},
		)
	}
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenCParen, Bytes: []byte(")")},
	)

	return tokens
}
