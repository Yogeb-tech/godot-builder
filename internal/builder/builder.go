package builder

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"godot-builder/internal/config"
	"godot-builder/internal/deps"
	"godot-builder/internal/executor"
	"godot-builder/internal/platforms"
)

// Builder orchestrates the Godot build process
type Builder struct {
	Config  *config.BuildConfig
	Exec    *executor.Executor
	DepsMgr *deps.Manager
}

// New creates a new builder
func New(cfg *config.BuildConfig) (*Builder, error) {
	// Validate source directory
	if _, err := os.Stat(cfg.SourceDir); err != nil {
		return nil, fmt.Errorf("source directory not found: %s", cfg.SourceDir)
	}

	// Create executor
	env := map[string]string{
		"GH_TOKEN":                    cfg.GithubToken,
		"GODOT_BASE_BRANCH":           cfg.BaseBranch,
		"DOTNET_NOLOGO":               "true",
		"DOTNET_CLI_TELEMETRY_OPTOUT": "true",
	}

	exec := executor.New(cfg.SourceDir, env)
	depsMgr := deps.New(exec)

	return &Builder{
		Config:  cfg,
		Exec:    exec,
		DepsMgr: depsMgr,
	}, nil
}

// Build executes the full build process
func (b *Builder) Build() error {
	log.Printf("  Godot Builder - Starting build process")
	log.Printf("  Repository: %s", b.Config.Repo)
	log.Printf("  Tag: %s", b.Config.Tag)
	log.Printf("  Platforms: %v", b.Config.Platforms)
	log.Printf("  Output directory: %s", b.Config.OutputDir)

	// Setup dependencies
	if err := b.setupDependencies(); err != nil {
		return fmt.Errorf("dependency setup failed: %w", err)
	}

	// Prepare output directory
	if err := b.prepareOutputDirectory(); err != nil {
		return fmt.Errorf("output directory preparation failed: %w", err)
	}

	// Build for each platform
	for _, platform := range b.Config.Platforms {
		if err := b.buildPlatform(platform); err != nil {
			return fmt.Errorf("build failed for %s: %w", platform, err)
		}
	}

	log.Printf("All builds completed successfully")
	return nil
}

// setupDependencies installs required tools and dependencies
func (b *Builder) setupDependencies() error {
	log.Printf("\n=== Setting up dependencies ===")

	// Setup Python and SCons
	if err := b.DepsMgr.SetupPythonAndScons("3.x", "4.9.0"); err != nil {
		log.Printf("Python/SCons setup warning: %v (trying to continue)", err)
	}

	// Create SCons cache directory
	if err := b.DepsMgr.CreateCacheDir(b.Config.SCons.CachePath); err != nil {
		log.Printf("Cache directory creation warning: %v", err)
	}

	// Setup platform-specific dependencies
	for _, platform := range b.Config.Platforms {
		if err := b.DepsMgr.InstallSystemDeps(string(platform)); err != nil {
			log.Printf("System dependency setup warning for %s: %v", platform, err)
		}

		switch platform {
		case config.PlatformWeb:
			if err := b.DepsMgr.SetupEmscripten("4.0.11"); err != nil {
				log.Printf("Emscripten setup warning: %v", err)
			}
		case config.PlatformAndroid:
			if err := b.DepsMgr.SetupJava(); err != nil {
				log.Printf("Java setup warning: %v", err)
			}
		}
	}

	return nil
}

// prepareOutputDirectory creates and validates the output directory
func (b *Builder) prepareOutputDirectory() error {
	log.Printf("\n=== Preparing output directory ===")

	if err := os.MkdirAll(b.Config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	absPath, err := filepath.Abs(b.Config.OutputDir)
	if err != nil {
		return err
	}

	log.Printf("✓ Output directory: %s", absPath)
	return nil
}

// buildPlatform builds for a specific platform
func (b *Builder) buildPlatform(platform config.Platform) error {
	log.Printf("\n=== Building for %s ===", platform)

	// Create platform-specific builder
	builder, err := platforms.NewBuilder(platform, b.Config, b.Exec)
	if err != nil {
		return err
	}

	log.Printf("→ Validating environment")
	if err := builder.ValidateEnvironment(); err != nil {
		return fmt.Errorf("environment validation failed for %s: %w", platform, err)
	}

	log.Printf("→ Starting %s build", builder.Name())
	return builder.Build()
}
