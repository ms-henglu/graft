package manifest

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func TestMergeModuleLists(t *testing.T) {
	tests := []struct {
		name     string
		base     []Module
		other    []Module
		expected []string // expected module names in order
	}{
		{
			name:     "empty lists",
			base:     nil,
			other:    nil,
			expected: nil,
		},
		{
			name: "base only",
			base: []Module{
				{Name: "a"},
				{Name: "b"},
			},
			other:    nil,
			expected: []string{"a", "b"},
		},
		{
			name: "other only",
			base: nil,
			other: []Module{
				{Name: "x"},
				{Name: "y"},
			},
			expected: []string{"x", "y"},
		},
		{
			name: "no overlap - preserves order",
			base: []Module{
				{Name: "a"},
				{Name: "b"},
			},
			other: []Module{
				{Name: "c"},
				{Name: "d"},
			},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name: "with overlap - merges and preserves base order",
			base: []Module{
				{Name: "a", Source: "source-a"},
				{Name: "b", Source: "source-b"},
			},
			other: []Module{
				{Name: "b", Version: "2.0"},
				{Name: "c", Source: "source-c"},
			},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModuleLists(tt.base, tt.other)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d modules, got %d", len(tt.expected), len(result))
				return
			}

			for i, name := range tt.expected {
				if result[i].Name != name {
					t.Errorf("expected module[%d].Name = %q, got %q", i, name, result[i].Name)
				}
			}
		})
	}
}

func TestMergeModules(t *testing.T) {
	tests := []struct {
		name           string
		base           Module
		other          Module
		expectedSource string
		expectedVer    string
	}{
		{
			name:           "base values preserved when other is empty",
			base:           Module{Name: "test", Source: "base-source", Version: "1.0"},
			other:          Module{Name: "test"},
			expectedSource: "base-source",
			expectedVer:    "1.0",
		},
		{
			name:           "other values override base (last write wins)",
			base:           Module{Name: "test", Source: "base-source", Version: "1.0"},
			other:          Module{Name: "test", Source: "other-source", Version: "2.0"},
			expectedSource: "other-source",
			expectedVer:    "2.0",
		},
		{
			name:           "partial override - only version",
			base:           Module{Name: "test", Source: "base-source", Version: "1.0"},
			other:          Module{Name: "test", Version: "2.0"},
			expectedSource: "base-source",
			expectedVer:    "2.0",
		},
		{
			name:           "partial override - only source",
			base:           Module{Name: "test", Source: "base-source", Version: "1.0"},
			other:          Module{Name: "test", Source: "other-source"},
			expectedSource: "other-source",
			expectedVer:    "1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeModules(tt.base, tt.other)

			if result.Source != tt.expectedSource {
				t.Errorf("expected Source = %q, got %q", tt.expectedSource, result.Source)
			}
			if result.Version != tt.expectedVer {
				t.Errorf("expected Version = %q, got %q", tt.expectedVer, result.Version)
			}
		})
	}
}

func TestMergeModules_NestedModules(t *testing.T) {
	base := Module{
		Name: "parent",
		Modules: []Module{
			{Name: "child1", Source: "child1-source"},
		},
	}
	other := Module{
		Name: "parent",
		Modules: []Module{
			{Name: "child2", Source: "child2-source"},
		},
	}

	result := mergeModules(base, other)

	if len(result.Modules) != 2 {
		t.Errorf("expected 2 nested modules, got %d", len(result.Modules))
		return
	}

	names := make(map[string]bool)
	for _, m := range result.Modules {
		names[m.Name] = true
	}

	if !names["child1"] || !names["child2"] {
		t.Errorf("expected both child1 and child2 in nested modules")
	}
}

