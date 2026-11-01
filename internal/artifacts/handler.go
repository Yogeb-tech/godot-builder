package artifacts

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Handler manages build artifacts
type Handler struct {
	outputDir string
}

// New creates a new artifact handler
func New(outputDir string) *Handler {
	return &Handler{
		outputDir: outputDir,
	}
}

// PrepareArtifacts prepares artifacts for a specific platform/target
func (h *Handler) PrepareArtifacts(platform, target, arch string, isMono bool) error {
	log.Printf("→ Preparing artifacts for %s %s (%s)", platform, target, arch)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(h.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Platform-specific artifact preparation
	switch platform {
	case "linux":
		return h.prepareLinuxArtifacts(target, arch)
	case "windows":
		return h.prepareWindowsArtifacts(target, arch)
	case "web":
		return h.prepareWebArtifacts(target)
	case "android":
		return h.prepareAndroidArtifacts(target)
	}

	return nil
}

// prepareLinuxArtifacts prepares Linux-specific artifacts
func (h *Handler) prepareLinuxArtifacts(target, arch string) error {
	binDir := filepath.Join(".", "bin")

	// Find and strip binaries
	pattern := filepath.Join(binDir, fmt.Sprintf("godot.linuxbsd.%s.*", target))
	log.Printf("  Stripping binaries matching: %s", pattern)

	matches, _ := filepath.Glob(pattern)
	for _, match := range matches {
		if strings.Contains(match, ".so") {
			// Skip shared libraries
			continue
		}
		log.Printf("  Stripping: %s", match)
		// In real implementation, would call: strip <binary>
		// For now, just log the action
	}

	return nil
}

// prepareWindowsArtifacts prepares Windows-specific artifacts
func (h *Handler) prepareWindowsArtifacts(target, arch string) error {
	binDir := filepath.Join(".", "bin")

	log.Printf("  Cleaning up Windows build artifacts")

	// Remove unnecessary files
	unnecessaryExts := []string{".exp", ".lib", ".pdb"}
	for _, ext := range unnecessaryExts {
		pattern := filepath.Join(binDir, "*"+ext)
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			log.Printf("  Removing: %s", match)
			os.Remove(match)
		}
	}

	return nil
}

// prepareWebArtifacts prepares Web-specific artifacts
func (h *Handler) prepareWebArtifacts(target string) error {
	binDir := filepath.Join(".", "bin")

	log.Printf("  Preparing web artifacts")

	// Verify required files exist
	requiredFiles := []string{
		filepath.Join(binDir, "godot.web."+target+".js"),
		filepath.Join(binDir, "godot.web."+target+".wasm"),
	}

	for _, file := range requiredFiles {
		if _, err := os.Stat(file); err != nil {
			log.Printf("  ⚠ Expected file not found: %s", file)
		} else {
			log.Printf("  ✓ Found: %s", file)
		}
	}

	return nil
}

// prepareAndroidArtifacts prepares Android-specific artifacts
func (h *Handler) prepareAndroidArtifacts(target string) error {
	binDir := filepath.Join(".", "bin")

	log.Printf("  Preparing Android artifacts")

	// Android builds produce APKs and template ZIPs
	patterns := []string{
		filepath.Join(binDir, "*.apk"),
		filepath.Join(binDir, "*template*.zip"),
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			log.Printf("  ✓ Found artifact: %s", match)
		}
	}

	return nil
}

// CopyToOutput copies build artifacts to the output directory
func (h *Handler) CopyToOutput(sourcePattern string) (int, error) {
	matches, err := filepath.Glob(sourcePattern)
	if err != nil {
		return 0, fmt.Errorf("glob pattern error: %w", err)
	}

	if len(matches) == 0 {
		return 0, fmt.Errorf("no files matching pattern: %s", sourcePattern)
	}

	for _, src := range matches {
		dest := filepath.Join(h.outputDir, filepath.Base(src))
		if err := copyFile(src, dest); err != nil {
			return 0, fmt.Errorf("failed to copy %s: %w", src, err)
		}
		log.Printf("  ✓ Copied: %s", filepath.Base(src))
	}

	return len(matches), nil
}

// ListArtifacts lists all artifacts in the output directory
func (h *Handler) ListArtifacts() ([]string, error) {
	entries, err := os.ReadDir(h.outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read output directory: %w", err)
	}

	var artifacts []string
	for _, entry := range entries {
		if !entry.IsDir() {
			artifacts = append(artifacts, filepath.Join(h.outputDir, entry.Name()))
		}
	}

	return artifacts, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}

// CleanArtifacts removes all artifacts from the output directory
func (h *Handler) CleanArtifacts() error {
	return os.RemoveAll(h.outputDir)
}
