package manifest

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// Manifest represents the structure of manifest.hcl
type Manifest struct {
	File           *hclwrite.File
	RootOverrides  []*hclwrite.Block
	Modules        []Module
	PatchedModules map[string]Module
}

// Module represents a module block in manifest.hcl
// We'll use hclwrite blocks directly
type Module struct {
	Name           string
	Source         string
	Version        string
	OverrideBlocks []*hclwrite.Block
	Modules        []Module
}

// Parse parses the manifest.hcl file using hclwrite
func Parse(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	f, diags := hclwrite.ParseConfig(data, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse manifest: %s", diags.Error())
	}

	m := &Manifest{File: f}
	m.RootOverrides = filterBlocks(f.Body(), "override")
	m.Modules = parseModules(f.Body())

	m.PatchedModules = make(map[string]Module)
	collectPatchedModules(m.Modules, "", m.PatchedModules)

	return m, nil
}

func parseModules(body *hclwrite.Body) []Module {
	var modules []Module
	for _, block := range body.Blocks() {
		if block.Type() != "module" {
			continue
		}

		name := ""
		if len(block.Labels()) > 0 {
			name = block.Labels()[0]
		}

		// Extract source and version
		source := ""
		version := ""
		if attr := block.Body().GetAttribute("source"); attr != nil {
			source = string(attr.Expr().BuildTokens(nil).Bytes())
			source = strings.Trim(source, "\"")
		}
		if attr := block.Body().GetAttribute("version"); attr != nil {
			version = string(attr.Expr().BuildTokens(nil).Bytes())
			version = strings.Trim(version, "\"")
		}

		mod := Module{
			Name:           name,
			Source:         source,
			Version:        version,
			OverrideBlocks: filterBlocks(block.Body(), "override"),
			Modules:        parseModules(block.Body()),
		}
		modules = append(modules, mod)

	}
	return modules
}

func collectPatchedModules(modules []Module, parentKey string, patched map[string]Module) {
	for _, mod := range modules {
		currentKey := mod.Name
		if parentKey != "" {
			currentKey = parentKey + "." + mod.Name
		}

		// Check if this module has overrides
		if len(mod.OverrideBlocks) > 0 {
			patched[currentKey] = mod
		}

		// Recurse
		collectPatchedModules(mod.Modules, currentKey, patched)
	}
}

func filterBlocks(body *hclwrite.Body, blockType string) []*hclwrite.Block {
	var blocks []*hclwrite.Block
	for _, block := range body.Blocks() {
		if block.Type() == blockType {
			blocks = append(blocks, block)
		}
	}
	return blocks
}
