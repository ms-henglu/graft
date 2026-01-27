package vendors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ms-henglu/graft/internal/log"
	"github.com/ms-henglu/graft/internal/manifest"
	"github.com/otiai10/copy"
)

// Module represents a module in modules.json
type Module struct {
	Key     string `json:"Key"`
	Source  string `json:"Source"`
	Version string `json:"Version,omitempty"`
	Dir     string `json:"Dir"`
}

// ModulesJSON represents the structure of modules.json
type ModulesJSON struct {
	Modules []Module `json:"Modules"`
}

// LoadModulesJSON reads and parses a modules.json file
func LoadModulesJSON(projectDir string) (ModulesJSON, error) {
	modulesJSONPath := filepath.Join(projectDir, ".terraform", "modules", "modules.json")
	data, err := os.ReadFile(modulesJSONPath)
	if err != nil {
		return ModulesJSON{}, fmt.Errorf("failed to read modules.json: %w; please run 'terraform init' to initialize the project", err)
	}

	var modules ModulesJSON
	if err := json.Unmarshal(data, &modules); err != nil {
		return ModulesJSON{}, fmt.Errorf("failed to parse modules.json: %w; please run 'terraform init' to initialize the project", err)
	}

	return modules, nil
}

// VendorModules reads modules.json and downloads/hydrates modules to .graft/build
// It uses global cache to avoid redundant downloads.
func VendorModules(projectDir string, m *manifest.Manifest) (map[string]string, error) {
	if len(m.PatchedModules) == 0 {
		return map[string]string{}, nil
	}

	// Load modules.json to discover module sources and versions
	modulesJSON, err := LoadModulesJSON(projectDir)
	if err != nil {
		log.Warn(fmt.Sprintf("Failed to load modules.json: %v", err))
	}

	// Sort keys for deterministic output
	var modKeys []string
	for k := range m.PatchedModules {
		modKeys = append(modKeys, k)
	}
	sort.Strings(modKeys)

	cacheStatus := make(map[string]bool)
	terraformModuleMap := make(map[string]Module)
	// Download remote modules to global cache
	for _, modKey := range modKeys {
		mod := m.PatchedModules[modKey]
		modSource := mod.Source
		modVersion := mod.Version

		// Check if module is in modules.json
		for _, mod := range modulesJSON.Modules {
			if mod.Key == modKey {
				if modSource == "" {
					modSource = mod.Source
				}
				if modVersion == "" {
					modVersion = mod.Version
				}
				break
			}
		}

		if modSource == "" {
			return nil, fmt.Errorf("source not found for module %s. Is it in modules.json?", modKey)
		}

		terraformModuleMap[modKey] = Module{
			Key:     modKey,
			Source:  modSource,
			Version: modVersion,
		}

		if !isLocalModule(modSource) {
			if _, hit, err := EnsureGlobalCache(modSource, modVersion); err != nil {
				return nil, fmt.Errorf("failed to ensure cache for %s: %w", modKey, err)
			} else {
				cacheStatus[modKey] = hit
			}
		}
	}

	moduleMap := make(map[string]string)
	// Track which modules are in manifest to vendor them
	for _, modKey := range modKeys {
		log.Debug("Processing module %s", modKey)

		// 1. Resolve Source Path using Anchor Resolution Strategy
		sourcePath, err := ResolveTrueSourcePath(modKey, terraformModuleMap)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve source path for %s: %w", modKey, err)
		}

		// Print Summary
		mod := terraformModuleMap[modKey]
		extra := ""
		if !isLocalModule(mod.Source) {
			if cacheStatus[modKey] {
				extra = " [Cache Hit]"
			} else {
				extra = " [Downloaded]"
			}
		} else {
			extra = " (Local)"
		}

		versionStr := ""
		if mod.Version != "" {
			versionStr = fmt.Sprintf(" (v%s)", mod.Version)
		}

		log.Item(fmt.Sprintf("%s%s%s", modKey, versionStr, extra))

		log.Debug("Resolved source for %s: %s", modKey, sourcePath)

		// 2. Vendor Module to Workspace
		dstDir, err := VendorModule(modKey, sourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to vendor module %s: %w", modKey, err)
		}

		moduleMap[modKey] = dstDir
	}

	return moduleMap, nil
}

// VendorModule copies the module from cache to the build directory.
// Returns the absolute path to the build directory.
func VendorModule(moduleKey string, cachePath string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	buildPath := filepath.Join(cwd, ".graft", "build", moduleKey)

	// Clean target
	if err := os.RemoveAll(buildPath); err != nil {
		return "", fmt.Errorf("failed to clean build directory %s: %w", buildPath, err)
	}

	// Create target directory (parent)
	if err := os.MkdirAll(filepath.Dir(buildPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create build directory parent: %w", err)
	}

	log.Debug("Vendoring %s from %s...", moduleKey, cachePath)
	if err := copy.Copy(cachePath, buildPath); err != nil {
		return "", fmt.Errorf("failed to copy module from cache: %w", err)
	}

	return buildPath, nil
}

// RedirectModules updates .terraform/modules/modules.json to point to hydrated modules
func RedirectModules(projectDir string) error {
	modulesJSON, err := LoadModulesJSON(projectDir)
	if err != nil {
		return fmt.Errorf("failed to load modules.json: %w", err)
	}

	buildDir := filepath.Join(projectDir, ".graft", "build")

	// Create a lookup map for existing build directories
	buildMap := make(map[string]bool)
	entries, err := os.ReadDir(buildDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				buildMap[e.Name()] = true
			}
		}
	}

	// Iterate through tfModules and redirect if we have a hydrated build
	for i, m := range modulesJSON.Modules {
		if buildMap[m.Key] {
			// Redirect to .graft/build/{key}
			modulesJSON.Modules[i].Dir = filepath.Join(".graft", "build", m.Key)
		}
	}

	updatedData, err := json.MarshalIndent(modulesJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated modules.json: %w", err)
	}

	terraformModulesPath := filepath.Join(projectDir, ".terraform", "modules", "modules.json")
	if err := os.WriteFile(terraformModulesPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write updated modules.json: %w", err)
	}

	return nil
}
