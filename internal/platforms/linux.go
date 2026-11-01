package platforms

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"godot-builder/internal/accesskit"
	"godot-builder/internal/config"
	"godot-builder/internal/executor"
)

// LinuxBuilder handles Linux platform builds
type LinuxBuilder struct {
	*BaseBuilder
	accesskitInstaller *accesskit.Installer
}

// NewLinuxBuilder creates a new Linux builder
func NewLinuxBuilder(cfg *config.BuildConfig, exec *executor.Executor) *LinuxBuilder {
	return &LinuxBuilder{
		BaseBuilder: &BaseBuilder{
			Platform: config.PlatformLinux,
			Config:   cfg,
			Exec:     exec,
			WorkDir:  cfg.SourceDir,
		},
		accesskitInstaller: accesskit.New(cfg.SourceDir, exec),
	}
}

// Name returns the platform name
func (lb *LinuxBuilder) Name() string {
	return "Linux"
}

// ValidateEnvironment checks if all required tools are available
func (lb *LinuxBuilder) ValidateEnvironment() error {
	required := []string{"python", "scons", "gcc", "g++"}
	for _, cmd := range required {
		if _, err := exec.LookPath(cmd); err != nil {
			return fmt.Errorf("required tool not found: %s", cmd)
		}
	}
	return nil
}

// Build executes the Linux build process
func (lb *LinuxBuilder) Build() error {
	log.Printf("→ Starting Linux builds")

	// Install dependencies if needed
	if err := lb.installDependencies(); err != nil {
		return fmt.Errorf("dependency installation failed: %w", err)
	}

	// Install AccessKit SDK
	// accesskitEnabled, _ := lb.accesskitInstaller.Install()
	// lb.Config.AccessKitEnabled = &accesskitEnabled

	// Build matrix: x86_64 and x86_32
	architectures := []struct {
		arch string
		name string
	}{
		{"x86_64", "x86_64"},
		// {"x86_32", "x86_32"},
	}

	for _, arch := range architectures {
		// Editor builds
		if lb.Config.ShouldBuildEditor(false) {
			if err := lb.buildEditor(arch.arch); err != nil {
				return fmt.Errorf("editor build failed for %s: %w", arch.name, err)
			}
		}

		// Template builds
		if lb.Config.ShouldBuildTemplate(false) {
			if err := lb.buildTemplate(arch.arch, config.TargetTemplateRelease); err != nil {
				return fmt.Errorf("template release build failed for %s: %w", arch.name, err)
			}
			if err := lb.buildTemplate(arch.arch, config.TargetTemplateDebug); err != nil {
				return fmt.Errorf("template debug build failed for %s: %w", arch.name, err)
			}
		}

		// Mono builds (if enabled)
		if lb.Config.RunEditorMono && lb.Config.ShouldBuildEditor(true) {
			if err := lb.buildEditorMono(arch.arch); err != nil {
				return fmt.Errorf("mono editor build failed for %s: %w", arch.name, err)
			}
		}

		if lb.Config.RunTemplateMono && lb.Config.ShouldBuildTemplate(true) {
			if err := lb.buildTemplateMono(arch.arch); err != nil {
				return fmt.Errorf("mono template build failed for %s: %w", arch.name, err)
			}
		}
	}

	log.Printf("✓ Linux builds completed")
	return nil
}

// installDependencies installs platform-specific dependencies
func (lb *LinuxBuilder) installDependencies() error {
	log.Printf("→ Installing Linux dependencies")

	// Basic dependencies
	// Only attempt automated apt-get installs when running as root to avoid prompting/sudo issues
	if os.Geteuid() != 0 {
		log.Printf("⚠ Not running as root; skipping automated apt-get installs")
		return nil
	}

	if _, err := exec.LookPath("apt-get"); err == nil {
		packages := []string{
			"build-essential",
			"libwayland-bin",
			"pkg-config",
		}

		for _, pkg := range packages {
			log.Printf("  Installing: %s", pkg)
			if err := lb.Exec.Run("apt-get", "install", "-y", pkg); err != nil {
				log.Printf("  ⚠ Failed to install %s (may already be installed)", pkg)
			}
		}
	}

	log.Printf("Linux dependencies installed")
	return nil
}

// buildEditor builds the Linux editor
func (lb *LinuxBuilder) buildEditor(arch string) error {
	log.Printf("→ Building Linux editor (%s)", arch)

	flags := lb.buildSconsFlags(config.TargetEditor, arch, false)
	return lb.runScons(flags)
}

