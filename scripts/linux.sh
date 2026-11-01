#!/bin/bash
set -euo pipefail

# Quickstart smoke test for godot-builder

# Dependency check and (attempted) installer
command_exists() { command -v "$1" >/dev/null 2>&1; }

detect_pkg_manager() {
  if command_exists apt-get; then
    echo "apt"
  elif command_exists dnf; then
    echo "dnf"
  elif command_exists yum; then
    echo "yum"
  elif command_exists pacman; then
    echo "pacman"
  elif command_exists apk; then
    echo "apk"
  elif command_exists zypper; then
    echo "zypper"
  elif command_exists brew; then
    echo "brew"
  else
    echo ""
  fi
}

PKG_MANAGER=$(detect_pkg_manager)

REQUIRED_CMDS=(go git scons python3 pkg-config zip unzip)
MISSING_CMD=()
for c in "${REQUIRED_CMDS[@]}"; do
  if ! command_exists "$c"; then
    MISSING_CMD+=("$c")
  fi
done
# Require either clang or gcc
if command_exists clang || command_exists gcc; then
  : # ok
else
  MISSING_CMD+=(gcc)
fi

if [ ${#MISSING_CMD[@]} -ne 0 ]; then
  echo "Missing commands: ${MISSING_CMD[*]}"
  if [ -z "$PKG_MANAGER" ]; then
    echo "No supported package manager found. Please install: ${MISSING_CMD[*]} and re-run this script." >&2
    exit 1
  fi

  echo "Attempting to install missing packages using $PKG_MANAGER (requires sudo)."
  case "$PKG_MANAGER" in
  apt)
    sudo apt-get update --quiet
    sudo apt-get install -y git golang scons python3 pkg-config build-essential clang zip unzip
    ;;
  dnf)
    sudo dnf install -y git golang scons python3 pkgconfig gcc make clang zip unzip
    ;;
  yum)
    sudo yum install -y git golang scons python3 pkgconfig gcc make clang zip unzip
    ;;
  pacman)
    sudo pacman -Sy --noconfirm go git scons python pkgconf base-devel clang zip unzip
    ;;
  apk)
    sudo apk add --no-cache go git scons python3 pkgconfig build-base clang zip unzip
    ;;
  zypper)
    sudo zypper install -y git golang scons python3 pkg-config gcc make clang zip unzip
    ;;
  brew)
    brew install go scons python pkg-config llvm gcc zip unzip || true
    ;;
  *)
    echo "Unsupported package manager: $PKG_MANAGER. Please manually install: ${MISSING_CMD[*]}" >&2
    exit 1
    ;;
  esac

  # Re-check
  STILL_MISSING=()
  for c in "${MISSING_CMD[@]}"; do
    if ! command_exists "$c"; then
      STILL_MISSING+=("$c")
    fi
  done
  if [ ${#STILL_MISSING[@]} -ne 0 ]; then
    echo "After attempted install, still missing: ${STILL_MISSING[*]}" >&2
    echo "Please install them manually and re-run the script." >&2
    exit 1
  fi
fi

# Set up a Python virtual environment for SCons
VENV_DIR="/tmp/godot-builder-venv"
echo "==> Setting up Python virtual environment for SCons at $VENV_DIR"

# Ensure python3-venv is installed (Debian/Ubuntu specific)
if [[ "$PKG_MANAGER" == "apt" ]] && ! dpkg -s python3-venv >/dev/null 2>&1; then
  echo "  Installing python3-venv..."
  sudo apt-get install -y python3-venv
fi

# Create venv if it doesn't exist
if [ ! -d "$VENV_DIR" ]; then
  python3 -m venv "$VENV_DIR"
fi

# Activate venv by prepending its bin to PATH (so pip, scons are found)
export PATH="$VENV_DIR/bin:$PATH"

# Install SCons 4.9.0 inside the venv (this will succeed because venv is not system-managed)
echo "  Installing SCons 4.9.0 via pip in venv..."
pip install scons==4.9.0

# Verify scons is available
if ! command_exists scons; then
  echo "ERROR: scons not found after venv setup." >&2
  exit 1
fi
echo "  SCons version: $(scons --version | head -n1)"
# ---------------------------------------------------------------------

# Quickstart variables
BUILDER_DIR="/home/yogeb/Sync/raw/web-development/godot-builder"
GODOT_DIR="/tmp/godot"
OUTPUT_DIR="/tmp/godot-build"
GODOT_TAG="4.7"

echo "==> 1. Building godot-builder"
cd "$BUILDER_DIR"

go build -o ./bin/godot-builder ./cmd/godot-builder

echo "==> Builder binary:"
./bin/godot-builder --help || true

echo
echo "==> 2. Preparing Godot $GODOT_TAG"

if [ ! -d "$GODOT_DIR/.git" ]; then
  echo "    Cloning Godot (shallow clone to reduce download size)..."
  rm -rf "$GODOT_DIR"
  # Shallow clone the requested tag/branch to speed up downloads
  git clone --depth 1 --branch "${GODOT_TAG}-stable" https://github.com/godotengine/godot "$GODOT_DIR" ||
    git clone --depth 1 https://github.com/godotengine/godot "$GODOT_DIR"
fi

cd "$GODOT_DIR"

git fetch --tags

echo "    Checking out $GODOT_TAG..."
git checkout "tags/${GODOT_TAG}-stable"

echo "    Updating submodules..."
git submodule update --init --recursive --depth 1 || git submodule update --init --recursive

echo
echo "==> Godot revision:"
git describe --tags --always

echo
echo "==> 3. Running godot-builder"

cd "$BUILDER_DIR"

rm -rf "$OUTPUT_DIR"

export GODOT_TAG="$GODOT_TAG"
export SOURCE_DIR="$GODOT_DIR"
export PLATFORMS="linux"
export OUTPUT_DIR="$OUTPUT_DIR"

# Keep the quick test as small as possible.
export RUN_EDITOR="true"
export RUN_TEMPLATE="true"
export LTO="none"

# The venv's bin is already in PATH, so pip and scons will be found.
./bin/godot-builder -v

echo
echo "========================================"
echo " Quickstart test completed successfully"
echo "========================================"

echo
echo "Artifacts:"
find "$OUTPUT_DIR" -type f -maxdepth 4 -print

echo
echo "Checking for Godot binary..."

GODOT_BINARY="$OUTPUT_DIR/bin/godot.linuxbsd.editor.x86_64"

if [ -f "$GODOT_BINARY" ]; then
  echo
  echo "Godot version:"
  "$GODOT_BINARY" --version
else
  echo
  echo "WARNING: Expected binary not found:"
  echo "  $GODOT_BINARY"
  echo
  echo "Available files:"
  find "$OUTPUT_DIR" -type f -print
  exit 1
fi
