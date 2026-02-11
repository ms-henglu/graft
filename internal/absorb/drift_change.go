package absorb

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// DriftChange represents a single resource with drift
type DriftChange struct {
	Address      string
	ModulePath   []string
	ResourceType string
	ResourceName string
	ProviderName string
	Mode         string
	Index        interface{} // nil = no index, float64 = count, string = for_each
	ChangedAttrs map[string]interface{}
	BeforeAttrs  map[string]interface{}
}

// IsIndexed returns true if the resource uses count or for_each.
func (change DriftChange) IsIndexed() bool {
	return change.Index != nil
}

// IsCountIndexed returns true if the resource uses count (numeric index).
func (change DriftChange) IsCountIndexed() bool {
	_, ok := change.Index.(float64)
	return ok
}

// IsForEachIndexed returns true if the resource uses for_each (string key).
func (change DriftChange) IsForEachIndexed() bool {
	_, ok := change.Index.(string)
	return ok
}

// indexKey returns a string suitable for use as a map key in the lookup expression.
// For count: "0", "1", etc. For for_each: the key string itself.
func (change DriftChange) indexKey() string {
	switch idx := change.Index.(type) {
	case float64:
		return fmt.Sprintf("%d", int(idx))
	case string:
		return idx
	default:
		return ""
	}
}

// indexRef returns the HCL expression referencing the current instance index.
// For count: "count.index". For for_each: "each.key".
func (change DriftChange) indexRef() string {
	if change.IsCountIndexed() {
		return "count.index"
	}
	return "each.key"
}

func (change DriftChange) ToBlock(schema *tfjson.SchemaBlock) *hclwrite.Block {
	// Filter computed attributes when schema is available
	result := filterComputedAttrs(change.ChangedAttrs, schema, "")
	attrs, _ := result.(map[string]interface{})
	if len(attrs) == 0 {
		return nil
	}

	// Narrow to minimal diff using before state and schema
	if change.BeforeAttrs != nil && schema != nil {
		attrs = deepDiffBlock(change.BeforeAttrs, attrs, schema)
		if len(attrs) == 0 {
			return nil
		}
	}

	resBlock := hclwrite.NewBlock("resource", []string{change.ResourceType, change.ResourceName})
	resBody := resBlock.Body()

	// Collect block-type attributes that need _graft removal
	var removals []string

	var keys []string
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := attrs[key]
		if shouldRenderAsBlock(schema, key) {
			blocks := toBlocks(key, value, schema)
			for _, b := range blocks {
				resBody.AppendBlock(b)
			}
			if len(blocks) > 1 {
				removals = append(removals, key)
			} else if len(blocks) == 1 {
				// Single block — check for nested multi-blocks that need removal
				nestedRemovals := collectNestedRemovals(key, blocks[0])
				removals = append(removals, nestedRemovals...)
			}
		} else {
			ctyVal := interfaceToCtyValue(value)
			resBody.SetAttributeValue(key, ctyVal)
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

// collectNestedRemovals inspects the child blocks of a single HCL block to find
// block types that appear more than once, returning their dotted paths (e.g.
// "backend_http_settings.connection_draining"). It recurses into block types
// that appear exactly once to discover deeply nested multi-blocks.
func collectNestedRemovals(prefix string, block *hclwrite.Block) []string {
	// Count occurrences of each child block type
	counts := make(map[string]int)
	for _, child := range block.Body().Blocks() {
		counts[child.Type()]++
	}

	var removals []string
	visited := make(map[string]bool)
	for _, child := range block.Body().Blocks() {
		blockType := child.Type()
		if visited[blockType] {
			continue
		}
		visited[blockType] = true
		path := prefix + "." + blockType
		if counts[blockType] > 1 {
			removals = append(removals, path)
		} else {
			removals = append(removals, collectNestedRemovals(path, child)...)
		}
	}
	return removals
}

func toBlocks(blockName string, value interface{}, schema *tfjson.SchemaBlock) []*hclwrite.Block {
	out := make([]*hclwrite.Block, 0)
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				out = append(out, toBlocks(blockName, itemMap, schema)...)
			}
		}
	case map[string]interface{}:
		block := hclwrite.NewBlock(blockName, nil)
		// Get the nested schema for this block
		nestedSchema := nestedBlockSchema(schema, blockName)

		var keys []string
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			attrValue := v[key]
			if attrValue == nil {
				continue
			}
			// Skip empty strings in nested blocks.
			// Known limitation: legitimate empty-string values (e.g.
			// description = "") are also dropped. A future improvement
			// could use the provider schema to distinguish intentional
			// empty strings from default/unset values.
			if str, ok := attrValue.(string); ok && str == "" {
				continue
			}

			// Recursively handle nested blocks within blocks
			if nestedSchema != nil && shouldRenderAsBlock(nestedSchema, key) {
				blocks := toBlocks(key, attrValue, nestedSchema)
				for _, b := range blocks {
					block.Body().AppendBlock(b)
				}
				continue
			}

			ctyVal := interfaceToCtyValue(attrValue)
			block.Body().SetAttributeValue(key, ctyVal)
		}
		out = append(out, block)
	}
	return out
}