// buildTemplate builds a Linux template
func (lb *LinuxBuilder) buildTemplate(arch string, target config.Target) error {
	log.Printf("→ Building Linux template %s (%s)", target, arch)

	flags := lb.buildSconsFlags(target, arch, false)
	return lb.runScons(flags)
}

// buildEditorMono builds the Linux Mono editor
func (lb *LinuxBuilder) buildEditorMono(arch string) error {
	log.Printf("→ Building Linux Mono editor (%s)", arch)

	// First, build the pre-glue version
	flags := lb.buildSconsFlags(config.TargetEditor, arch, true)
	flags += " mono_glue=no"
	if err := lb.runScons(flags); err != nil {
		return err
	}

	// Generate C# glue
	glueGenBinary := filepath.Join(".", "bin", fmt.Sprintf("godot.linuxbsd.editor.%s.mono", arch))
	log.Printf("→ Generating C# glue using: %s", glueGenBinary)
	if err := lb.Exec.Run(glueGenBinary, "--headless", "--generate-mono-glue", "./modules/mono/glue"); err != nil {
		return fmt.Errorf("glue generation failed: %w", err)
	}

	// Build final Mono editor with glue
	flags = lb.buildSconsFlags(config.TargetEditor, arch, true)
	flags += " mono_glue=yes"
	return lb.runScons(flags)
}

// buildTemplateMono builds Linux Mono templates
func (lb *LinuxBuilder) buildTemplateMono(arch string) error {
	log.Printf("→ Building Linux Mono templates (%s)", arch)

	// The glue should already be generated by buildEditorMono
	for _, target := range []config.Target{config.TargetTemplateRelease, config.TargetTemplateDebug} {
		flags := lb.buildSconsFlags(target, arch, true)
		flags += " mono_glue=yes"
		if err := lb.runScons(flags); err != nil {
			return fmt.Errorf("mono template build failed: %w", err)
		}
	}

	return nil
}

// buildSconsFlags constructs SCons flags for the build
func (lb *LinuxBuilder) buildSconsFlags(target config.Target, arch string, isMono bool) string {
	var flags strings.Builder

	// Base flags
	flags.WriteString("platform=linuxbsd ")
	fmt.Fprintf(&flags, "target=%s ", target)
	fmt.Fprintf(&flags, "arch=%s ", arch)

	// Verbose and warnings
	if lb.Config.SCons.Verbose {
		flags.WriteString("verbose=yes ")
	}
	fmt.Fprintf(&flags, "warnings=%s ", lb.Config.SCons.Warnings)
	flags.WriteString("werror=no ")
	flags.WriteString("strict_checks=yes ")

	// Module flags (single source of truth)
	if lb.Config.ModuleFlags != "" {
		flags.WriteString(lb.Config.ModuleFlags)
		flags.WriteString(" ")
	}

	// Mono flags
	if isMono {
		flags.WriteString("module_mono_enabled=yes ")
		flags.WriteString("mono_static=yes ")
	}

	// AccessKit
	if lb.Config.AccessKitEnabled != nil && *lb.Config.AccessKitEnabled {
		flags.WriteString("accesskit=yes ")
		fmt.Fprintf(&flags, "accesskit_sdk_path=%s/accesskit-c-0.21.2/ ", lb.WorkDir)
	} else {
		flags.WriteString("accesskit=no ")
	}

	// Cache
	fmt.Fprintf(&flags, "cache_path=%s ", lb.Config.SCons.CachePath)

	// Other settings
	flags.WriteString("redirect_build_objects=no ")

	return strings.TrimSpace(flags.String())
}

// runScons executes the SCons build command
func (lb *LinuxBuilder) runScons(flags string) error {
	log.Printf("→ Preparing SCons build")
	log.Printf("  Working directory: %s", lb.WorkDir)
	log.Printf("  Command: scons %s", flags)

	// Convert the flags string into individual command-line arguments.
	args := strings.Fields(flags)

	log.Printf("  SCons arguments: %v", args)
	log.Printf("  → Starting SCons...")

	// Handle editor builds
	if strings.Contains(flags, "target=editor") {
		err := lb.Exec.Run("scons", args...)
		if err != nil {
			log.Printf("  ✗ SCons returned an error: %v", err)
			return err
		}

		log.Printf("  ✓ SCons completed successfully")
		return nil
	}

	err := lb.Exec.Run("scons", args...)
	if err != nil {
		log.Printf("  ✗ SCons returned an error: %v", err)
		return err
	}

	log.Printf("  ✓ SCons completed successfully")

	return nil
}
