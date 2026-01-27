package vendors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveTrueSourcePath resolves the pristine source path for a module,
// ignoring the dirty Dir field in modules.json.
func ResolveTrueSourcePath(targetKey string, moduleMap map[string]Module) (string, error) {
	// Base Case 2: Root Module
	if targetKey == "" {
		return os.Getwd()
	}

	targetModule, ok := moduleMap[targetKey]
	if !ok {
		return "", fmt.Errorf("module key not found: %s", targetKey)
	}

	// Base Case 1: Remote Module (Anchor)
	if !isLocalModule(targetModule.Source) {
		// Calculate location in Global Cache
		cacheDir, err := GlobalCacheDir()
		if err != nil {
			return "", fmt.Errorf("failed to get global cache dir: %w", err)
		}

		// Reconstruct cache key used in EnsureGlobalCache
		// cacheKey = hash(source|version)
		cacheKey := GetCacheKey(targetModule.Source, targetModule.Version)
		return filepath.Join(cacheDir, cacheKey), nil
	}

	// Recursive Case: Local Module (Parasite)
	parentKey := getParentKey(targetKey)
	parentPath, err := ResolveTrueSourcePath(parentKey, moduleMap)
	if err != nil {
		return "", fmt.Errorf("failed to resolve parent %s for %s: %w", parentKey, targetKey, err)
	}

	// Combine parent path with relative source
	return filepath.Join(parentPath, targetModule.Source), nil
}

// isLocalModule checks if the source is a local relative path.
func isLocalModule(source string) bool {
	return strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../")
}

// getParentKey derives the parent key from the module key.
// e.g., "eks.node_group" -> "eks"
// e.g., "vpc" -> ""
func getParentKey(key string) string {
	parts := strings.Split(key, ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}
