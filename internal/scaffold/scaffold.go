package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/ms-henglu/graft/internal/log"
	"github.com/ms-henglu/graft/internal/tree"
	"github.com/ms-henglu/graft/internal/vendors"
)

type ModuleNode struct {
	Key              string
	Module           vendors.Module
	Resources        []ResourceInfo
	Children         []*ModuleNode
	IsScaffoldTarget bool
}

type ResourceInfo struct {
	Type     string
	Name     string
	FilePath string
	Line     int
}

func ScanResources(dir string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	files, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		// Ignore graft files
		if strings.HasPrefix(filepath.Base(file), "_graft") {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		f, diags := hclsyntax.ParseConfig(content, file, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			log.Warn(fmt.Sprintf("Failed to parse %s: %s", file, diags.Error()))
			continue // Skip problematic files
		}

		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}

		for _, block := range body.Blocks {
			if block.Type == "resource" && len(block.Labels) == 2 {
				resources = append(resources, ResourceInfo{
					Type:     block.Labels[0],
					Name:     block.Labels[1],
					FilePath: file,
					Line:     block.TypeRange.Start.Line,
				})
			}
		}
	}
	return resources, nil
}

func ToTreeNode(node *ModuleNode) *tree.Node {
	name := node.Key
	if node.Key != "root" {
		sourceInfo := ""
		if strings.HasPrefix(node.Module.Source, ".") || strings.HasPrefix(node.Module.Source, "/") {
			sourceInfo = fmt.Sprintf("(local: %s)", node.Module.Source)
		} else {
			sourceInfo = fmt.Sprintf("(registry: %s, %s)", node.Module.Source, node.Module.Version)
		}
		name = fmt.Sprintf("%s %s", node.Key, sourceInfo)
	}

	resLabel := fmt.Sprintf("[%d resources]", len(node.Resources))

	// Sort children by key
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Key < node.Children[j].Key
	})

	tNode := &tree.Node{
		Name: name,
	}

	if node.Key != "root" {
		tNode.Children = append(tNode.Children, &tree.Node{Name: resLabel})
	}

	for _, child := range node.Children {
		tNode.Children = append(tNode.Children, ToTreeNode(child))
	}

	return tNode
}

func WriteModuleBlock(body *hclwrite.Block, node *ModuleNode, targets []string) {
	isTarget := false
	isTargetParent := node.Key == "root"
	if len(targets) == 0 {
		isTarget = true
	} else {
		for _, t := range targets {
			if t == node.Key || strings.HasPrefix(node.Key, t+".") {
				isTarget = true
				break
			}
		}
		if !isTarget {
			for _, t := range targets {
				if strings.HasPrefix(t, node.Key+".") {
					isTargetParent = true
					break
				}
			}
		}
	}

	if !isTarget && !isTargetParent {
		return
	}

	parts := strings.Split(node.Key, ".")
	moduleName := parts[len(parts)-1]

	modBlock := body.Body().AppendNewBlock("module", []string{moduleName})
	modBody := modBlock.Body()

	if isTarget {
		overrideBlock := modBody.AppendNewBlock("override", nil)
		overrideBody := overrideBlock.Body()
		for _, res := range node.Resources {
			comment := fmt.Sprintf("# resource \"%s\" \"%s\" {}", res.Type, res.Name)
			overrideBody.AppendUnstructuredTokens(hclwrite.Tokens{
				{Type: hclsyntax.TokenComment, Bytes: []byte(comment)},
				{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
				{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
			})
		}
	}

	for _, child := range node.Children {
		WriteModuleBlock(modBlock, child, targets)
	}

	body.Body().AppendNewline()
}
