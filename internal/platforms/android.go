// Package platforms build different godot builds depending on requested platform
package platforms

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"godot-builder/internal/config"
	"godot-builder/internal/executor"
)

// AndroidBuilder handles Android platform builds
type AndroidBuilder struct {
	*BaseBuilder
}

// NewAndroidBuilder creates a new Android builder
func NewAndroidBuilder(cfg *config.BuildConfig, exec *executor.Executor) *AndroidBuilder {
	return &AndroidBuilder{
		BaseBuilder: &BaseBuilder{
			Platform: config.PlatformAndroid,
			Config:   cfg,
			Exec:     exec,
			WorkDir:  cfg.SourceDir,
		},
	}
}

// Name returns the platform name
func (ab *AndroidBuilder) Name() string {
	return "Android"
}

// ValidateEnvironment checks if all required tools are available
func (ab *AndroidBuilder) ValidateEnvironment() error {
	required := []string{"python", "scons", "java", "gradlew"}
	for _, cmd := range required {
		if _, err := exec.LookPath(cmd); err != nil {
			if cmd != "gradlew" {
				return fmt.Errorf("required tool not found: %s (Android builds require Android SDK/NDK, Java, and SCons)", cmd)
			}
		}
	}

	// Check for Gradle wrapper
	gradlewPath := filepath.Join(ab.WorkDir, "platform", "android", "java", "gradlew")
	if _, err := os.Stat(gradlewPath); err != nil {
		log.Printf("⚠ Gradle wrapper not found at %s", gradlewPath)
	}

	return nil
}

// Build executes the Android build process
func (ab *AndroidBuilder) Build() error {
	log.Printf("→ Starting Android builds")

	// Android builds require Java setup
	if err := ab.setupJavaEnvironment(); err != nil {
		return fmt.Errorf("java setup failed: %w", err)
	}

	// Android uses two architectures: arm64 and arm32
	architectures := []string{"arm64", "arm32"}

	// Build templates
	if ab.Config.ShouldBuildTemplate(false) {
		for _, arch := range architectures {
			if err := ab.buildTemplate(arch); err != nil {
				return fmt.Errorf("template build failed for %s: %w", arch, err)
			}
		}
	}

	// Build Mono templates (if enabled)
	if ab.Config.RunTemplateMono && ab.Config.ShouldBuildTemplate(true) {
		// Generate C# glue first (if needed)
		if err := ab.generateMonoGlue(); err != nil {
			return fmt.Errorf("mono glue generation failed: %w", err)
		}

		for _, arch := range architectures {
			if err := ab.buildTemplateMono(arch); err != nil {
				return fmt.Errorf("mono template build failed for %s: %w", arch, err)
			}
		}

		// Build .NET assemblies
		if err := ab.buildDotNetAssemblies(); err != nil {
			return fmt.Errorf("failed to build .NET assemblies: %w", err)
		}
	}

	// Generate APK and template packages
	if err := ab.generateTemplatePackages(); err != nil {
		return fmt.Errorf("template package generation failed: %w", err)
	}

	log.Printf("✓ Android builds completed")
	return nil
}

// setupJavaEnvironment configures Java environment for builds
func (ab *AndroidBuilder) setupJavaEnvironment() error {
	log.Printf("→ Setting up Java environment")

	// Check Java version
	output, err := ab.Exec.RunSilent("java", "-version")
	if err != nil {
		return fmt.Errorf("java not available: %w", err)
	}
	log.Printf("  Java version: %s", strings.TrimSpace(output))

	// Verify Android tools if they exist
	if _, err := exec.LookPath("adb"); err == nil {
		output, _ := ab.Exec.RunSilent("adb", "version")
		log.Printf("  ADB version: %s", strings.TrimSpace(output))
	}

	return nil
}

// buildTemplate builds an Android template for a specific architecture
func (ab *AndroidBuilder) buildTemplate(arch string) error {
	log.Printf("→ Building Android template (%s)", arch)

	flags := ab.buildSconsFlags(arch, false)

	if ab.Config.EncryptionKey != "" {
		ab.Exec.SetEnv("SCRIPT_AES256_ENCRYPTION_KEY", ab.Config.EncryptionKey)
	}

	return ab.Exec.Run("scons", flags)
}