// deepDiffBlock performs a schema-aware recursive diff between before and after
// maps, returning only the changed attributes in after. For single nested blocks
// (map values where shouldRenderAsBlock is true), it recurses to find the minimal
// diff. For multiple blocks (slice values where shouldRenderAsBlock is true) and
// plain attributes, it captures the full after value.
func deepDiffBlock(before, after map[string]interface{}, schema *tfjson.SchemaBlock) map[string]interface{} {
	if len(after) == 0 {
		return nil
	}

	result := make(map[string]interface{})

	for key, afterVal := range after {
		beforeVal, hasBefore := before[key]

		// If there's no before value, the attribute is new — capture full value
		if !hasBefore {
			result[key] = afterVal
			continue
		}

		// If the values are equal, skip
		if deepEqual(beforeVal, afterVal) {
			continue
		}

		// Check if this key should be rendered as a block
		if shouldRenderAsBlock(schema, key) {
			afterMap, beforeMap, isSingleBlock := extractSingleBlock(afterVal, beforeVal)

			// Single block — recurse for minimal diff
			if isSingleBlock {
				nestedSchema := nestedBlockSchema(schema, key)
				if nestedSchema != nil {
					diffed := deepDiffBlock(beforeMap, afterMap, nestedSchema)
					if len(diffed) > 0 {
						result[key] = diffed
					}
				} else {
					// No nested schema available — capture full value
					result[key] = afterVal
				}
				continue
			}

			// Multiple blocks (slice) — capture full array (needed for _graft remove)
			result[key] = afterVal
			continue
		}

		// Plain attribute (scalars, maps like tags) — capture full value
		result[key] = afterVal
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// extractSingleBlock checks whether both after and before values represent a
// single block. In real Terraform plan JSON, single blocks are encoded as a
// one-element array (e.g. "os_disk": [{...}]), but in test data they may appear
// as plain maps. This function handles both representations and returns the
// inner maps if exactly one block is present on each side.
func extractSingleBlock(afterVal, beforeVal interface{}) (afterMap, beforeMap map[string]interface{}, ok bool) {
	afterMap, afterOk := asSingleBlockMap(afterVal)
	beforeMap, beforeOk := asSingleBlockMap(beforeVal)
	if afterOk && beforeOk {
		return afterMap, beforeMap, true
	}
	return nil, nil, false
}

// asSingleBlockMap extracts a single map from a value that is either a plain
// map or a one-element slice containing a map.
func asSingleBlockMap(val interface{}) (map[string]interface{}, bool) {
	if m, ok := val.(map[string]interface{}); ok {
		return m, true
	}
	if arr, ok := val.([]interface{}); ok && len(arr) == 1 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			return m, true
		}
	}
	return nil, false
}

func interfaceToCtyValue(v interface{}) cty.Value {
	if v == nil {
		return cty.NullVal(cty.DynamicPseudoType)
	}

	switch val := v.(type) {
	case bool:
		return cty.BoolVal(val)
	case float64:
		if val == float64(int64(val)) {
			return cty.NumberIntVal(int64(val))
		}
		return cty.NumberFloatVal(val)
	case string:
		return cty.StringVal(val)
	case []interface{}:
		if len(val) == 0 {
			return cty.ListValEmpty(cty.DynamicPseudoType)
		}
		var vals []cty.Value
		for _, item := range val {
			vals = append(vals, interfaceToCtyValue(item))
		}
		return cty.TupleVal(vals)
	case map[string]interface{}:
		if len(val) == 0 {
			return cty.MapValEmpty(cty.DynamicPseudoType)
		}
		vals := make(map[string]cty.Value)
		for k, item := range val {
			vals[k] = interfaceToCtyValue(item)
		}
		return cty.ObjectVal(vals)
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return cty.StringVal(fmt.Sprintf("%v", v))
		}
		ctyVal, err := ctyjson.Unmarshal(jsonBytes, cty.DynamicPseudoType)
		if err != nil {
			return cty.StringVal(string(jsonBytes))
		}
		return ctyVal
	}
}

// resourceKey returns a grouping key for indexed resources: "type.name".
func (change DriftChange) resourceKey() string {
	return change.ResourceType + "." + change.ResourceName
}

