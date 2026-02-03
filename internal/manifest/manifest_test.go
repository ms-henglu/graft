package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverManifests(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "graft-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files
	files := []string{
		"10-network.graft.hcl",
		"00-base.graft.hcl",
		"99-overrides.graft.hcl",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte(""), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", f, err)
		}
	}

	// Create a non-graft file that should be ignored
	if err := os.WriteFile(filepath.Join(tmpDir, "main.tf"), []byte(""), 0644); err != nil {
		t.Fatalf("failed to create main.tf: %v", err)
	}

	// Test discovery
	manifests, err := DiscoverManifests(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverManifests failed: %v", err)
	}

	if len(manifests) != 3 {
		t.Errorf("expected 3 manifests, got %d", len(manifests))
	}

	// Verify alphabetical order
	expected := []string{
		filepath.Join(tmpDir, "00-base.graft.hcl"),
		filepath.Join(tmpDir, "10-network.graft.hcl"),
		filepath.Join(tmpDir, "99-overrides.graft.hcl"),
	}
	for i, exp := range expected {
		if manifests[i] != exp {
			t.Errorf("expected manifests[%d] = %s, got %s", i, exp, manifests[i])
		}
	}
}

func TestDiscoverManifests_NoFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graft-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifests, err := DiscoverManifests(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverManifests failed: %v", err)
	}

	if manifests != nil {
		t.Errorf("expected nil, got %v", manifests)
	}
}

func TestParseMultiple_SingleFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graft-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := `
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  override {
    resource "aws_vpc" "this" {
      tags = {
        Name = "test"
      }
    }
  }
}
`
	filePath := filepath.Join(tmpDir, "manifest.graft.hcl")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	m, err := ParseMultiple([]string{filePath})
	if err != nil {
		t.Fatalf("ParseMultiple failed: %v", err)
	}

	if len(m.Modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(m.Modules))
	}

	if m.Modules[0].Name != "vpc" {
		t.Errorf("expected module name 'vpc', got '%s'", m.Modules[0].Name)
	}
}

func TestParseMultiple_MergeModules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graft-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// File A: scaffold.graft.hcl
	contentA := `
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  override {
    resource "aws_vpc" "this" {
      tags = {
        Name = "vpc"
      }
    }
  }
}
`
	fileA := filepath.Join(tmpDir, "a_scaffold.graft.hcl")
	if err := os.WriteFile(fileA, []byte(contentA), 0644); err != nil {
		t.Fatalf("failed to write file A: %v", err)
	}

	// File B: fix.graft.hcl - adds a different resource to the same module
	contentB := `
module "vpc" {
  override {
    resource "aws_subnet" "private" {
      tags = {
        Fix = "True"
      }
    }
  }
}
`
	fileB := filepath.Join(tmpDir, "b_fix.graft.hcl")
	if err := os.WriteFile(fileB, []byte(contentB), 0644); err != nil {
		t.Fatalf("failed to write file B: %v", err)
	}

	m, err := ParseMultiple([]string{fileA, fileB})
	if err != nil {
		t.Fatalf("ParseMultiple failed: %v", err)
	}

	// Should have 1 merged module
	if len(m.Modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(m.Modules))
	}

	// Module should have source and version from file A (check contains since parsing includes whitespace)
	if !strings.Contains(m.Modules[0].Source, "terraform-aws-modules/vpc/aws") {
		t.Errorf("expected source to contain 'terraform-aws-modules/vpc/aws', got '%s'", m.Modules[0].Source)
	}

	// Module should have 2 flattened content blocks (aws_vpc and aws_subnet resources)
	if len(m.Modules[0].OverrideBlocks) != 2 {
		t.Errorf("expected 2 flattened content blocks, got %d", len(m.Modules[0].OverrideBlocks))
	}
}

