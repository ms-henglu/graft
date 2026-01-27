package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/ms-henglu/graft/internal/tree"
	"github.com/ms-henglu/graft/internal/vendors"
)

func TestScanResources(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string]string
		expectedCount     int
		expectedResources []string
	}{
		{
			name: "basic resources",
			files: map[string]string{
				"main.tf": `
resource "aws_instance" "web" {
  ami = "ami-12345678"
}
resource "aws_s3_bucket" "bucket" {
  bucket = "my-bucket"
}
`,
			},
			expectedCount: 2,
			expectedResources: []string{
				"aws_instance.web",
				"aws_s3_bucket.bucket",
			},
		},
		{
			name: "ignore graft files",
			files: map[string]string{
				"main.tf":            `resource "r1" "n1" {}`,
				"_graft_override.tf": `resource "r2" "n2" {}`,
			},
			expectedCount: 1,
			expectedResources: []string{
				"r1.n1",
			},
		},
		{
			name: "ignore invalid hcl",
			files: map[string]string{
				"valid.tf":   `resource "r1" "n1" {}`,
				"invalid.tf": `resource "r2" {`,
			},
			expectedCount: 1,
			expectedResources: []string{
				"r1.n1",
			},
		},
		{
			name:          "empty directory",
			files:         map[string]string{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "scaffold_test")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			for filename, content := range tt.files {
				err = os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
				if err != nil {
					t.Fatal(err)
				}
			}

			resources, err := ScanResources(tmpDir)
			if err != nil {
				t.Fatalf("ScanResources failed: %v", err)
			}

			if len(resources) != tt.expectedCount {
				t.Errorf("Expected %d resources, got %d", tt.expectedCount, len(resources))
			}

			resourceMap := make(map[string]bool)
			for _, r := range resources {
				resourceMap[r.Type+"."+r.Name] = true
			}

			for _, want := range tt.expectedResources {
				if !resourceMap[want] {
					t.Errorf("Missing expected resource: %s", want)
				}
			}
		})
	}
}

func TestToTreeNode(t *testing.T) {
	tests := []struct {
		name          string
		node          *ModuleNode
		expectedName  string
		checkChildren func(*testing.T, []*tree.Node)
	}{
		{
			name: "root node with one child",
			node: &ModuleNode{
				Key: "root",
				Children: []*ModuleNode{
					{
						Key: "child",
						Module: vendors.Module{
							Source:  "git::https://example.com/mod.git",
							Version: "v1.0.0",
						},
						Resources: []ResourceInfo{{Type: "r", Name: "n"}},
					},
				},
			},
			expectedName: "root",
			checkChildren: func(t *testing.T, children []*tree.Node) {
				if len(children) != 1 {
					t.Errorf("Expected 1 child, got %d", len(children))
					return
				}
				child := children[0]
				if !strings.Contains(child.Name, "child") {
					t.Errorf("Expected child name to contain 'child', got %s", child.Name)
				}
				if len(child.Children) != 1 {
					t.Errorf("Expected child node to have 1 child (resource count), got %d", len(child.Children))
				} else {
					if !strings.Contains(child.Children[0].Name, "[1 resources]") {
						t.Errorf("Expected resource count label, got %s", child.Children[0].Name)
					}
				}
			},
		},
		{
			name: "child node display local source",
			node: &ModuleNode{
				Key: "root",
				Children: []*ModuleNode{
					{
						Key: "local_mod",
						Module: vendors.Module{
							Source: "./local/path",
						},
						Resources: []ResourceInfo{},
					},
				},
			},
			expectedName: "root",
			checkChildren: func(t *testing.T, children []*tree.Node) {
				child := children[0]
				if !strings.Contains(child.Name, "local: ./local/path") {
					t.Errorf("Expected child to show local source, got %s", child.Name)
				}
			},
		},
		{
			name: "children sorting",
			node: &ModuleNode{
				Key: "root",
				Children: []*ModuleNode{
					{Key: "b_child", Module: vendors.Module{Source: "."}},
					{Key: "a_child", Module: vendors.Module{Source: "."}},
				},
			},
			expectedName: "root",
			checkChildren: func(t *testing.T, children []*tree.Node) {
				if len(children) != 2 {
					t.Fatalf("Expected 2 children, got %d", len(children))
				}
				// The implementation sorts children by key
				if !strings.Contains(children[0].Name, "a_child") {
					t.Errorf("Expected a_child to be first, got %s", children[0].Name)
				}
				if !strings.Contains(children[1].Name, "b_child") {
					t.Errorf("Expected b_child to be second, got %s", children[1].Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			treeNode := ToTreeNode(tt.node)
			if treeNode.Name != tt.expectedName {
				t.Errorf("Expected name %s, got %s", tt.expectedName, treeNode.Name)
			}
			if tt.checkChildren != nil {
				tt.checkChildren(t, treeNode.Children)
			}
		})
	}
}

func TestWriteModuleBlock(t *testing.T) {
	tests := []struct {
		name         string
		node         *ModuleNode
		targets      []string
		wantContains []string
		notContains  []string
	}{
		{
			name: "target specific module",
			node: &ModuleNode{
				Key: "root",
				Children: []*ModuleNode{
					{
						Key:       "child",
						Resources: []ResourceInfo{{Type: "aws_test", Name: "test"}},
					},
				},
			},
			targets:      []string{"child"},
			wantContains: []string{"module \"child\"", "resource \"aws_test\" \"test\""},
		},
		{
			name: "target parent module includes children recursively",
			node: &ModuleNode{
				Key: "root",
				Children: []*ModuleNode{
					{
						Key: "parent",
						Children: []*ModuleNode{
							{
								Key:       "parent.child",
								Resources: []ResourceInfo{{Type: "r", Name: "n"}},
							},
						},
					},
				},
			},
			targets:      []string{"parent"},
			wantContains: []string{"module \"parent\"", "module \"child\"", "resource \"r\" \"n\""},
		},
		{
			name: "not target module",
			node: &ModuleNode{
				Key: "root",
				Children: []*ModuleNode{
					{
						Key:       "other",
						Resources: []ResourceInfo{{Type: "r", Name: "n"}},
					},
				},
			},
			targets:     []string{"target"},
			notContains: []string{"module \"other\""},
		},
		{
			name: "empty targets implies all (implementation detail check)",
			// Based on code: if len(targets) == 0 { isTarget = true }
			node: &ModuleNode{
				Key: "root",
				Children: []*ModuleNode{
					{
						Key:       "any",
						Resources: []ResourceInfo{{Type: "r", Name: "n"}},
					},
				},
			},
			targets:      []string{},
			wantContains: []string{"module \"any\"", "resource \"r\" \"n\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := hclwrite.NewEmptyFile()
			rootBlock := f.Body().AppendNewBlock("dummy", nil)

			WriteModuleBlock(rootBlock, tt.node, tt.targets)

			output := string(f.Bytes())
			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("Output missing %q", want)
				}
			}
			for _, notWant := range tt.notContains {
				if strings.Contains(output, notWant) {
					t.Errorf("Output should not contain %q", notWant)
				}
			}
		})
	}
}
