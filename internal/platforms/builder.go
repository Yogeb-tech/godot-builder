package platforms

import (
	"fmt"

	"godot-builder/internal/config"
	"godot-builder/internal/executor"
)

// Builder defines the interface for platform-specific builders
type Builder interface {
	// Build executes the build process for this platform
	Build() error
	// Name returns the platform name
	Name() string
	// ValidateEnvironment checks if all required tools are available
	ValidateEnvironment() error
}

// BaseBuilder provides common build functionality
type BaseBuilder struct {
	Platform config.Platform
	Config   *config.BuildConfig
	Exec     *executor.Executor
	WorkDir  string
}

// NewBuilder creates the appropriate builder for a platform
func NewBuilder(platform config.Platform, cfg *config.BuildConfig, exec *executor.Executor) (Builder, error) {
	switch platform {
	case config.PlatformLinux:
		return NewLinuxBuilder(cfg, exec), nil
	case config.PlatformWindows:
		return NewWindowsBuilder(cfg, exec), nil
	case config.PlatformWeb:
		return NewWebBuilder(cfg, exec), nil
	case config.PlatformAndroid:
		return NewAndroidBuilder(cfg, exec), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// BuildStep represents a single build step
type BuildStep struct {
	Name        string
	Description string
	Enabled     bool
	Target      config.Target
	Arch        string
	IsMono      bool
	SconsFlags  string
}

// BuildSteps generates the build steps for a platform
type BuildSteps struct {
	steps []*BuildStep
}

func (bs *BuildSteps) Add(step *BuildStep) {
	if step != nil {
		bs.steps = append(bs.steps, step)
	}
}

func (bs *BuildSteps) GetEnabled() []*BuildStep {
	var enabled []*BuildStep
	for _, step := range bs.steps {
		if step.Enabled {
			enabled = append(enabled, step)
		}
	}
	return enabled
}

func (bs *BuildSteps) Count() int {
	return len(bs.steps)
}

func (bs *BuildSteps) CountEnabled() int {
	return len(bs.GetEnabled())
}