// IndexedChangesToBlock generates a single resource block for a group of indexed
// DriftChange entries (same ResourceType+ResourceName). For Category 1 (root-level
// attributes), each attribute is rendered as:
//
//	attr = lookup({ idx1 = val1, idx2 = val2 }, count.index/each.key, graft.source)
//
// Category 2/3 (block drift) is not yet supported for indexed resources and falls
// back to emitting blocks from the first change (lossy).
func IndexedChangesToBlock(changes []DriftChange, schema *tfjson.SchemaBlock) *hclwrite.Block {
	if len(changes) == 0 {
		return nil
	}

	first := changes[0]
	indexRef := first.indexRef()

	// Pre-process each change: filter computed attrs and compute deep diff
	var processed []DriftChange
	for _, change := range changes {
		result := filterComputedAttrs(change.ChangedAttrs, schema, "")
		attrs, _ := result.(map[string]interface{})
		if len(attrs) == 0 {
			continue
		}
		if change.BeforeAttrs != nil && schema != nil {
			attrs = deepDiffBlock(change.BeforeAttrs, attrs, schema)
			if len(attrs) == 0 {
				continue
			}
		}
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
	var sortedKeys []string
	for key := range allKeys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	// Check if any key is a block type — those are not yet supported with lookup
	hasBlockDrift := false
	var blockKeys []string
	var attrKeys []string
	for _, key := range sortedKeys {
		if shouldRenderAsBlock(schema, key) {
			hasBlockDrift = true
			blockKeys = append(blockKeys, key)
		} else {
			attrKeys = append(attrKeys, key)
		}
	}

	resBlock := hclwrite.NewBlock("resource", []string{first.ResourceType, first.ResourceName})
	resBody := resBlock.Body()

	// Render attribute drift using lookup()
	for _, key := range attrKeys {
		tokens := buildLookupTokens(processed, key, indexRef, first.IsCountIndexed())
		resBody.SetAttributeRaw(key, tokens)
	}

	// Render block drift — fall back to first change's blocks (lossy, for now)
	var removals []string
	if hasBlockDrift {
		for _, key := range blockKeys {
			// Find first instance that has this block
			for _, pc := range processed {
				val, exists := pc.ChangedAttrs[key]
				if !exists {
					continue
				}
				blocks := toBlocks(key, val, schema)
				for _, b := range blocks {
					resBody.AppendBlock(b)
				}
				if len(blocks) > 1 {
					removals = append(removals, key)
				} else if len(blocks) == 1 {
					nestedRemovals := collectNestedRemovals(key, blocks[0])
					removals = append(removals, nestedRemovals...)
				}
				break
			}
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

// buildLookupTokens generates HCL tokens for:
//
//	lookup({ idx1 = val1, idx2 = val2 }, count.index, graft.source)
//
// or for for_each:
//
//	lookup({ "key1" = val1, "key2" = val2 }, each.key, graft.source)
func buildLookupTokens(changes []DriftChange, key string, indexRef string, isCount bool) hclwrite.Tokens {
	var tokens hclwrite.Tokens

	// lookup(
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("lookup")},
		&hclwrite.Token{Type: hclsyntax.TokenOParen, Bytes: []byte("(")},
	)

	// Opening brace for map
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenOBrace, Bytes: []byte("{")},
		&hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
	)

	// Sort the changes by index for deterministic output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].indexKey() < changes[j].indexKey()
	})

	for _, change := range changes {
		val, exists := change.ChangedAttrs[key]
		if !exists {
			continue
		}

		idx := change.indexKey()

		// Indent
		tokens = append(tokens,
			&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("    ")},
		)

		// Key: for count use bare number, for for_each use quoted string
		if isCount {
			tokens = append(tokens,
				&hclwrite.Token{Type: hclsyntax.TokenNumberLit, Bytes: []byte(idx)},
			)
		} else {
			tokens = append(tokens,
				&hclwrite.Token{Type: hclsyntax.TokenOQuote, Bytes: []byte("\"")},
				&hclwrite.Token{Type: hclsyntax.TokenStringLit, Bytes: []byte(idx)},
				&hclwrite.Token{Type: hclsyntax.TokenCQuote, Bytes: []byte("\"")},
			)
		}

		// = value
		tokens = append(tokens,
			&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(" ")},
			&hclwrite.Token{Type: hclsyntax.TokenEqual, Bytes: []byte("=")},
			&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(" ")},
		)

		// Render the value as tokens
		valTokens := valueToTokens(val)
		tokens = append(tokens, valTokens...)

		tokens = append(tokens,
			&hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
		)
	}

	// Closing brace for map
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("  ")},
		&hclwrite.Token{Type: hclsyntax.TokenCBrace, Bytes: []byte("}")},
	)

	// , count.index/each.key
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenComma, Bytes: []byte(",")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(" ")},
	)

	// Render indexRef (count.index or each.key)
	parts := strings.Split(indexRef, ".")
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(parts[0])},
		&hclwrite.Token{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(parts[1])},
	)

	// , graft.source)
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenComma, Bytes: []byte(",")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(" ")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("graft")},
		&hclwrite.Token{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("source")},
		&hclwrite.Token{Type: hclsyntax.TokenCParen, Bytes: []byte(")")},
	)

	return tokens
}

// valueToTokens renders an interface{} value as HCL tokens.
func valueToTokens(v interface{}) hclwrite.Tokens {
	// Use hclwrite to render the value, then extract the expression tokens
	ctyVal := interfaceToCtyValue(v)
	f := hclwrite.NewEmptyFile()
	f.Body().SetAttributeValue("_tmp", ctyVal)
	attr := f.Body().GetAttribute("_tmp")
	if attr == nil {
		return hclwrite.Tokens{
			{Type: hclsyntax.TokenIdent, Bytes: []byte("null")},
		}
	}
	return attr.Expr().BuildTokens(nil)
}
