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

## Workflow to Go Translation

This section explains how GitHub Actions constructs map to Go code:

### GitHub Actions Matrix → Go Loop

**Workflow:**

```yaml
strategy:
  matrix:
    include:
      - target: editor
        arch: x86_64
      - target: template_release
        arch: x86_64
```

**Go Translation:**

```go
architectures := []string{"x86_64"}
for _, arch := range architectures {
  if cfg.ShouldBuildEditor(false) {
    lb.buildEditor(arch)
  }
  // ...
}
```

### Conditional Steps → Boolean Guards

**Workflow:**

```yaml
- name: Install Mono
  if: matrix.build-mono == true
  run: apt-get install -y mono-devel
```

**Go Translation:**

```go
if isMono {
  // Install Mono via script
}
```

### Environment Variable Masking → Validation

**Workflow:**

```yaml
- name: Map encryption key
  if: inputs.encryption_key != ''
  run: |
    if echo "$KEY" | grep -Eq '[;&|<>`$]'; then
      exit 1
    fi
    echo "ENCRYPTION_KEY=$KEY" >> $GITHUB_ENV
```

**Go Translation:**

```go
func validateEncryptionKey(key string) error {
  unsafeChars := []string{";", "&", "|", "<", ">", "`", "$"}
  for _, char := range unsafeChars {
    if strings.Contains(key, char) {
      return fmt.Errorf("unsafe character: %s", char)
    }
  }
  return nil
}
```

### Artifact Upload → Artifact Handler

**Workflow:**

```yaml
- name: Upload artifact
  uses: actions/upload-artifact@v7
  with:
    name: linux-${{ matrix.target }}-${{ matrix.arch }}
    path: bin
```

**Go Translation:**

```go
handler := artifacts.New(cfg.OutputDir)
handler.CopyToOutput("./bin/*")
```

### Secrets & Outputs → Environment Variables

**Workflow:**

```yaml
env:
  SCRIPT_AES256_ENCRYPTION_KEY: ${{ inputs.encryption_key }}
```

**Go Translation:**

```go
exec.SetEnv("SCRIPT_AES256_ENCRYPTION_KEY", cfg.EncryptionKey)
```

## Behavioral Differences & Limitations

### What's Preserved

✓ Multi-architecture support (x86_64, x86_32, arm64, arm32)
✓ Mono/C# build support with glue generation
✓ Script encryption for templates
✓ Platform-specific SDKs (AccessKit, D3D12, ANGLE, Emscripten)
✓ Caching via SCons cache directory
✓ Artifact organization by platform/target
✓ Same SCons command structure and flags

### Limitations & Workarounds

1. **No Cache Cleanup Script (4.7+ feature)**
   - Workflows optionally use a purge script to clean old cache
   - Go version: Set `SCONS_CACHE_PATH` to a clean directory or implement external cleanup

2. **No Workflow Trigger Events**
   - Workflows respond to GitHub events (PR, push, schedule)
   - Go version: Triggered manually via environment variables; integrate with your own scheduler

3. **No Artifact Download/Upload Actions**
   - Workflows use GitHub's artifact storage
   - Go version: Stores artifacts in local directory; integrate with your own storage system

4. **No Problem Matchers**
   - Workflows use problem-matcher JSON for enhanced error reporting
   - Go version: Logs to stdout/stderr; integrate with your parsing tools

5. **No Parallel Jobs**
   - Workflows run matrix jobs in parallel via GitHub Actions
   - Go version: Runs platforms sequentially; can be parallelized in future versions

## Extending the Builder

### Adding a New Platform

1. Create `internal/platforms/newplatform.go`:

```go
type NewPlatformBuilder struct {
  *BaseBuilder
}

func (npb *NewPlatformBuilder) Name() string { return "NewPlatform" }
func (npb *NewPlatformBuilder) ValidateEnvironment() error { /* ... */ }
func (npb *NewPlatformBuilder) Build() error { /* ... */ }
```

1. Register in `internal/platforms/builder.go`:

```go
case config.PlatformNewPlatform:
  return NewNewPlatformBuilder(cfg, exec), nil
```

### Custom Dependency Installation

Override `deps.Manager.InstallSystemDeps()`:

```go
func (m *Manager) InstallSystemDeps(platform string) error {
  // Custom logic
}
```

## Testing

Build a test version:

```bash
go build -o ./bin/godot-builder-test ./cmd/godot-builder
```

Test with a dry run (no actual SCons):

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=linux
export SOURCE_DIR=/path/to/godot/source
export OUTPUT_DIR=/tmp/test-build
./bin/godot-builder-test
```

## Troubleshooting

### `GODOT_TAG is required`

Set the `GODOT_TAG` environment variable:

```bash
export GODOT_TAG=4.2.1
```

### `Python not found in PATH`

Install Python 3:

```bash
# Ubuntu/Debian
sudo apt-get install python3 python3-pip

# macOS
brew install python3
```

### `SCons installation failed`

Install manually:

```bash
python3 -m pip install scons==4.9.0
```

### `command failed: scons`

Ensure SCons can access the Godot source:

```bash
export SOURCE_DIR=/absolute/path/to/godot
export GODOT_TAG=4.2.1
```

## Performance Notes

- **First build**: ~30-60 minutes per platform (full compilation)
- **Cached build**: ~10-15 minutes per platform (reuses object files)
- **Web builds**: Slower due to Emscripten linking; ~20-40 minutes
- **Android builds**: Requires Java/Gradle; ~20-30 minutes

Cache directory can grow to 5-10 GB; manage with `SCONS_CACHE_PATH`.

## License

This Go implementation is provided as part of the Godot builder tools. See the original repository for licensing details.
