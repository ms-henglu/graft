package vendors

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/ms-henglu/graft/internal/log"
)

// GlobalCacheDir returns the global cache directory.
// It checks GRAFT_CACHE_DIR environment variable first, then defaults to ~/.graft/cache.
func GlobalCacheDir() (string, error) {
	if dir := os.Getenv("GRAFT_CACHE_DIR"); dir != "" {
		return dir, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		cwd, wdErr := os.Getwd()
		if wdErr != nil {
			return "", fmt.Errorf("failed to get user home directory (%v) and current working directory (%w)", err, wdErr)
		}
		log.Warn(fmt.Sprintf("Failed to get user home directory: %v. Falling back to current directory for cache. Set GRAFT_CACHE_DIR to specify a custom cache location.", err))
		return filepath.Join(cwd, ".graft", "cache"), nil
	}
	return filepath.Join(homeDir, ".graft", "cache"), nil
}

// EnsureGlobalCache checks if the module is in the global cache, and if not, downloads it.
// Returns the absolute path to the cached module and a boolean indicating if it was a cache hit.
func EnsureGlobalCache(source string, version string) (string, bool, error) {
	cacheDir, err := GlobalCacheDir()
	if err != nil {
		return "", false, err
	}

	// Create a stable hash for the cache key
	cacheKey := GetCacheKey(source, version)
	cachePath := filepath.Join(cacheDir, cacheKey)

	if _, err := os.Stat(cachePath); err == nil {
		log.Debug("Cache hit for %s@%s (%s)", source, version, cacheKey)
		return cachePath, true, nil
	}

	log.Debug("Downloading %s@%s to cache...", source, version)

	srcUrl := source

	// Check if source is a Registry Module

	switch {
	case isRegistrySource(source):
		log.Debug("Resolving registry module %s...", source)
		resolvedURL, err := resolveRegistrySource(source, version)
		if err != nil {
			return "", false, fmt.Errorf("failed to resolve registry module: %w", err)
		}
		log.Debug("Resolved %s to %s", source, resolvedURL)
		srcUrl = resolvedURL
	case version != "" && !strings.Contains(srcUrl, "?ref="):
		// If version is present and NOT a registry module (e.g. git url),
		// we assume it's a direct URL where we might need to append ref.
		// Simple heuristic:
		srcUrl = fmt.Sprintf("%s?ref=%s", srcUrl, version)
	}

	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return "", false, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Use go-getter to download
	client := &getter.Client{
		Src:  srcUrl,
		Dst:  cachePath,
		Mode: getter.ClientModeAny,
	}

	if err := client.Get(); err != nil {
		return "", false, fmt.Errorf("failed to download module: %w", err)
	}

	return cachePath, false, nil
}

// GetCacheKey returns a human-readable and unique cache key for a module.
func GetCacheKey(source, version string) string {
	// Human readable part
	// source: terraform-aws-modules/vpc/aws -> terraform-aws-modules-vpc-aws
	sanitizedSource := strings.ReplaceAll(source, "/", "-")
	sanitizedSource = strings.ReplaceAll(sanitizedSource, ":", "-")

	// Hash part for uniqueness
	fullHash := hashString(fmt.Sprintf("%s|%s", source, version))
	shortHash := fullHash[:8]

	// Combine: source-version-hash
	key := fmt.Sprintf("%s-%s-%s", sanitizedSource, version, shortHash)

	// Final sanitization to ensure valid filename
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '?' || r == '*' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '-'
		}
		return r
	}, key)
}

func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
