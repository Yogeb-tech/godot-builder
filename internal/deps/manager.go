package deps

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"godot-builder/internal/executor"
)

// Manager handles dependency setup (Python, SCons, etc)
type Manager struct {
	exec *executor.Executor
}

// New creates a new dependency manager
func New(exec *executor.Executor) *Manager {
	return &Manager{
		exec: exec,
	}
}

// SetupPythonAndScons installs Python and SCons
func (m *Manager) SetupPythonAndScons(pythonVersion, sconsVersion string) error {
	if pythonVersion == "" {
		pythonVersion = "3.x"
	}
	if sconsVersion == "" {
		sconsVersion = "4.9.0"
	}

	log.Printf("→ Setting up Python %s and SCons %s", pythonVersion, sconsVersion)

	// Check if Python is available
	if err := executor.CheckCommand("python"); err != nil {
		if err = executor.CheckCommand("python3"); err != nil {
			return fmt.Errorf("python not found in PATH")
		}
	}

	// Get Python version
	output, err := m.exec.RunSilent("python", "-c", "import sys; print(sys.version)")
	if err != nil {
		output, _ = m.exec.RunSilent("python3", "-c", "import sys; print(sys.version)")
	}
	log.Printf("  Python version: %s", strings.TrimSpace(output))

	// Install SCons via pip
	log.Printf("→ Installing SCons %s via pip", sconsVersion)
	if err := m.exec.Run("pip", "install", fmt.Sprintf("scons==%s", sconsVersion)); err != nil {
		return fmt.Errorf("failed to install SCons: %w", err)
	}

	// Verify SCons installation
	if _, err := m.exec.RunSilent("scons", "--version"); err != nil {
		return fmt.Errorf("scons verification failed: %w", err)
	}
	log.Printf("SCons installed successfully")

	return nil
}

// CheckPython verifies Python is available
func (m *Manager) CheckPython() error {
	if err := executor.CheckCommand("python"); err == nil {
		return nil
	}
	if err := executor.CheckCommand("python3"); err == nil {
		return nil
	}
	return fmt.Errorf("python not found in PATH")
}

// CheckSCons verifies SCons is available
func (m *Manager) CheckSCons() error {
	return executor.CheckCommand("scons")
}

// InstallSystemDeps installs platform-specific system dependencies
func (m *Manager) InstallSystemDeps(platform string) error {
	log.Printf("→ Installing system dependencies for %s", platform)

	// Detect OS
	osType := getOSType()

	switch osType {
	case "linux":
		return m.installLinuxDeps(platform)
	case "darwin":
		return m.installMacOSDeps(platform)
	case "windows":
		// Windows dependencies typically require Visual Studio or manual setup
		log.Printf("Windows: ensure you have Visual Studio Build Tools or MSVC installed")
		return nil
	default:
		return fmt.Errorf("unsupported OS: %s", osType)
	}
}

// installLinuxDeps installs Linux-specific dependencies
func (m *Manager) installLinuxDeps(platform string) error {
	// Check if apt is available (Ubuntu/Debian)
	if _, err := exec.LookPath("apt-get"); err != nil {
		log.Printf("apt-get not found, skipping automated dependency installation")
		return nil
	}

	packages := []string{"build-essential", "pkg-config"}

	switch platform {
	case "linux":
		packages = append(packages, "libx11-dev", "libxcursor-dev", "libxrandr-dev", "libxinerama-dev", "libxi-dev")
	case "windows":
		// Godot exposes the Windows platform on POSIX hosts only when
		// a MinGW-w64 cross compiler is available.
		packages = append(packages,
			"gcc-mingw-w64-x86-64",
			"g++-mingw-w64-x86-64",
			"binutils-mingw-w64-x86-64",
			"mingw-w64-tools",
		)
	case "web":
		// Emscripten is handled separately
	case "android":
		packages = append(packages, "default-jdk", "android-sdk", "android-ndk")
	}

	for _, pkg := range packages {
		log.Printf("→ Installing package: %s", pkg)
		if err := m.exec.Run("apt-get", "install", "-y", pkg); err != nil {
			log.Printf("Failed to install %s (may already be installed): %v", pkg, err)
		}
	}

	return nil
}

