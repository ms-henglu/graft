package patch

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func TestReplaceGraftSourceTokens(t *testing.T) {
	tests := []struct {
		name        string
		inputExpr   string
		replaceExpr string
		expected    string
	}{
		{
			name:        "simple replacement",
			inputExpr:   "graft.source",
			replaceExpr: `"original"`,
			expected:    `"original"`,
		},
		{
			name:        "inside function",
			inputExpr:   `merge(graft.source, { new = "val" })`,
			replaceExpr: `{ old = "val" }`,
			expected:    `merge({ old = "val" }, { new = "val" })`,
		},
		{
			name:        "multiple occurrences",
			inputExpr:   `concat(graft.source, graft.source)`,
			replaceExpr: `["a"]`,
			expected:    `concat(["a"],["a"])`, // Adjusted expectation: no space after comma
		},
		{
			name:        "no graft source",
			inputExpr:   `"static"`,
			replaceExpr: `"original"`,
			expected:    `"static"`,
		},
		{
			name:        "partial match graft",
			inputExpr:   `graft.something`,
			replaceExpr: `"original"`,
			expected:    `graft.something`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputTokens := getTokensFromExpr(tt.inputExpr)
			replaceTokens := getTokensFromExpr(tt.replaceExpr)

			gotTokens := replaceGraftSourceTokens(inputTokens, replaceTokens)

			var got string
			if gotTokens == nil {
				got = string(inputTokens.Bytes())
			} else {
				got = string(gotTokens.Bytes())
			}

			// Trim spaces to avoid issues with hclwrite formatting nuances in tests
			if strings.TrimSpace(got) != strings.TrimSpace(tt.expected) {
				t.Errorf("replaceGraftSourceTokens() mismatch.\nGot:  %q\nWant: %q", got, tt.expected)
				t.Logf("Input tokens: %v", inputTokens)
			}
		})
	}
}

func TestResolveGraftTokens(t *testing.T) {
	testCases := []struct {
		name              string
		overrideHCL       string
		existingLocalsHCL string
		existingBlockHCL  string
		expectedHCL       string
	}{
		{
			name: "locals merge strategy",
			overrideHCL: `
locals {
  tags = merge(graft.source, { env = "dev" })
}
`,
			existingLocalsHCL: `tags = { owner = "me" }`,
			existingBlockHCL:  ``,
			expectedHCL: `
locals {
  tags = merge( { owner = "me" }, { env = "dev" })
}
`,
		},
		{
			name: "resource attribute concat",
			overrideHCL: `
resource "test" "example" {
  lifecycle {
     ignore_changes = concat(graft.source, ["tags"])
  }
}
`,
			existingLocalsHCL: ``,
			existingBlockHCL: `
resource "test" "example" {
  lifecycle {
     ignore_changes = ["id"]
  }
}
`,
			expectedHCL: `
resource "test" "example" {
  lifecycle {
     ignore_changes = concat( ["id"], ["tags"])
  }
}
`,
		},
		{
			name: "top level resource attribute",
			overrideHCL: `
resource "test" "example" {
  tags = merge(graft.source, { new = "val" })
}
`,
			existingLocalsHCL: ``,
			existingBlockHCL: `
resource "test" "example" {
  tags = { old = "val" }
}
`,
			expectedHCL: `
resource "test" "example" {
  tags = merge( { old = "val" }, { new = "val" })
}
`,
		},
		{
			name: "list append in locals",
			overrideHCL: `
locals {
  items = concat(graft.source, ["item3"])
}
`,
			existingLocalsHCL: `items = ["item1", "item2"]`,
			existingBlockHCL:  ``,
			expectedHCL: `
locals {
  items = concat( ["item1", "item2"], ["item3"])
}
`,
		},
		{
			name: "no existing match",
			overrideHCL: `
resource "test" "example" {
  tags = merge(graft.source, { new = "val" })
}
`,
			existingLocalsHCL: ``,
			existingBlockHCL: `
resource "test" "example" {
  # no tags
}
`,
			expectedHCL: `
resource "test" "example" {
  tags = merge(null, { new = "val" })
}
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wrappedHCL := "override {\n" + tc.overrideHCL + "\n}"
			f, _ := hclwrite.ParseConfig([]byte(wrappedHCL), "", hcl.Pos{Line: 1, Column: 1})
			overrideBlocks := f.Body().Blocks()

			var existingLocalsMap map[string]*hclwrite.Attribute
			if tc.existingLocalsHCL != "" {
				existingLocalsMap = createLocalsMapFromString(tc.existingLocalsHCL)
			} else {
				existingLocalsMap = make(map[string]*hclwrite.Attribute)
			}

			existingBlocksMap := make(map[string]*hclwrite.Block)
			if tc.existingBlockHCL != "" {
				fExisting, _ := hclwrite.ParseConfig([]byte(tc.existingBlockHCL), "", hcl.Pos{Line: 1, Column: 1})
				if len(fExisting.Body().Blocks()) > 0 {
					blk := fExisting.Body().Blocks()[0]
					key := blockKey(blk)
					existingBlocksMap[key] = blk
				}
			}

			resolveGraftTokens(overrideBlocks, existingBlocksMap, existingLocalsMap)
			got := string(overrideBlocks[0].Body().BuildTokens(nil).Bytes())
			expectHCL(t, got, tc.expectedHCL)
		})
	}
}

// Helper to get tokens from an expression string
func getTokensFromExpr(expr string) hclwrite.Tokens {
	// Wrap in a dummy attribute to parse. Use no space after = to minimize WS tokens.
	config := "attr=" + expr
	f, _ := hclwrite.ParseConfig([]byte(config), "", hcl.Pos{Line: 1, Column: 1})
	return f.Body().GetAttribute("attr").Expr().BuildTokens(nil)
}

func createLocalsMapFromString(hclSnippet string) map[string]*hclwrite.Attribute {
	f, _ := hclwrite.ParseConfig([]byte(hclSnippet), "", hcl.Pos{Line: 1, Column: 1})
	m := make(map[string]*hclwrite.Attribute)
	for name, attr := range f.Body().Attributes() {
		m[name] = attr
	}
	return m
}

func expectHCL(t *testing.T, got, expected string) {
	t.Helper()
	if strings.TrimSpace(got) != strings.TrimSpace(expected) {
		t.Errorf("Got: %s, Want: %s", got, expected)
	}
}
