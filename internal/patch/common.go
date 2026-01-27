package patch

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"os"
	"path/filepath"
	"strings"
)

func listLocals(dir string) (map[string]*hclwrite.Attribute, error) {
	locals := make(map[string]*hclwrite.Attribute)
	matches, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return nil, err
	}

	for _, path := range matches {
		name := filepath.Base(path)
		if strings.HasPrefix(name, "_graft_") {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		f, diags := hclwrite.ParseConfig(content, name, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			continue
		}

		for _, block := range f.Body().Blocks() {
			if block.Type() == "locals" {
				for name, attr := range block.Body().Attributes() {
					locals[name] = attr
				}
			}
		}
	}
	return locals, nil
}

func listBlocks(dir string) (map[string]*hclwrite.Block, error) {
	blocks := make(map[string]*hclwrite.Block)
	matches, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return nil, err
	}

	for _, path := range matches {
		name := filepath.Base(path)
		if strings.HasPrefix(name, "_graft_") {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		f, diags := hclwrite.ParseConfig(content, name, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			// Skip malformed files or files hclwrite can't parse
			continue
		}

		for _, block := range f.Body().Blocks() {
			blocks[blockKey(block)] = block
		}
	}
	return blocks, nil
}

func blockKey(b *hclwrite.Block) string {
	return fmt.Sprintf("%s.%s", b.Type(), strings.Join(b.Labels(), "."))
}

func filterBlockWithType(body *hclwrite.Block, blockType string) *hclwrite.Block {
	for _, b := range body.Body().Blocks() {
		if b.Type() == blockType {
			return b
		}
	}
	return nil
}
