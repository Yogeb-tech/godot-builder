# Quick Start Guide

## Setup

### 1. Build the Binary

```bash
cd /home/yogeb/Sync/raw/web-development/godot-builder
go build -o ./bin/godot-builder ./cmd/godot-builder
```

### 2. Clone Godot Source

```bash
# Clone and checkout the tag properly
git clone https://github.com/godotengine/godot /tmp/godot
cd /tmp/godot
# fetch tags and check out the exact tag
git fetch --tags
git checkout tags/4.2.1-stable
git submodule update --init --recursive
```

### 3. Run a Build

```bash
export GODOT_TAG=4.2.1
export SOURCE_DIR=/tmp/godot
export PLATFORMS=linux
export OUTPUT_DIR=/tmp/godot-build
/path/to/godot-builder/bin/godot-builder
```

## Environment Variables Reference

### Required

- `GODOT_TAG` - Git tag/branch (e.g., `4.2.1`, `master`)

### Optional but Common

- `SOURCE_DIR` - Path to Godot source (default: `.`)
- `OUTPUT_DIR` - Artifact output directory (default: `./bin`)
- `PLATFORMS` - Build targets (default: `linux,windows,web,android`)
- `RUN_EDITOR` - Build editor (default: `true`)
- `RUN_TEMPLATE` - Build templates (default: `true`)
- `LTO` - Optimization: `none`, `thin`, `full` (default: `none`)

### Advanced

- `RUN_EDITOR_MONO` - Editor with C# (default: `false`)
- `RUN_TEMPLATE_MONO` - Templates with C# (default: `false`)
- `MODULE_FLAGS` - Extra SCons flags (e.g., `module_webm_enabled=no`)
- `ENCRYPTION_KEY` - AES256 key for script encryption
- `SCONS_CACHE_PATH` - Cache directory (default: `./.scons_cache/`)
- `DEBUG_SYMBOLS` - Include debug symbols (default: `false`)

## Common Tasks

### Linux Build (Fast)

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=linux
export SOURCE_DIR=/path/to/godot
./bin/godot-builder
```

### Web Build (Requires Emscripten)

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=web
export SOURCE_DIR=/path/to/godot
./bin/godot-builder
# Install Emscripten first if not available
```

### Android Build (Requires Java & Android SDK)

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=android
export SOURCE_DIR=/path/to/godot
./bin/godot-builder
```

### All Platforms

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=linux,windows,web,android
export SOURCE_DIR=/path/to/godot
./bin/godot-builder
# Builds all 4 platforms sequentially (~2-3 hours)
```

### With C# Support

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=linux
export RUN_EDITOR_MONO=true
export RUN_TEMPLATE_MONO=true
export SOURCE_DIR=/path/to/godot
./bin/godot-builder
```

### With Custom Modules

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=linux
export MODULE_FLAGS="module_webm_enabled=no module_svg_enabled=no"
export SOURCE_DIR=/path/to/godot
./bin/godot-builder
```

### With Optimization

```bash
export GODOT_TAG=4.2.1
export PLATFORMS=web
# Prefer to set LTO as part of MODULE_FLAGS now, e.g. MODULE_FLAGS="lto=thin" (or "lto=full")
export SOURCE_DIR=/path/to/godot
./bin/godot-builder
```

## Verify Installation

### Check Binary

```bash
file ./bin/godot-builder
# Should show: ELF 64-bit executable, x86-64, Go (on Linux)
```

### Check Artifacts

```bash
ls -la /tmp/godot-build/bin/
# Should show: godot.linuxbsd.editor.x86_64, etc.
```

### Test Binary Works

```bash
/tmp/godot-build/bin/godot.linuxbsd.editor.x86_64 --version
# Should print Godot version
```

## Docker Usage

### Build Docker Image

```dockerfile
FROM ubuntu:24.04

# Install system deps
RUN apt-get update && apt-get install -y \
    build-essential pkg-config \
    python3 python3-pip git \
    && rm -rf /var/lib/apt/lists/*

# Copy builder
COPY godot-builder /usr/local/bin/

# Set defaults
ENV GODOT_TAG=4.2.1
ENV PLATFORMS=linux

ENTRYPOINT ["/usr/local/bin/godot-builder"]
```

### Build Image

```bash
docker build -t godot-builder .
```

### Run Build

```bash
docker run -e GODOT_TAG=4.2.1 \
  -e PLATFORMS=linux \
  -v /path/to/godot:/godot \
  -v /tmp/artifacts:/artifacts \
  -e SOURCE_DIR=/godot \
  -e OUTPUT_DIR=/artifacts \
  godot-builder
```

## Troubleshooting

### Build Hangs

- Check disk space: `df -h`
- Check memory: `free -h`
- Increase timeout or check logs

### Missing Dependencies

```bash
# Install all common dependencies
apt-get install -y build-essential python3 python3-pip libwayland-bin

# For Web: Install Emscripten
git clone https://github.com/emscripten-core/emsdk.git
cd emsdk && ./emsdk install 4.0.11 && ./emsdk activate 4.0.11

# For Android: Install Java
apt-get install -y default-jdk

# For .NET/Mono
apt-get install -y mono-devel dotnet-sdk-6.0
```

### "scons not found"

```bash
python3 -m pip install scons==4.9.0
```

### "Python not found"

```bash
apt-get install python3  # Linux
brew install python3     # macOS
```

## Performance Tips

1. **Use Caching**

   ```bash
   export SCONS_CACHE_PATH=/ssd/cache  # Use fast storage
   ```

2. **Parallel Builds** (manual, requires script wrapper)

   ```bash
   # Build multiple platforms simultaneously
   PLATFORMS=linux ./bin/godot-builder &
   PLATFORMS=windows ./bin/godot-builder &
   wait
   ```

3. **Incremental Builds**
   - Don't delete cache directory between builds
   - Only changed files recompile (~5-10 min vs. 30-40 min)

4. **Resource Allocation**
   - 4+ CPU cores recommended
   - 16+ GB RAM recommended
   - 50+ GB free disk space recommended

## Next Steps

1. Read [README.md](README.md) for complete documentation
2. Review [ARCHITECTURE.md](ARCHITECTURE.md) for technical details
3. See [TRANSLATION.md](TRANSLATION.md) for workflow mapping
4. Check [DELIVERY.md](DELIVERY.md) for feature summary

## Support

For issues or questions:

1. Check [README.md](README.md) troubleshooting section
2. Review [TRANSLATION.md](TRANSLATION.md) for workflow construct details
3. Inspect logs: add `2>&1 | tee build.log` for full output
