package platforms

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"godot-builder/internal/accesskit"
	"godot-builder/internal/config"
	"godot-builder/internal/executor"
)

// WindowsBuilder handles Windows platform builds
type WindowsBuilder struct {
	*BaseBuilder
	accesskitInstaller *accesskit.Installer
	d3d12Enabled       bool
	angleEnabled       bool
}

// NewWindowsBuilder creates a new Windows builder
func NewWindowsBuilder(cfg *config.BuildConfig, exec *executor.Executor) *WindowsBuilder {
	return &WindowsBuilder{
		BaseBuilder: &BaseBuilder{
			Platform: config.PlatformWindows,
			Config:   cfg,
			Exec:     exec,
			WorkDir:  cfg.SourceDir,
		},
		accesskitInstaller: accesskit.New(cfg.SourceDir, exec),
		d3d12Enabled:       true,
		angleEnabled:       true,
	}
}

// Name returns the platform name
func (wb *WindowsBuilder) Name() string {
	return "Windows"
}

// ValidateEnvironment checks if all required tools are available
func (wb *WindowsBuilder) ValidateEnvironment() error {
	required := []string{"python", "scons"}
	for _, cmd := range required {
		if _, err := exec.LookPath(cmd); err != nil {
			return fmt.Errorf("required tool not found: %s", cmd)
		}
	}

	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil {
			return fmt.Errorf("Windows cross-compiler not found: x86_64-w64-mingw32-g++; install MinGW-w64")
		}
	} else if _, err := exec.LookPath("cl.exe"); err != nil {
		log.Printf("⚠ MSVC compiler (cl.exe) not found. Ensure Visual Studio Build Tools are installed.")
	}

	return nil
}

// Build executes the Windows build process
func (wb *WindowsBuilder) Build() error {
	log.Printf("→ Starting Windows builds")

	// Setup platform-specific SDKs
	if err := wb.setupSDKs(); err != nil {
		log.Printf("⚠ SDK setup failed (build may continue without some features): %v", err)
	}

	// Install AccessKit SDK
	accesskitEnabled, _ := wb.accesskitInstaller.Install()
	wb.Config.AccessKitEnabled = &accesskitEnabled

	// Build matrix for Windows: x86_64, x86, arm64
	architectures := []struct {
		arch string
		name string
	}{
		{"x86_64", "x86_64"},
	}

	for _, arch := range architectures {
		// Template builds
		if wb.Config.ShouldBuildTemplate(false) {
			if err := wb.buildTemplate(arch.arch, config.TargetTemplateRelease); err != nil {
				return fmt.Errorf("template release build failed for %s: %w", arch.name, err)
			}
			if err := wb.buildTemplate(arch.arch, config.TargetTemplateDebug); err != nil {
				return fmt.Errorf("template debug build failed for %s: %w", arch.name, err)
			}
		}

		// Editor builds
		if wb.Config.ShouldBuildEditor(false) {
			if err := wb.buildEditor(arch.arch); err != nil {
				return fmt.Errorf("editor build failed for %s: %w", arch.name, err)
			}
		}

		// Mono builds (if enabled)
		if wb.Config.RunTemplateMono && wb.Config.ShouldBuildTemplate(true) {
			if err := wb.buildTemplateMono(arch.arch); err != nil {
				return fmt.Errorf("mono template build failed for %s: %w", arch.name, err)
			}
		}

		if wb.Config.RunEditorMono && wb.Config.ShouldBuildEditor(true) {
			if err := wb.buildEditorMono(arch.arch); err != nil {
				return fmt.Errorf("mono editor build failed for %s: %w", arch.name, err)
			}
		}
	}

	log.Printf("✓ Windows builds completed")
	return nil
}

// setupSDKs downloads and configures Windows SDKs
func (wb *WindowsBuilder) setupSDKs() error {
	log.Printf("→ Setting up Windows SDKs")

	// Try Direct3D 12 SDK (4.7+)
	d3d12ScriptPath := filepath.Join(wb.WorkDir, "misc", "scripts", "install_d3d12_sdk_windows.py")
	if _, err := os.Stat(d3d12ScriptPath); err == nil {
		log.Printf("  Installing Direct3D 12 SDK")
		cmd := exec.Command("python", d3d12ScriptPath)
		cmd.Dir = wb.WorkDir
		if err := cmd.Run(); err != nil {
			log.Printf("  ⚠ Direct3D 12 SDK installation failed")
			wb.d3d12Enabled = false
		} else {
			wb.d3d12Enabled = true
		}
	}

	// Try ANGLE SDK (4.7+)
	angleScriptPath := filepath.Join(wb.WorkDir, "misc", "scripts", "install_angle.py")
	if _, err := os.Stat(angleScriptPath); err == nil {
		log.Printf("  Installing ANGLE SDK")
		cmd := exec.Command("python", angleScriptPath)
		cmd.Dir = wb.WorkDir
		if err := cmd.Run(); err != nil {
			log.Printf("  ⚠ ANGLE SDK installation failed")
			wb.angleEnabled = false
		} else {
			wb.angleEnabled = true
		}
	}

	// Try WinRT (4.7+)
	winrtScriptPath := filepath.Join(wb.WorkDir, "misc", "scripts", "install_winrt.py")
	if _, err := os.Stat(winrtScriptPath); err == nil {
		log.Printf("  Installing WinRT components")
		cmd := exec.Command("python", winrtScriptPath)
		cmd.Dir = wb.WorkDir
		if err := cmd.Run(); err != nil {
			log.Printf("  warning: failed to install WinRT components: %v", err)
		}
	}

	return nil
}

