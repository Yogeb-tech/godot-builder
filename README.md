# Godot Builder - Go Implementation

A production-grade, standalone Go application that converts GitHub Actions Godot build workflows into a native executable. This tool faithfully reproduces the build logic from the original workflows without any GitHub Actions dependencies.

## Overview

This project replaces the GitHub Actions-based Godot build system with a native Go CLI that:

- **Parses workflow logic** from the original GitHub Actions workflows
- **Executes builds natively** in Go by invoking SCons and platform-specific tools
- **Supports multiple platforms**: Linux, Windows, Web (Emscripten), Android
- **Accepts configuration via environment variables** for easy integration with Docker, CI/CD, and automation systems
- **Produces identical build artifacts** as the original workflows (executables, templates, APKs, etc.)
- **Manages dependencies** (Python, SCons, Emscripten, Java, .NET, etc.)
- **Uses SQLite** to persist API build job state

## Building the Go Application

### Prerequisites

- Go 1.21 or later
- Make (optional, for convenience)

### Build

```bash
cd godot-builder
go build -o ./bin/godot-builder ./cmd/godot-builder
```

## Usage

### Environment Variables

All configuration is via environment variables (no CLI flags):

#### Required Variables

- `GODOT_TAG` - Git tag/branch to build (e.g., `4.2.1`, `master`)
- `GODOT_REPO` - Repository URL or path (default: `godotengine/godot`)

#### Optional Variables

- `SOURCE_DIR` - Path to Godot source directory (default: `.`)
- `OUTPUT_DIR` - Where to place built artifacts (default: `./bin`)
- `PLATFORMS` - Comma-separated list: `linux,windows,web,android` (default: all non-Apple)

#### Build Options

- `LTO` - Link-time optimization: `none`, `thin`, `full` (default: `none`)
- `MODULE_FLAGS` - Extra SCons flags, e.g., `module_webm_enabled=no`
- `TEMPLATE_MODULE_FLAGS` - Extra flags for template builds only
- `ENCRYPTION_KEY` - AES256 key for script encryption (template builds only)

#### Build Targets

- `RUN_EDITOR` - Build editor (default: `true`)
- `RUN_EDITOR_MONO` - Build editor with C# support (default: `false`)
- `RUN_TEMPLATE` - Build export templates (default: `true`)
- `RUN_TEMPLATE_MONO` - Build export templates with C# support (default: `false`)

#### Caching & Environment

- `SCONS_CACHE_PATH` - SCons cache directory (default: `./.scons_cache/`)
- `DEBUG_SYMBOLS` - Include debug symbols (default: `false`)
- `GITHUB_TOKEN` - For authenticated API access if needed (optional)

### Examples

#### Linux x86_64 builds

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=linux
export OUTPUT_DIR=./artifacts
./bin/godot-builder
```

### Build API job storage

Run the API with `go run ./cmd/api`. Job state is stored in `tmp/jobs.sqlite` by default; set `JOBS_DB_PATH` to use another SQLite file.

#### Web template with optimization

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=web
export LTO=thin
export MODULE_FLAGS="module_webm_enabled=no"
./bin/godot-builder
```

#### Android with Mono

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=android
export RUN_TEMPLATE_MONO=true
./bin/godot-builder
```

#### Multiple platforms

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=linux,windows,web,android
export RUN_EDITOR=false
export RUN_TEMPLATE=true
./bin/godot-builder
```

## Project Structure

```
godot-builder/
├── cmd/
│   ├── api/
│   │   └── main.go                 # API entry point
│   └── godot-builder/
│       └── main.go                 # Entry point
├── internal/
│   ├── api/
│   │   └── server.go               # HTTP handlers and build worker
│   ├── jobs/
│   │   └── store.go                # SQLite-backed build job storage
│   ├── builder/
│   │   └── builder.go              # Main orchestrator
│   ├── config/
│   │   └── config.go               # Configuration from environment
│   ├── executor/
│   │   └── executor.go             # Command execution with logging
│   ├── platforms/
│   │   ├── builder.go              # Platform interface
│   │   ├── linux.go                # Linux builder
│   │   ├── windows.go              # Windows builder
│   │   ├── web.go                  # Web/Emscripten builder
│   │   └── android.go              # Android builder
│   ├── deps/
│   │   └── manager.go              # Dependency management
│   ├── accesskit/
│   │   └── installer.go            # AccessKit SDK installer
│   └── artifacts/
│       └── handler.go              # Artifact management
├── go.mod
└── README.md
```

## Architecture & Design

### Configuration (`internal/config`)

- Loads all settings from environment variables
- Validates inputs and rejects unsafe characters
- Provides type-safe access to configuration

### Execution (`internal/executor`)

- Wraps Go's `os/exec` with proper environment and error handling
- Supports both interactive (logs to stdout) and silent execution
- Maintains consistent environment variables across commands

### Platforms (`internal/platforms`)

- **Builder interface**: Each platform implements the same build workflow
- **LinuxBuilder**: Handles x86_64, x86_32 architectures; supports Mono
- **WindowsBuilder**: Handles x86_64; manages D3D12, ANGLE, AccessKit SDKs
- **WebBuilder**: Emscripten-based builds; supports threaded and dlink variants
- **AndroidBuilder**: arm64, arm32 architectures; Gradle APK generation; Mono support

### Dependencies (`internal/deps`)

- Detects and installs Python, SCons
- Manages platform-specific tools (Emscripten, Java, .NET)
- Creates and validates cache directories

### AccessKit (`internal/accesskit`)

- Auto-detects AccessKit availability
- Falls back to GitHub release downloads if Python script unavailable
- Gracefully degrades if installation fails