// installMacOSDeps installs macOS-specific dependencies
func (m *Manager) installMacOSDeps(platform string) error {
	// Check if Homebrew is available
	if _, err := exec.LookPath("brew"); err != nil {
		log.Printf("Homebrew not found, skipping automated dependency installation")
		return nil
	}

	packages := []string{"pkg-config"}

	for _, pkg := range packages {
		log.Printf("→ Installing package: %s", pkg)
		if err := m.exec.Run("brew", "install", pkg); err != nil {
			log.Printf("Failed to install %s: %v", pkg, err)
		}
	}

	return nil
}

// SetupEmscripten sets up Emscripten for web builds
func (m *Manager) SetupEmscripten(version string) error {
	if version == "" {
		version = "4.0.11"
	}

	log.Printf("→ Setting up Emscripten %s", version)

	// Check if emcc is available
	if err := executor.CheckCommand("emcc"); err != nil {
		return fmt.Errorf("emscripten not found in PATH. Please install from https://emscripten.org/docs/getting_started/index.html")
	}

	// Verify version
	if output, err := m.exec.RunSilent("emcc", "--version"); err == nil {
		log.Printf("  Emscripten version: %s", strings.TrimSpace(output))
	}

	return nil
}

// SetupJava sets up Java for Android builds
func (m *Manager) SetupJava() error {
	log.Printf("→ Setting up Java for Android builds")

	if err := executor.CheckCommand("java"); err != nil {
		return fmt.Errorf("java not found in PATH. Please install JDK 11+")
	}

	output, _ := m.exec.RunSilent("java", "-version")
	log.Printf("  Java version: %s", strings.TrimSpace(output))

	return nil
}

// SetupMono sets up Mono for C# support
func (m *Manager) SetupMono() error {
	log.Printf("→ Setting up Mono runtime")

	if err := executor.CheckCommand("mono"); err != nil {
		return fmt.Errorf("mono not found in PATH. Please install Mono runtime")
	}

	output, _ := m.exec.RunSilent("mono", "--version")
	log.Printf("  Mono version: %s", strings.TrimSpace(output))

	return nil
}

// SetupDotNet sets up .NET SDK for C# builds
func (m *Manager) SetupDotNet(version string) error {
	if version == "" {
		version = "6.0.x"
	}

	log.Printf("→ Setting up .NET SDK %s", version)

	if err := executor.CheckCommand("dotnet"); err != nil {
		return fmt.Errorf("dotnet SDK not found in PATH. Please install from https://dotnet.microsoft.com/download")
	}

	output, _ := m.exec.RunSilent("dotnet", "--version")
	log.Printf("  .NET version: %s", strings.TrimSpace(output))

	return nil
}

// getOSType returns the operating system type
func getOSType() string {
	return runtime.GOOS
}

// FreeDiskSpace removes unnecessary files to free up disk space
func (m *Manager) FreeDiskSpace() error {
	log.Printf("→ Freeing up disk space")

	// Check disk usage before
	if _, err := m.exec.RunSilent("df", "-h"); err == nil {
		log.Printf("  Disk usage (before):")
		m.exec.Run("df", "-h")
	}

	// Remove common large directories not needed for builds
	dirsToRemove := []string{
		"/usr/local/lib/android",
		"/opt/android",
	}

	for _, dir := range dirsToRemove {
		if _, err := os.Stat(dir); err == nil {
			log.Printf("  Removing %s", dir)
			os.RemoveAll(dir)
		}
	}

	// Check disk usage after
	if _, err := m.exec.RunSilent("df", "-h"); err == nil {
		log.Printf("  Disk usage (after):")
		m.exec.Run("df", "-h")
	}

	return nil
}

// CreateCacheDir creates the SCons cache directory
func (m *Manager) CreateCacheDir(cachePath string) error {
	if cachePath == "" {
		return nil
	}

	absPath, err := filepath.Abs(cachePath)
	if err != nil {
		return fmt.Errorf("invalid cache path: %w", err)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	log.Printf("Created SCons cache directory: %s", absPath)
	return nil
}
