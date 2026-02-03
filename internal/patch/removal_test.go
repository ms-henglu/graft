package patch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func TestParseRemovals(t *testing.T) {
	tests := []struct {
		name     string
		hcl      string
		expected []string
	}{
		{
			name: "single item",
			hcl: `
_graft {
  remove = ["foo"]
}
`,
			expected: []string{"foo"},
		},
		{
			name: "multiple items",
			hcl: `
_graft {
  remove = ["foo", "bar.baz"]
}
`,
			expected: []string{"foo", "bar.baz"},
		},
		{
			name: "empty list",
			hcl: `
_graft {
  remove = []
}
`,
			expected: []string(nil),
		},
		{
			name: "no remove attr",
			hcl: `
_graft {
  other = "val"
}
`,
			expected: []string(nil),
		},
		{
			name: "invalid type (string instead of list)",
			hcl: `
_graft {
  remove = "foo"
}
`,
			expected: []string(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, diags := hclwrite.ParseConfig([]byte(tt.hcl), "test.hcl", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("failed to parse hcl: %s", diags.Error())
			}
			block := f.Body().FirstMatchingBlock("_graft", nil)
			if block == nil {
				t.Fatal("graft block not found")
			}

			got := parseRemovals(block)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseRemovals() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestApplyRemovals(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		override string
		expected map[string]string
	}{
		{
			name: "remove self",
			files: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  name = "test"
}
`,
			},
			override: `
override {
  resource "foo" "bar" {
    _graft {
      remove = ["self"]
    }
  }
}
`,
			expected: map[string]string{
				"main.tf": "\n",
			},
		},
		{
			name: "remove attribute",
			files: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  name = "test"
  tags = {
    env = "dev"
  }
}
`,
			},
			override: `
override {
  resource "foo" "bar" {
    _graft {
      remove = ["tags"]
    }
  }
}
`,
			expected: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  name = "test"
}
`,
			},
		},
		{
			name: "remove nested attribute",
			files: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  network_rule {
    ip_address = "1.2.3.4"
    mask       = "24"
  }
}
`,
			},
			override: `
override {
  resource "foo" "bar" {
    _graft {
      remove = ["network_rule.mask"]
    }
  }
}
`,
			expected: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  network_rule {
    ip_address = "1.2.3.4"
  }
}
`,
			},
		},
		{
			name: "remove nested block",
			files: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  network_rule {
    ip_address = "1.2.3.4"
  }
  network_rule {
    ip_address = "5.6.7.8"
  }
}
`,
			},
			override: `
override {
  resource "foo" "bar" {
    _graft {
      remove = ["network_rule"]
    }
  }
}
`,
			expected: map[string]string{
				"main.tf": `
resource "foo" "bar" {
}
`,
			},
		},
		{
			name: "remove dynamic block by label",
			files: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  name = "test"
  dynamic "subnet" {
    for_each = var.subnets
    content {
      name = subnet.value.name
    }
  }
}
`,
			},
			override: `
override {
  resource "foo" "bar" {
    _graft {
      remove = ["subnet"]
    }
  }
}
`,
			expected: map[string]string{
				"main.tf": `
resource "foo" "bar" {
  name = "test"
}
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Create input files
			for name, content := range tt.files {
				err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
				if err != nil {
					t.Fatalf("failed to create file %s: %v", name, err)
				}
			}

			// Parse override blocks
			f, diags := hclwrite.ParseConfig([]byte(tt.override), "override.hcl", hcl.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				t.Fatalf("failed to parse override: %s", diags.Error())
			}

			// Flatten: extract content blocks from override wrappers
			var overrides []*hclwrite.Block
			for _, b := range f.Body().Blocks() {
				if b.Type() == "override" {
					overrides = append(overrides, b.Body().Blocks()...)
				}
			}

			err := applyRemovals(dir, overrides)
			if err != nil {
				t.Fatalf("applyRemovals failed: %v", err)
			}

			// Check results
			for name, expectedContent := range tt.expected {
				content, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatalf("failed to read file %s: %v", name, err)
				}

				// Normalize line endings and trim spaces for comparison to avoid whitespace issues
				got := string(hclwrite.Format(content))
				expected := string(hclwrite.Format([]byte(expectedContent)))

				if got != expected {
					t.Errorf("file %s content mismatch.\nGot:\n%s\nExpected:\n%s", name, got, expected)
				}
			}
		})
	}
}
