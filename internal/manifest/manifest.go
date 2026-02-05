package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	grafthcl "github.com/ms-henglu/graft/internal/hcl"
)

// Manifest represents the structure of manifest.hcl
type Manifest struct {
	RootOverrides  []*hclwrite.Block // Flattened content blocks (resource, data, locals, etc.)
	Modules        []Module
	PatchedModules map[string]Module
}

// Module represents a module block in manifest.hcl
// We'll use hclwrite blocks directly
type Module struct {
	Name           string
	Source         string
	Version        string
	OverrideBlocks []*hclwrite.Block // Flattened content blocks (resource, data, locals, etc.)
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

	m := &Manifest{}
	m.RootOverrides = flattenOverrideBlocks(grafthcl.BlocksByType(f.Body(), "override"))
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
			OverrideBlocks: flattenOverrideBlocks(grafthcl.BlocksByType(block.Body(), "override")),
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

// flattenOverrideBlocks extracts and flattens all content blocks from override blocks.
// This removes the "override" wrapper and returns the actual resource/data/locals blocks.
func flattenOverrideBlocks(overrideBlocks []*hclwrite.Block) []*hclwrite.Block {
	var result []*hclwrite.Block
	for _, override := range overrideBlocks {
		result = append(result, override.Body().Blocks()...)
	}
	return result
}

// DiscoverManifests finds all *.graft.hcl files in the given directory
// and returns them sorted alphabetically
func DiscoverManifests(dir string) ([]string, error) {
	pattern := filepath.Join(dir, "*.graft.hcl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob manifest files: %w", err)
	}

	if len(matches) == 0 {
		return nil, nil
	}

	// Sort alphabetically to ensure deterministic loading order
	sort.Strings(matches)
	return matches, nil
}

// ParseMultiple parses multiple manifest files and merges them using deep merge logic.
// Files are processed in the order provided (should be alphabetically sorted).
// For modules with the same name, their contents are merged:
// - override blocks are combined
// - nested modules are merged recursively
// - attributes use "last write wins" semantics
func ParseMultiple(paths []string) (*Manifest, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no manifest files provided")
	}

	merged := &Manifest{}
	// Merge subsequent files
	for _, path := range paths {
		other, err := Parse(path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}

		merged = mergeManifests(merged, other)
	}

	// Rebuild PatchedModules map after merge
	merged.PatchedModules = make(map[string]Module)
	collectPatchedModules(merged.Modules, "", merged.PatchedModules)

	return merged, nil
}

// mergeManifests merges two manifests using deep merge logic
func mergeManifests(base, other *Manifest) *Manifest {
	result := &Manifest{
		RootOverrides:  mergeOverrideBlocks(base.RootOverrides, other.RootOverrides),
		Modules:        mergeModuleLists(base.Modules, other.Modules),
		PatchedModules: make(map[string]Module),
	}
	return result
}
