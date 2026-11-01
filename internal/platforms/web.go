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

// WebBuilder handles Web (Emscripten) platform builds
type WebBuilder struct {
	*BaseBuilder
}

// NewWebBuilder creates a new Web builder
func NewWebBuilder(cfg *config.BuildConfig, exec *executor.Executor) *WebBuilder {
	return &WebBuilder{
		BaseBuilder: &BaseBuilder{
			Platform: config.PlatformWeb,
			Config:   cfg,
			Exec:     exec,
			WorkDir:  cfg.SourceDir,
		},
	}
}

// Name returns the platform name
func (wb *WebBuilder) Name() string {
	return "Web (Emscripten)"
}

// ValidateEnvironment checks if all required tools are available
func (wb *WebBuilder) ValidateEnvironment() error {
	required := []string{"python", "scons", "emcc"}
	for _, cmd := range required {
		if _, err := exec.LookPath(cmd); err != nil {
			return fmt.Errorf("required tool not found: %s (Web builds require Emscripten)", cmd)
		}
	}

	// Verify Emscripten version
	if output, err := wb.Exec.RunSilent("emcc", "-v"); err == nil {
		log.Printf("  Emscripten version: %s", strings.TrimSpace(output))
	}

	return nil
}

// Build executes the Web build process
func (wb *WebBuilder) Build() error {
	log.Printf("→ Starting Web builds")

	// Web builds use both threaded and non-threaded variants
	variants := []struct {
		name  string
		dlink bool
		label string
	}{
		{"template_release", false, "template_release"},
		{"template_debug", false, "template_debug"},
	}

	for _, variant := range variants {
		if !wb.Config.ShouldBuildTemplate(false) {
			continue
		}

		target := config.TargetTemplateRelease
		if strings.Contains(variant.name, "debug") {
			target = config.TargetTemplateDebug
		}

		if err := wb.buildWeb(target, variant.dlink); err != nil {
			return fmt.Errorf("web build failed (%s): %w", variant.label, err)
		}
	}

	log.Printf("✓ Web builds completed")
	return nil
}

// buildWeb builds a Web template
func (wb *WebBuilder) buildWeb(target config.Target, dlinkEnabled bool) error {
	label := string(target)
	if dlinkEnabled {
		label += " (dlink)"
	}
	log.Printf("→ Building Web %s", label)

	if err := patchWebGL2Externs(wb.WorkDir); err != nil {
		return fmt.Errorf("patching WebGL2 Closure externs: %w", err)
	}

	flags := wb.buildSconsFlags(target, dlinkEnabled)

	if wb.Config.EncryptionKey != "" {
		wb.Exec.SetEnv("SCRIPT_AES256_ENCRYPTION_KEY", wb.Config.EncryptionKey)
	}

	return wb.Exec.Run("scons", strings.Fields(flags)...)
}

func patchWebGL2Externs(sourceDir string) error {
	externsPath := filepath.Join(sourceDir, "platform", "web", "js", "libs", "library_godot_webgl2.externs.js")
	content, err := os.ReadFile(externsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", externsPath, err)
	}

	const duplicateConstructor = `/**
 * @constructor OVR_multiview2
 */
function OVR_multiview2() {}

`
	if !strings.Contains(string(content), duplicateConstructor) {
		return nil
	}

	updated := strings.Replace(string(content), duplicateConstructor, "", 1)
	info, err := os.Stat(externsPath)
	if err != nil {
		return fmt.Errorf("statting %s: %w", externsPath, err)
	}
	if err := os.WriteFile(externsPath, []byte(updated), info.Mode()); err != nil {
		return fmt.Errorf("writing %s: %w", externsPath, err)
	}

	log.Printf("  Removed stale OVR_multiview2 Closure extern declaration")
	return nil
}

// buildSconsFlags constructs SCons flags for the build
func (wb *WebBuilder) buildSconsFlags(target config.Target, dlinkEnabled bool) string {
	var flags strings.Builder

	// Base flags
	flags.WriteString("platform=web ")
	fmt.Fprintf(&flags, "target=%s ", target)

	// Web-specific optimization
	flags.WriteString("verbose=yes ")
	flags.WriteString("warnings=extra ")
	flags.WriteString("werror=no ")
	flags.WriteString("debug_symbols=no ")
	flags.WriteString("optimize=size ")
	flags.WriteString("use_closure_compiler=yes ")
	flags.WriteString("strict_checks=yes ")

	// Module flags (single source of truth)
	if wb.Config.ModuleFlags != "" {
		flags.WriteString(wb.Config.ModuleFlags)
		flags.WriteString(" ")
	}

	// Windows-specific
	flags.WriteString("use_static_cpp=yes ")

	// DLink option
	if dlinkEnabled {
		flags.WriteString("dlink_enabled=yes ")
	}

	// Cache
	fmt.Fprintf(&flags, "cache_path=%s ", wb.Config.SCons.CachePath)
	flags.WriteString("redirect_build_objects=no ")

	return strings.TrimSpace(flags.String())
}
