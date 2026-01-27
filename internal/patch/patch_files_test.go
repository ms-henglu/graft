package patch

import (
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func TestGenerateOverrideFile(t *testing.T) {
	testcases := []struct {
		name           string
		overrideHCL    string
		existingBlocks map[string]*hclwrite.Block
		existingLocals map[string]*hclwrite.Attribute
		expected       string
	}{
		{
			name: "basic override separation",
			overrideHCL: `
override {
  resource "test" "existing" {
    bucket = "new"
  }
  
  resource "test" "new" {
    bucket = "add"
  }

  locals {
    foo = "bar"
  }

  # Attribute at top level
  version = "2.0"
}
`,
			existingBlocks: map[string]*hclwrite.Block{
				"resource.test.existing": hclwrite.NewBlock("resource", []string{"test", "existing"}),
			},
			existingLocals: createLocalsMap(map[string]cty.Value{
				"foo": cty.StringVal("val"),
			}),
			expected: `version = "2.0"
resource "test" "existing" {
  bucket = "new"
}
locals {
  foo = "bar"
}
`,
		},
		{
			name: "partial local override",
			overrideHCL: `
override {
  locals {
    exists = "1"
    new_one = "2"
  }
}
`,
			existingBlocks: map[string]*hclwrite.Block{},
			existingLocals: createLocalsMap(map[string]cty.Value{
				"exists": cty.StringVal("old"),
			}),
			expected: `locals {
  exists = "1"
}
`,
		},
		{
			name: "graft source replacement in resource",
			overrideHCL: `
override {
  resource "test" "existing" {
    tags = merge(graft.source, { new = "val" })
  }
}
`,
			existingBlocks: map[string]*hclwrite.Block{
				"resource.test.existing": createBlockWithAttribute("resource", []string{"test", "existing"}, map[string]cty.Value{
					"tags": cty.ObjectVal(map[string]cty.Value{
						"old": cty.StringVal("val"),
					}),
				}),
			},
			existingLocals: nil,
			expected: `resource "test" "existing" {
  tags = merge({
    old = "val"
  }, { new = "val" })
}
`,
		},
		{
			name: "graft source replacement in locals",
			overrideHCL: `
override {
  locals {
    common = merge(graft.source, { new = "val" })
  }
}
`,
			existingBlocks: nil,
			existingLocals: createLocalsMap(map[string]cty.Value{
				"common": cty.ObjectVal(map[string]cty.Value{
					"old": cty.StringVal("val"),
				}),
			}),
			expected: `locals {
  common = merge({
    old = "val"
  }, { new = "val" })
}
`,
		},
		{
			name: "graft source list append",
			overrideHCL: `
override {
  resource "test" "list" {
    list_attr = concat(graft.source, ["new"])
  }
}
`,
			existingBlocks: map[string]*hclwrite.Block{
				"resource.test.list": createBlockWithAttribute("resource", []string{"test", "list"}, map[string]cty.Value{
					"list_attr": cty.ListVal([]cty.Value{cty.StringVal("old")}),
				}),
			},
			existingLocals: nil,
			expected: `resource "test" "list" {
  list_attr = concat(["old"], ["new"])
}
`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			f, diags := hclwrite.ParseConfig([]byte(tc.overrideHCL), "override.hcl", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatal(diags.Error())
			}
			overrideBlock := f.Body().FirstMatchingBlock("override", nil)
			overrideBlocks := []*hclwrite.Block{overrideBlock}

			resolveGraftTokens(overrideBlocks, tc.existingBlocks, tc.existingLocals)

			gotFile := generateOverrideFile(overrideBlocks, tc.existingBlocks, tc.existingLocals)
			got := string(gotFile.Bytes())

			if strings.TrimSpace(got) != strings.TrimSpace(tc.expected) {
				t.Errorf("generateOverrideFile output mismatch.\nGot:\n%s\nExpected:\n%s", got, tc.expected)
			}
		})
	}
}

func TestGenerateAddFile(t *testing.T) {
	testcases := []struct {
		name           string
		overrideHCL    string
		existingBlocks map[string]*hclwrite.Block
		existingLocals map[string]*hclwrite.Attribute
		expected       string
	}{
		{
			name: "basic add separation",
			overrideHCL: `
override {
  resource "test" "existing" {
    bucket = "new"
  }
  
  resource "test" "new" {
    bucket = "add"
  }
}
`,
			existingBlocks: map[string]*hclwrite.Block{
				"resource.test.existing": hclwrite.NewBlock("resource", []string{"test", "existing"}),
			},
			existingLocals: map[string]*hclwrite.Attribute{},
			expected: `resource "test" "new" {
  bucket = "add"
}
`,
		},
		{
			name: "partial local addition",
			overrideHCL: `
override {
  locals {
    exists = "1"
    new_one = "2"
  }
}
`,
			existingBlocks: map[string]*hclwrite.Block{},
			existingLocals: createLocalsMap(map[string]cty.Value{
				"exists": cty.StringVal("val"),
			}),
			expected: `locals {
  new_one = "2"
}
`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			f, diags := hclwrite.ParseConfig([]byte(tc.overrideHCL), "override.hcl", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatal(diags.Error())
			}
			overrideBlock := f.Body().FirstMatchingBlock("override", nil)
			overrideBlocks := []*hclwrite.Block{overrideBlock}

			gotFile := generateAddFile(overrideBlocks, tc.existingBlocks, tc.existingLocals)
			got := string(gotFile.Bytes())

			if strings.TrimSpace(got) != strings.TrimSpace(tc.expected) {
				t.Errorf("generateAddFile output mismatch.\nGot:\n%s\nExpected:\n%s", got, tc.expected)
			}
		})
	}
}

func createLocalsMap(localsMap map[string]cty.Value) map[string]*hclwrite.Attribute {
	block := createBlockWithAttribute("locals", nil, localsMap)
	return block.Body().Attributes()
}

func createBlockWithAttribute(typeName string, labels []string, attrMap map[string]cty.Value) *hclwrite.Block {
	b := hclwrite.NewBlock(typeName, labels)
	for attrName, attrValue := range attrMap {
		b.Body().SetAttributeValue(attrName, attrValue)
	}
	return b
}

// Helper to confirm blockKey behaves as expected since we mock keys
func TestBlockKeyHelper(t *testing.T) {
	b := hclwrite.NewBlock("resource", []string{"a", "b"})
	key := blockKey(b)
	expected := "resource.a.b"
	if key != expected {
		t.Errorf("blockKey = %q, want %q", key, expected)
	}
}
