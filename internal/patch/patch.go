package patch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/ms-henglu/graft/internal/log"
	"github.com/ms-henglu/graft/internal/manifest"
)

// ApplyPatches applies overrides to vendored modules
func ApplyPatches(vendorMap map[string]string, m *manifest.Manifest) error {
	// Apply root overrides
	if len(m.RootOverrides) > 0 {
		// Use current directory for root overrides
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		err = applyOverrides("root", cwd, m.RootOverrides)
		if err != nil {
			return err
		}

		log.Debug("Root override applied to %s", cwd)
	}

	// Sort keys for deterministic output
	var modKeys []string
	for k := range m.PatchedModules {
		modKeys = append(modKeys, k)
	}
	sort.Strings(modKeys)

	// Apply patched module overrides
	for _, modKey := range modKeys {
		mod := m.PatchedModules[modKey]
		vendorPath, ok := vendorMap[modKey]
		if !ok {
			log.Warn(fmt.Sprintf("Module %s is not vendored, skipping override", modKey))
			continue
		}

		err := applyOverrides(modKey, vendorPath, mod.OverrideBlocks)
		if err != nil {
			return err
		}

		count := len(mod.OverrideBlocks)
		suffix := "s"
		if count == 1 {
			suffix = ""
		}
		log.Item(fmt.Sprintf("%s: %d override%s", modKey, count, suffix))
		log.Debug("Patched module %s in %s", modKey, vendorPath)
	}
	return nil
}

func applyOverrides(modKey string, modulePath string, overrideBlocks []*hclwrite.Block) error {
	// First, apply removals before reading existing blocks
	// This ensures that removed blocks won't be included in deep merge
	err := applyRemovals(modulePath, overrideBlocks)
	if err != nil {
		return fmt.Errorf("failed to calculate removals for %s: %w", modKey, err)
	}

	// Now read existing blocks after removals have been applied
	existingBlocks, err := listBlocks(modulePath)
	if err != nil {
		return fmt.Errorf("failed to scan module for existing blocks: %w", err)
	}

	existingLocals, err := listLocals(modulePath)
	if err != nil {
		return fmt.Errorf("failed to list locals: %w", err)
	}

	resolveGraftTokens(overrideBlocks, existingBlocks, existingLocals)

	overrideFile := generateOverrideFile(overrideBlocks, existingBlocks, existingLocals)
	if len(overrideFile.Body().Attributes()) > 0 || len(overrideFile.Body().Blocks()) > 0 {
		outputPath := filepath.Join(modulePath, "_graft_override.tf")
		if err = os.WriteFile(outputPath, overrideFile.Bytes(), 0644); err != nil {
			return err
		}
	}

	addFile := generateAddFile(overrideBlocks, existingBlocks, existingLocals)
	if len(addFile.Body().Attributes()) > 0 || len(addFile.Body().Blocks()) > 0 {
		outputPath := filepath.Join(modulePath, "_graft_add.tf")
		if err = os.WriteFile(outputPath, addFile.Bytes(), 0644); err != nil {
			return err
		}
	}
	return nil
}

func generateAddFile(overrideBlocks []*hclwrite.Block, existingBlocks map[string]*hclwrite.Block, existingLocals map[string]*hclwrite.Attribute) *hclwrite.File {
	fAdd := hclwrite.NewEmptyFile()
	bAdd := fAdd.Body()

	for _, block := range overrideBlocks {
		if block.Type() == "locals" {
			newLocals := hclwrite.NewBlock("locals", nil)
			hasAttr := false
			for name, attr := range block.Body().Attributes() {
				if existingLocals[name] == nil {
					newLocals.Body().SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
					hasAttr = true
				}
			}
			if hasAttr {
				bAdd.AppendBlock(newLocals)
			}
			continue
		}
		key := blockKey(block)
		if existingBlocks[key] == nil {
			if len(block.Body().Attributes()) == 0 && len(block.Body().Blocks()) == 0 {
				continue
			}
			bAdd.AppendBlock(block)
		}
	}
	return fAdd
}

func generateOverrideFile(overrideBlocks []*hclwrite.Block, existingBlocks map[string]*hclwrite.Block, existingLocals map[string]*hclwrite.Attribute) *hclwrite.File {
	fOverride := hclwrite.NewEmptyFile()
	bOverride := fOverride.Body()

	for _, block := range overrideBlocks {
		key := blockKey(block)

		if block.Type() == "locals" {
			newLocals := hclwrite.NewBlock("locals", nil)
			hasAttr := false
			for name, attr := range block.Body().Attributes() {
				if _, ok := existingLocals[name]; ok {
					newLocals.Body().SetAttributeRaw(name, attr.Expr().BuildTokens(nil))
					hasAttr = true
				}
			}
			if hasAttr {
				bOverride.AppendBlock(newLocals)
			}
			continue
		}

		existingBlock := existingBlocks[key]
		if existingBlock != nil {
			// If block is empty, which can happen if we stripped things, check if it's meaningful
			if len(block.Body().Attributes()) == 0 && len(block.Body().Blocks()) == 0 {
				continue
			}

			// Check if override has nested blocks that need deep merging
			if len(block.Body().Blocks()) > 0 {
				// Deep merge nested blocks from source
				mergedBlock := deepMergeNestedBlock(existingBlock, block, true)
				bOverride.AppendBlock(mergedBlock)
			} else {
				// No nested blocks, use as-is (Terraform handles shallow merge for attributes)
				bOverride.AppendBlock(block)
			}
		}
	}
	return fOverride
}