func TestMergeOverrideBlocks(t *testing.T) {
	tests := []struct {
		name          string
		baseHCL       string
		otherHCL      string
		expectedCount int
		checkFunc     func(t *testing.T, result []*hclwrite.Block)
	}{
		{
			name:          "empty base returns other",
			baseHCL:       "",
			otherHCL:      `resource "aws_vpc" "main" { cidr = "10.0.0.0/16" }`,
			expectedCount: 1,
		},
		{
			name:          "empty other returns base",
			baseHCL:       `resource "aws_vpc" "main" { cidr = "10.0.0.0/16" }`,
			otherHCL:      "",
			expectedCount: 1,
		},
		{
			name:          "no overlap - combines blocks",
			baseHCL:       `resource "aws_vpc" "main" { cidr = "10.0.0.0/16" }`,
			otherHCL:      `resource "aws_subnet" "public" { cidr = "10.0.1.0/24" }`,
			expectedCount: 2,
		},
		{
			name:          "same block - merges attributes",
			baseHCL:       `resource "aws_vpc" "main" { cidr = "10.0.0.0/16" }`,
			otherHCL:      `resource "aws_vpc" "main" { tags = { Name = "test" } }`,
			expectedCount: 1,
			checkFunc: func(t *testing.T, result []*hclwrite.Block) {
				block := result[0]
				attrs := block.Body().Attributes()
				if len(attrs) != 2 {
					t.Errorf("expected 2 attributes after merge, got %d", len(attrs))
				}
			},
		},
		{
			name:          "same block same attr - last write wins",
			baseHCL:       `resource "aws_vpc" "main" { cidr = "10.0.0.0/16" }`,
			otherHCL:      `resource "aws_vpc" "main" { cidr = "192.168.0.0/16" }`,
			expectedCount: 1,
			checkFunc: func(t *testing.T, result []*hclwrite.Block) {
				block := result[0]
				attr := block.Body().GetAttribute("cidr")
				if attr == nil {
					t.Error("expected cidr attribute")
					return
				}
				val := strings.TrimSpace(string(attr.Expr().BuildTokens(nil).Bytes()))
				if !strings.Contains(val, "192.168.0.0/16") {
					t.Errorf("expected cidr to be overwritten to 192.168.0.0/16, got %s", val)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var base, other []*hclwrite.Block

			if tt.baseHCL != "" {
				f, diags := hclwrite.ParseConfig([]byte(tt.baseHCL), "base.hcl", hcl.Pos{Line: 1, Column: 1})
				if diags.HasErrors() {
					t.Fatalf("failed to parse baseHCL: %s", diags.Error())
				}
				base = f.Body().Blocks()
			}

			if tt.otherHCL != "" {
				f, diags := hclwrite.ParseConfig([]byte(tt.otherHCL), "other.hcl", hcl.Pos{Line: 1, Column: 1})
				if diags.HasErrors() {
					t.Fatalf("failed to parse otherHCL: %s", diags.Error())
				}
				other = f.Body().Blocks()
			}

			result := mergeOverrideBlocks(base, other)

			if len(result) != tt.expectedCount {
				t.Errorf("expected %d blocks, got %d", tt.expectedCount, len(result))
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestBlockKey(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		labels   []string
		expected string
	}{
		{
			name:     "resource with two labels",
			typ:      "resource",
			labels:   []string{"aws_vpc", "main"},
			expected: "resource:aws_vpc.main",
		},
		{
			name:     "data with two labels",
			typ:      "data",
			labels:   []string{"aws_ami", "ubuntu"},
			expected: "data:aws_ami.ubuntu",
		},
		{
			name:     "locals with no labels",
			typ:      "locals",
			labels:   nil,
			expected: "locals:",
		},
		{
			name:     "variable with one label",
			typ:      "variable",
			labels:   []string{"instance_type"},
			expected: "variable:instance_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := hclwrite.NewBlock(tt.typ, tt.labels)
			result := blockKey(block)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMergeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		baseHCL  string
		otherHCL string
		check    func(t *testing.T, result *hclwrite.Block)
	}{
		{
			name:     "merges different attributes",
			baseHCL:  `resource "test" "example" { foo = "a" }`,
			otherHCL: `resource "test" "example" { bar = "b" }`,
			check: func(t *testing.T, result *hclwrite.Block) {
				attrs := result.Body().Attributes()
				if len(attrs) != 2 {
					t.Errorf("expected 2 attributes, got %d", len(attrs))
				}
				if attrs["foo"] == nil {
					t.Error("expected 'foo' attribute")
				}
				if attrs["bar"] == nil {
					t.Error("expected 'bar' attribute")
				}
			},
		},
		{
			name:     "last write wins for same attribute",
			baseHCL:  `resource "test" "example" { foo = "old" }`,
			otherHCL: `resource "test" "example" { foo = "new" }`,
			check: func(t *testing.T, result *hclwrite.Block) {
				attr := result.Body().GetAttribute("foo")
				val := strings.TrimSpace(string(attr.Expr().BuildTokens(nil).Bytes()))
				if !strings.Contains(val, "new") {
					t.Errorf("expected foo to be 'new', got %s", val)
				}
			},
		},
		{
			name: "merges nested blocks",
			baseHCL: `resource "test" "example" {
  lifecycle {
    create_before_destroy = true
  }
}`,
			otherHCL: `resource "test" "example" {
  timeouts {
    create = "30m"
  }
}`,
			check: func(t *testing.T, result *hclwrite.Block) {
				blocks := result.Body().Blocks()
				if len(blocks) != 2 {
					t.Errorf("expected 2 nested blocks, got %d", len(blocks))
				}
			},
		},
		{
			name: "merges same nested block",
			baseHCL: `resource "test" "example" {
  lifecycle {
    create_before_destroy = true
  }
}`,
			otherHCL: `resource "test" "example" {
  lifecycle {
    prevent_destroy = true
  }
}`,
			check: func(t *testing.T, result *hclwrite.Block) {
				blocks := result.Body().Blocks()
				if len(blocks) != 1 {
					t.Errorf("expected 1 merged lifecycle block, got %d", len(blocks))
					return
				}
				lifecycleAttrs := blocks[0].Body().Attributes()
				if len(lifecycleAttrs) != 2 {
					t.Errorf("expected 2 attributes in lifecycle block, got %d", len(lifecycleAttrs))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fBase, _ := hclwrite.ParseConfig([]byte(tt.baseHCL), "base.hcl", hcl.Pos{Line: 1, Column: 1})
			fOther, _ := hclwrite.ParseConfig([]byte(tt.otherHCL), "other.hcl", hcl.Pos{Line: 1, Column: 1})

			base := fBase.Body().Blocks()[0]
			other := fOther.Body().Blocks()[0]

			result := mergeBlocks(base, other)
			tt.check(t, result)
		})
	}
}

func TestCopyBlockContents(t *testing.T) {
	srcHCL := `resource "test" "example" {
  foo = "bar"
  count = 1
  
  nested {
    inner = "value"
  }
}`
	f, _ := hclwrite.ParseConfig([]byte(srcHCL), "src.hcl", hcl.Pos{Line: 1, Column: 1})
	src := f.Body().Blocks()[0]

	dst := hclwrite.NewBlock("resource", []string{"test", "example"})
	copyBlockContents(src, dst)

	// Check attributes copied
	attrs := dst.Body().Attributes()
	if len(attrs) != 2 {
		t.Errorf("expected 2 attributes copied, got %d", len(attrs))
	}

	// Check nested blocks copied
	blocks := dst.Body().Blocks()
	if len(blocks) != 1 {
		t.Errorf("expected 1 nested block copied, got %d", len(blocks))
	}

	if blocks[0].Type() != "nested" {
		t.Errorf("expected nested block type 'nested', got %q", blocks[0].Type())
	}
}

func TestMergeOverrideBlocks_PreservesOrder(t *testing.T) {
	baseHCL := `
resource "aws_vpc" "main" {}
resource "aws_subnet" "a" {}
resource "aws_subnet" "b" {}
`
	otherHCL := `
resource "aws_security_group" "sg" {}
resource "aws_subnet" "a" { tags = {} }
`
	fBase, _ := hclwrite.ParseConfig([]byte(baseHCL), "base.hcl", hcl.Pos{Line: 1, Column: 1})
	fOther, _ := hclwrite.ParseConfig([]byte(otherHCL), "other.hcl", hcl.Pos{Line: 1, Column: 1})

	result := mergeOverrideBlocks(fBase.Body().Blocks(), fOther.Body().Blocks())

	// Expected order: aws_vpc.main, aws_subnet.a, aws_subnet.b, aws_security_group.sg
	expectedOrder := []string{
		"resource:aws_vpc.main",
		"resource:aws_subnet.a",
		"resource:aws_subnet.b",
		"resource:aws_security_group.sg",
	}

	if len(result) != len(expectedOrder) {
		t.Fatalf("expected %d blocks, got %d", len(expectedOrder), len(result))
	}

	for i, block := range result {
		key := blockKey(block)
		if key != expectedOrder[i] {
			t.Errorf("expected block[%d] = %q, got %q", i, expectedOrder[i], key)
		}
	}
}