// buildTemplateMono builds an Android Mono template
func (ab *AndroidBuilder) buildTemplateMono(arch string) error {
	log.Printf("→ Building Android Mono template (%s)", arch)

	flags := ab.buildSconsFlags(arch, true)

	if ab.Config.EncryptionKey != "" {
		ab.Exec.SetEnv("SCRIPT_AES256_ENCRYPTION_KEY", ab.Config.EncryptionKey)
	}

	return ab.Exec.Run("scons", flags)
}

// generateMonoGlue generates C# glue for Mono builds
func (ab *AndroidBuilder) generateMonoGlue() error {
	log.Printf("→ Generating C# glue for Mono")

	// First, build a Linux editor for glue generation
	log.Printf("  Building Linux editor (for glue generation)")

	linuxFlags := []string{
		"platform=linuxbsd",
		"target=editor",
		"arch=x86_64",
		"module_mono_enabled=yes",
		"mono_glue=no",
	}

	if ab.Config.ModuleFlags != "" {
		linuxFlags = append(linuxFlags, ab.Config.ModuleFlags)
	}

	if err := ab.Exec.Run("scons", strings.Join(linuxFlags, " ")); err != nil {
		return fmt.Errorf("linux editor build for glue generation failed: %w", err)
	}

	// Generate glue
	glueGenBinary := filepath.Join(".", "bin", "godot.linuxbsd.editor.x86_64.mono")
	log.Printf("  Generating glue using: %s", glueGenBinary)
	return ab.Exec.Run(glueGenBinary, "--headless", "--generate-mono-glue", "./modules/mono/glue")
}

// buildDotNetAssemblies builds .NET assemblies for Android
func (ab *AndroidBuilder) buildDotNetAssemblies() error {
	log.Printf("→ Building .NET assemblies")

	scriptPath := filepath.Join(ab.WorkDir, "modules", "mono", "build_scripts", "build_assemblies.py")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("build_assemblies.py not found: %w", err)
	}

	return ab.Exec.Run("python", scriptPath,
		"--godot-output-dir=./bin",
		"--godot-platform=android",
		"--werror")
}

// generateTemplatePackages generates APK and template packages
func (ab *AndroidBuilder) generateTemplatePackages() error {
	log.Printf("→ Generating Android APK/template packages")

	gradlewDir := filepath.Join(ab.WorkDir, "platform", "android", "java")
	if _, err := os.Stat(gradlewDir); err != nil {
		return fmt.Errorf("android gradle directory not found: %w", err)
	}

	// Run gradle to generate templates
	if _, err := os.Stat(filepath.Join(gradlewDir, "gradlew")); err == nil {
		log.Printf("  Running gradlew generateGodotTemplates")
		cmd := exec.Command("./gradlew", "generateGodotTemplates")
		cmd.Dir = gradlewDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gradle generateGodotTemplates failed: %w", err)
		}
	}

	// List generated artifacts
	binDir := filepath.Join(ab.WorkDir, "bin")
	log.Printf("  Artifacts in %s:", binDir)
	if entries, err := os.ReadDir(binDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				log.Printf("    - %s", entry.Name())
			}
		}
	}

	return nil
}

// buildSconsFlags constructs SCons flags for the build
func (ab *AndroidBuilder) buildSconsFlags(arch string, isMono bool) string {
	var flags strings.Builder

	// Base flags
	flags.WriteString("platform=android ")
	flags.WriteString("target=template_release ")
	fmt.Fprintf(&flags, "arch=%s ", arch)

	// Warnings and checks
	flags.WriteString("verbose=yes ")
	flags.WriteString("warnings=extra ")
	flags.WriteString("werror=yes ")
	flags.WriteString("debug_symbols=no ")
	flags.WriteString("strict_checks=yes ")

	// Module flags (single source of truth)
	if ab.Config.ModuleFlags != "" {
		flags.WriteString(ab.Config.ModuleFlags)
		flags.WriteString(" ")
	}

	// Mono flags
	if isMono {
		flags.WriteString("module_mono_enabled=yes ")
		flags.WriteString("mono_glue=yes ")
	}

	// Cache
	fmt.Fprintf(&flags, "cache_path=%s ", ab.Config.SCons.CachePath)
	flags.WriteString("redirect_build_objects=no ")

	return strings.TrimSpace(flags.String())
}