func TestParseMultiple_LastWriteWins(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graft-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// File A: base version
	contentA := `
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "4.0.0"
}
`
	fileA := filepath.Join(tmpDir, "a_base.graft.hcl")
	if err := os.WriteFile(fileA, []byte(contentA), 0644); err != nil {
		t.Fatalf("failed to write file A: %v", err)
	}

	// File B: override version (should win)
	contentB := `
module "vpc" {
  version = "5.0.0"
}
`
	fileB := filepath.Join(tmpDir, "b_override.graft.hcl")
	if err := os.WriteFile(fileB, []byte(contentB), 0644); err != nil {
		t.Fatalf("failed to write file B: %v", err)
	}

	m, err := ParseMultiple([]string{fileA, fileB})
	if err != nil {
		t.Fatalf("ParseMultiple failed: %v", err)
	}

	// Version should be from file B (last write wins)
	if !strings.Contains(m.Modules[0].Version, "5.0.0") {
		t.Errorf("expected version to contain '5.0.0', got '%s'", m.Modules[0].Version)
	}

	// Source should be preserved from file A
	if !strings.Contains(m.Modules[0].Source, "terraform-aws-modules/vpc/aws") {
		t.Errorf("expected source to contain 'terraform-aws-modules/vpc/aws', got '%s'", m.Modules[0].Source)
	}
}

func TestParseMultiple_MultipleModules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graft-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// File A: network module
	contentA := `
module "network" {
  source = "terraform-aws-modules/vpc/aws"
  
  override {
    resource "aws_vpc" "this" {}
  }
}
`
	fileA := filepath.Join(tmpDir, "a_network.graft.hcl")
	if err := os.WriteFile(fileA, []byte(contentA), 0644); err != nil {
		t.Fatalf("failed to write file A: %v", err)
	}

	// File B: security module
	contentB := `
module "security" {
  source = "terraform-aws-modules/security-group/aws"
  
  override {
    resource "aws_security_group" "this" {}
  }
}
`
	fileB := filepath.Join(tmpDir, "b_security.graft.hcl")
	if err := os.WriteFile(fileB, []byte(contentB), 0644); err != nil {
		t.Fatalf("failed to write file B: %v", err)
	}

	m, err := ParseMultiple([]string{fileA, fileB})
	if err != nil {
		t.Fatalf("ParseMultiple failed: %v", err)
	}

	// Should have 2 separate modules
	if len(m.Modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(m.Modules))
	}

	// Verify module names
	moduleNames := make(map[string]bool)
	for _, mod := range m.Modules {
		moduleNames[mod.Name] = true
	}

	if !moduleNames["network"] {
		t.Error("expected 'network' module to be present")
	}
	if !moduleNames["security"] {
		t.Error("expected 'security' module to be present")
	}
}

func TestParseMultiple_NestedModules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graft-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// File A: parent module with nested module
	contentA := `
module "parent" {
  source = "example/parent/aws"
  
  module "child1" {
    override {
      resource "aws_instance" "this" {
        instance_type = "t2.micro"
      }
    }
  }
}
`
	fileA := filepath.Join(tmpDir, "a_parent.graft.hcl")
	if err := os.WriteFile(fileA, []byte(contentA), 0644); err != nil {
		t.Fatalf("failed to write file A: %v", err)
	}

	// File B: adds another nested module to parent
	contentB := `
module "parent" {
  module "child2" {
    override {
      resource "aws_instance" "this" {
        instance_type = "t2.large"
      }
    }
  }
}
`
	fileB := filepath.Join(tmpDir, "b_child.graft.hcl")
	if err := os.WriteFile(fileB, []byte(contentB), 0644); err != nil {
		t.Fatalf("failed to write file B: %v", err)
	}

	m, err := ParseMultiple([]string{fileA, fileB})
	if err != nil {
		t.Fatalf("ParseMultiple failed: %v", err)
	}

	// Should have 1 parent module
	if len(m.Modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(m.Modules))
	}

	// Parent should have 2 nested modules
	if len(m.Modules[0].Modules) != 2 {
		t.Errorf("expected 2 nested modules, got %d", len(m.Modules[0].Modules))
	}

	// Verify nested module names
	nestedNames := make(map[string]bool)
	for _, mod := range m.Modules[0].Modules {
		nestedNames[mod.Name] = true
	}

	if !nestedNames["child1"] {
		t.Error("expected 'child1' nested module to be present")
	}
	if !nestedNames["child2"] {
		t.Error("expected 'child2' nested module to be present")
	}
}
