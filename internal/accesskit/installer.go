package accesskit

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"godot-builder/internal/executor"
)

// Installer handles AccessKit SDK installation
type Installer struct {
	workDir string
	exec    *executor.Executor
}

// New creates a new AccessKit installer
func New(workDir string, exec *executor.Executor) *Installer {
	return &Installer{
		workDir: workDir,
		exec:    exec,
	}
}

// Install attempts to download and install AccessKit SDK
// Returns whether AccessKit is enabled and any error
func (i *Installer) Install() (bool, error) {
	// Try Python script installation first (modern approach)
	if enabled, err := i.installViaScript(); err == nil {
		return enabled, nil
	}

	// If script method fails, try downloading from GitHub (fallback)
	if enabled, err := i.installViaGithub(); err == nil {
		return enabled, nil
	}

	// If all fails, log warning but allow build to continue
	log.Printf("⚠ AccessKit SDK installation failed, building without AccessKit support")
	return false, nil
}

// installViaScript attempts to run the AccessKit installation Python script
func (i *Installer) installViaScript() (bool, error) {
	scriptPath := filepath.Join(i.workDir, "misc", "scripts", "install_accesskit.py")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return false, fmt.Errorf("script not found: %s", scriptPath)
	}

	// Try running the script
	cmd := exec.Command("python", scriptPath)
	cmd.Dir = i.workDir
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("script execution failed: %w", err)
	}

	log.Printf("✓ AccessKit SDK installed via Python script")
	return true, nil
}

// installViaGithub downloads pre-built AccessKit from GitHub
func (i *Installer) installViaGithub() (bool, error) {
	// AccessKit 0.21.2 is the standard version used in Godot 4.7+
	accesskitVersion := "0.21.2"
	outDir := filepath.Join(i.workDir, fmt.Sprintf("accesskit-c-%s", accesskitVersion))

	// Only try download if not already extracted
	if info, err := os.Stat(outDir); err == nil && info.IsDir() {
		log.Printf("✓ AccessKit SDK already available at %s", outDir)
		return true, nil
	}

	zipPath := filepath.Join(i.workDir, "accesskit.zip")
	defer os.Remove(zipPath)

	// Download URL template: this would need to be adjusted based on actual hosting
	// For now, we'll just fail gracefully and let the build continue
	url := fmt.Sprintf("https://github.com/AccessKit/accesskit/releases/download/v%s/accesskit-c-%s-x86_64-linux.zip",
		accesskitVersion, accesskitVersion)

	log.Printf("→ Downloading AccessKit from %s", url)
	if err := i.download(url, zipPath); err != nil {
		return false, err
	}

	log.Printf("✓ AccessKit SDK downloaded and installed")
	return true, nil
}

// download downloads a file from a URL with a timeout and simple retry logic
func (i *Installer) download(url, outPath string) error {
	// Use an HTTP client with timeout to avoid hanging indefinitely
	client := &http.Client{Timeout: 30 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		log.Printf("  → Download attempt %d: %s", attempt, url)
		resp, err := client.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("download failed: %w", err)
			log.Printf("    %v", lastErr)
			continue
		}

		// Ensure the body is closed for this attempt
		func() {
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				lastErr = fmt.Errorf("download failed with status %d", resp.StatusCode)
				log.Printf("    %v", lastErr)
				return
			}

			out, err := os.Create(outPath)
			if err != nil {
				lastErr = fmt.Errorf("failed to create output file: %w", err)
				log.Printf("    %v", lastErr)
				return
			}
			defer out.Close()

			if _, err := io.Copy(out, resp.Body); err != nil {
				lastErr = fmt.Errorf("failed to write file: %w", err)
				log.Printf("    %v", lastErr)
				return
			}

			// Attempt to extract
			if err := i.unzip(outPath, i.workDir); err != nil {
				lastErr = fmt.Errorf("failed to extract: %w", err)
				log.Printf("    %v", lastErr)
				return
			}

			// Success
			lastErr = nil
		}()

		if lastErr == nil {
			return nil
		}

		// Backoff between attempts
		if attempt < 3 {
			// Simple sleep without importing time at top-level; use exec sleep as a fallback
		exec.Command("sleep", "1").Run()
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("download failed: unknown error")
	}
	return lastErr
}

// unzip extracts a zip file to a destination directory
func (i *Installer) unzip(zipPath, destDir string) error {
	// Use unzip command if available
	if err := exec.Command("unzip", "-o", zipPath, "-d", destDir).Run(); err == nil {
		return nil
	}

	// Fallback: try with Python
	cmd := exec.Command("python", "-m", "zipfile", "-e", zipPath, destDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unzip failed: %w", err)
	}
	return nil
}

// GetPath returns the path to the AccessKit SDK
func (i *Installer) GetPath(version string) string {
	if version == "" {
		version = "0.21.2"
	}
	return filepath.Join(i.workDir, fmt.Sprintf("accesskit-c-%s", version))
}