// buildEditor builds the Windows editor
func (wb *WindowsBuilder) buildEditor(arch string) error {
	log.Printf("→ Building Windows editor (%s)", arch)

	flags := wb.buildSconsFlags(config.TargetEditor, arch, false)
	return wb.runScons(flags)
}

// buildTemplate builds a Windows template
func (wb *WindowsBuilder) buildTemplate(arch string, target config.Target) error {
	log.Printf("→ Building Windows template %s (%s)", target, arch)

	flags := wb.buildSconsFlags(target, arch, false)
	if wb.Config.EncryptionKey != "" && target != config.TargetEditor {
		wb.Exec.SetEnv("SCRIPT_AES256_ENCRYPTION_KEY", wb.Config.EncryptionKey)
	}
	return wb.runScons(flags)
}

// buildEditorMono builds the Windows Mono editor
func (wb *WindowsBuilder) buildEditorMono(arch string) error {
	log.Printf("→ Building Windows Mono editor (%s)", arch)

	// First, build the pre-glue version
	flags := wb.buildSconsFlags(config.TargetEditor, arch, true)
	flags += " mono_glue=no"
	if err := wb.runScons(flags); err != nil {
		return err
	}

	// Generate C# glue
	glueGenBinary := filepath.Join(".", "bin", fmt.Sprintf("godot.windows.editor.%s.mono.exe", arch))
	log.Printf("→ Generating C# glue using: %s", glueGenBinary)
	if err := wb.Exec.Run(glueGenBinary, "--headless", "--generate-mono-glue", "./modules/mono/glue"); err != nil {
		return fmt.Errorf("glue generation failed: %w", err)
	}

	// Build final Mono editor with glue
	flags = wb.buildSconsFlags(config.TargetEditor, arch, true)
	flags += " mono_glue=yes"
	return wb.runScons(flags)
}

// buildTemplateMono builds Windows Mono templates
func (wb *WindowsBuilder) buildTemplateMono(arch string) error {
	log.Printf("→ Building Windows Mono templates (%s)", arch)

	// The glue should already be generated by buildEditorMono
	for _, target := range []config.Target{config.TargetTemplateRelease, config.TargetTemplateDebug} {
		flags := wb.buildSconsFlags(target, arch, true)
		flags += " mono_glue=yes"

		if wb.Config.EncryptionKey != "" {
			wb.Exec.SetEnv("SCRIPT_AES256_ENCRYPTION_KEY", wb.Config.EncryptionKey)
		}

		if err := wb.runScons(flags); err != nil {
			return fmt.Errorf("mono template build failed: %w", err)
		}
	}

	return nil
}

// buildSconsFlags constructs SCons flags for the build
func (wb *WindowsBuilder) buildSconsFlags(target config.Target, arch string, isMono bool) string {
	var flags strings.Builder

	// Base flags
	flags.WriteString("platform=windows ")
	fmt.Fprintf(&flags, "target=%s ", target)
	fmt.Fprintf(&flags, "arch=%s ", arch)

	// Warnings and checks
	flags.WriteString("verbose=yes ")
	flags.WriteString("warnings=extra ")
	flags.WriteString("strict_checks=yes ")
	flags.WriteString("debug_symbols=no ")

	// Module flags (single source of truth)
	if wb.Config.ModuleFlags != "" {
		flags.WriteString(wb.Config.ModuleFlags)
		flags.WriteString(" ")
	}

	// Mono flags
	if isMono {
		flags.WriteString("module_mono_enabled=yes ")
		flags.WriteString("mono_glue=yes ")
		flags.WriteString("mono_static=yes ")
	}

	// Windows-specific flags
	flags.WriteString("use_static_cpp=yes ")
	if runtime.GOOS != "windows" {
		flags.WriteString("use_mingw=yes ")
	}

	// D3D12
	if wb.d3d12Enabled {
		flags.WriteString("d3d12=yes ")
	}

	// ANGLE
	if wb.angleEnabled {
		flags.WriteString("angle=yes ")
	}

	// AccessKit
	if wb.Config.AccessKitEnabled != nil && *wb.Config.AccessKitEnabled {
		flags.WriteString("accesskit=yes ")
	}

	// Cache
	fmt.Fprintf(&flags, "cache_path=%s ", wb.Config.SCons.CachePath)
	flags.WriteString("redirect_build_objects=no ")

	return strings.TrimSpace(flags.String())
}

// runScons executes SCons with each flag passed as a separate argument.
func (wb *WindowsBuilder) runScons(flags string) error {
	return wb.Exec.Run("scons", strings.Fields(flags)...)
}
