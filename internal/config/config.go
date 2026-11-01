package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Platform represents a target platform for building
type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
	PlatformWeb     Platform = "web"
	PlatformAndroid Platform = "android"
)

// Target represents build target type
type Target string

const (
	TargetEditor          Target = "editor"
	TargetTemplateRelease Target = "template_release"
	TargetTemplateDebug   Target = "template_debug"
)

// BuildConfig holds all build configuration from environment variables
type BuildConfig struct {
	// Repository and version
	Repo           string
	Tag            string
	BaseBranch     string
	SourceDir      string
	OutputDir      string

	// Build options
	LTO                  string // "none", "thin", or "full"
	ModuleFlags          string
	EncryptionKey        string
	GithubToken          string

	// Build targets
	RunEditor       bool
	RunEditorMono   bool
	RunTemplate     bool
	RunTemplateMono bool

	// Platform filter
	Platforms []Platform

	// Build environment
	SCons                 SconsConfig
	AccessKitEnabled      *bool // nil = auto-detect
	D3D12Enabled          *bool // nil = auto-detect (Windows only)
	AngleEnabled          *bool // nil = auto-detect (Windows only)
}

// SconsConfig holds SCons-specific settings
type SconsConfig struct {
	Verbose      bool
	Warnings     string // "extra", "all", etc
	StrictChecks bool
	DebugSymbols bool
	CachePath    string
}

// LoadFromEnv loads build configuration from environment variables
func LoadFromEnv() (*BuildConfig, error) {
	cfg := &BuildConfig{
		Repo:                getEnv("GODOT_REPO", "godotengine/godot"),
		Tag:                 getEnv("GODOT_TAG", ""),
		BaseBranch:          getEnv("GODOT_BASE_BRANCH", "master"),
		SourceDir:           getEnv("SOURCE_DIR", "."),
		OutputDir:           getEnv("OUTPUT_DIR", "./bin"),
		LTO:                 getEnv("LTO", "none"),
		ModuleFlags:         getEnv("MODULE_FLAGS", ""),
		EncryptionKey:       getEnv("ENCRYPTION_KEY", ""),
		GithubToken:         getEnv("GITHUB_TOKEN", ""),
		RunEditor:           getBoolEnv("RUN_EDITOR", true),
		RunEditorMono:       getBoolEnv("RUN_EDITOR_MONO", false),
		RunTemplate:         getBoolEnv("RUN_TEMPLATE", true),
		RunTemplateMono:     getBoolEnv("RUN_TEMPLATE_MONO", false),
		SCons: SconsConfig{
			Verbose:      true,
			Warnings:     "extra",
			StrictChecks: true,
			DebugSymbols: getBoolEnv("DEBUG_SYMBOLS", false),
			CachePath:    getEnv("SCONS_CACHE_PATH", "./.scons_cache/"),
		},
	}

	// Validate tag is set
	if cfg.Tag == "" {
		return nil, fmt.Errorf("environment variable GODOT_TAG is required")
	}

	// Parse platforms to build
	platformsStr := getEnv("PLATFORMS", "linux,windows,web,android")
	platforms := []Platform{}
	for _, p := range strings.Split(platformsStr, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		switch p {
		case "linux":
			platforms = append(platforms, PlatformLinux)
		case "windows":
			platforms = append(platforms, PlatformWindows)
		case "web":
			platforms = append(platforms, PlatformWeb)
		case "android":
			platforms = append(platforms, PlatformAndroid)
		default:
			if p != "" {
				return nil, fmt.Errorf("unknown platform: %s", p)
			}
		}
	}
	if len(platforms) == 0 {
		return nil, fmt.Errorf("no valid platforms specified in PLATFORMS")
	}
	cfg.Platforms = platforms

	// Validate encryption key for template builds
	if cfg.EncryptionKey != "" {
		if err := validateEncryptionKey(cfg.EncryptionKey); err != nil {
			return nil, fmt.Errorf("invalid encryption key: %w", err)
		}
	}

	// Validate LTO mode
	if !isValidLTO(cfg.LTO) {
		return nil, fmt.Errorf("invalid LTO mode: %s (must be 'none', 'thin', or 'full')", cfg.LTO)
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

func validateEncryptionKey(key string) error {
	// Reject unsafe shell characters as per the original workflow
	unsafeChars := []string{";", "&", "|", "<", ">", "`", "$"}
	for _, char := range unsafeChars {
		if strings.Contains(key, char) {
			return fmt.Errorf("encryption key contains unsafe character: %s", char)
		}
	}
	return nil
}

func isValidLTO(mode string) bool {
	mode = strings.ToLower(strings.TrimSpace(mode))
	return mode == "none" || mode == "thin" || mode == "full"
}

// ShouldBuildEditor returns true if this build target should be built
func (cfg *BuildConfig) ShouldBuildEditor(mono bool) bool {
	if mono {
		return cfg.RunEditorMono
	}
	return cfg.RunEditor
}

// ShouldBuildTemplate returns true if this template build should be built
func (cfg *BuildConfig) ShouldBuildTemplate(mono bool) bool {
	if mono {
		return cfg.RunTemplateMono
	}
	return cfg.RunTemplate
}
