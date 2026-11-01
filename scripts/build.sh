#!/bin/bash
set -euo pipefail

## CONFIGURATION

: "${BUILDER_DIR:=$PWD}"
: "${TMP_DIR:=$BUILDER_DIR/tmp}"
: "${GODOT_DIR:=$TMP_DIR/godot}"
: "${OUTPUT_DIR:=$TMP_DIR/godot-build}"
: "${VENV_DIR:=$TMP_DIR/venv}"
: "${EMSDK_DIR:=$TMP_DIR/emsdk}"
: "${GODOT_TAG:=4.7}"
: "${PLATFORMS:=windows}"
: "${RUN_EDITOR:=true}"
: "${RUN_TEMPLATE:=true}"
: "${LTO:=none}"
: "${BUILD_THREADS:=4}"

: "${GODOT_REPO:=https://github.com/godotengine/godot.git}"
: "${SCONS_VERSION:=4.9.0}"

: "${LOG_LEVEL:=info}"
: "${LOG_FILE:=./build.log}"

export BUILDER_DIR GODOT_DIR OUTPUT_DIR GODOT_TAG PLATFORMS
export RUN_EDITOR RUN_TEMPLATE LTO BUILD_THREADS
export TMP_DIR VENV_DIR EMSDK_DIR

mkdir -p "$TMP_DIR"

## LOGGING

log() {
  local level="$1"
  shift
  local msg="[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $*"
  echo "$msg"
  [[ -n "$LOG_FILE" ]] && echo "$msg" >>"$LOG_FILE"
}

log_info()  { log "INFO" "$@"; }
log_error() { log "ERROR" "$@" >&2; }
log_warn()  { log "WARN" "$@"; }
log_debug() { [[ "$LOG_LEVEL" == "debug" ]] && log "DEBUG" "$@"; }

## ERROR HANDLING

error_exit() {
  log_error "$@"
  exit 1
}

trap 'error_exit "Script interrupted"' INT TERM
trap 'log_error "Script failed on line $LINENO"' ERR

## DEPENDENCY CHECK AND INSTALLATION

command_exists() { command -v "$1" >/dev/null 2>&1; }

REQUIRED_CMDS=(go git scons python3 pkg-config zip unzip)
MISSING_CMD=()

for c in "${REQUIRED_CMDS[@]}"; do
  command_exists "$c" || MISSING_CMD+=("$c")
done

if ! command_exists clang && ! command_exists gcc; then
  MISSING_CMD+=(gcc)
fi

if [[ ${#MISSING_CMD[@]} -ne 0 ]]; then
  log_warn "Missing commands: ${MISSING_CMD[*]}"

  if [[ "${SKIP_DEP_INSTALL:-false}" == "true" ]]; then
    error_exit "Missing dependencies. Please install: ${MISSING_CMD[*]}"
  fi

  log_info "Installing missing packages using apt"

  export DEBIAN_FRONTEND=noninteractive
  sudo apt-get update --quiet
  sudo apt-get install -y git golang scons python3 pkg-config build-essential clang zip unzip

  STILL_MISSING=()
  for c in "${MISSING_CMD[@]}"; do
    command_exists "$c" || STILL_MISSING+=("$c")
  done

  if [[ ${#STILL_MISSING[@]} -ne 0 ]]; then
    error_exit "Still missing after install: ${STILL_MISSING[*]}"
  fi
fi

log_info "All required dependencies are installed"

## EMSCRIPTEN SETUP

setup_emscripten() {
  if [[ "$PLATFORMS" != *"web"* ]]; then
    log_info "Web builds not requested, skipping Emscripten setup"
    return 0
  fi

  if command_exists emcc; then
    log_info "Emscripten already installed: $(emcc --version | head -n1)"
    return 0
  fi

  log_info "Installing Emscripten via emsdk..."

  if [[ ! -d "$EMSDK_DIR" ]]; then
    git clone https://github.com/emscripten-core/emsdk.git "$EMSDK_DIR"
  fi

  cd "$EMSDK_DIR"
  ./emsdk install latest
  ./emsdk activate latest
  source "$EMSDK_DIR/emsdk_env.sh"

  if ! command_exists emcc; then
    error_exit "Failed to install Emscripten"
  fi

  log_info "Emscripten installed successfully: $(emcc --version | head -n1)"
}

setup_emscripten

## PYTHON VIRTUAL ENVIRONMENT SETUP

setup_venv() {
  local venv_dir="$VENV_DIR"
  log_info "Setting up Python virtual environment at $venv_dir"

  if ! dpkg -s python3-venv >/dev/null 2>&1; then
    log_info "Installing python3-venv..."
    export DEBIAN_FRONTEND=noninteractive
    sudo apt-get install -y python3-venv
  fi

  if [[ ! -d "$venv_dir" ]]; then
    python3 -m venv "$venv_dir"
  fi

  export PATH="$venv_dir/bin:$PATH"

  if ! pip show scons 2>/dev/null | grep -q "Version: $SCONS_VERSION"; then
    log_info "Installing SCons $SCONS_VERSION..."
    pip install --quiet "scons==$SCONS_VERSION"
  else
    log_debug "SCons $SCONS_VERSION already installed"
  fi

  if ! command_exists scons; then
    error_exit "SCons not found after venv setup"
  fi

  log_info "SCons version: $(scons --version | head -n1)"
}

setup_venv

## MINGW POSIX SETUP (for Windows builds)

setup_mingw_posix() {
  if [[ "$PLATFORMS" != *"windows"* ]]; then
    log_info "Windows builds not requested, skipping MinGW setup"
    return 0
  fi

  log_info "Setting up MinGW with POSIX threads for Windows builds"

  if ! dpkg -s g++-mingw-w64-x86-64-posix >/dev/null 2>&1; then
    log_info "Installing g++-mingw-w64-x86-64-posix and related packages..."
    export DEBIAN_FRONTEND=noninteractive
    sudo apt-get install -y g++-mingw-w64-x86-64-posix gcc-mingw-w64-x86-64-posix
  fi

  if update-alternatives --list x86_64-w64-mingw32-g++ &>/dev/null; then
    log_info "Switching MinGW alternatives to POSIX..."
    sudo update-alternatives --set x86_64-w64-mingw32-g++ /usr/bin/x86_64-w64-mingw32-g++-posix
    sudo update-alternatives --set x86_64-w64-mingw32-gcc /usr/bin/x86_64-w64-mingw32-gcc-posix
  else
    log_warn "POSIX alternative not found in update-alternatives. Falling back to environment variables."
    export CC=x86_64-w64-mingw32-gcc-posix
    export CXX=x86_64-w64-mingw32-g++-posix
    log_info "Set CC=$CC, CXX=$CXX"
  fi

  if command -v x86_64-w64-mingw32-g++ >/dev/null; then
    local thread_model
    thread_model=$(x86_64-w64-mingw32-g++ -v 2>&1 | grep -i "Thread model" | awk '{print $3}' || echo "unknown")
    if [[ "$thread_model" == "posix" ]]; then
      log_info "MinGW thread model is POSIX (good)"
    else
      error_exit "MinGW thread model is '$thread_model', expected 'posix'"
    fi
  else
    error_exit "x86_64-w64-mingw32-g++ not found; cannot build for Windows."
  fi
}

## GODOT REPOSITORY SETUP

setup_godot_repo() {
  log_info "Setting up Godot repository ($GODOT_TAG)"

  if [[ ! -d "$GODOT_DIR/.git" ]]; then
    log_info "Cloning Godot (shallow clone)..."
    rm -rf "$GODOT_DIR"

    if ! git clone --depth 1 --branch "${GODOT_TAG}-stable" "$GODOT_REPO" "$GODOT_DIR" 2>/dev/null; then
      log_warn "Tag ${GODOT_TAG}-stable not found, cloning default branch..."
      git clone --depth 1 "$GODOT_REPO" "$GODOT_DIR"
    fi
  fi

  cd "$GODOT_DIR"

  git fetch --tags --depth=1 2>/dev/null || git fetch --tags

  if git show-ref "tags/${GODOT_TAG}-stable" >/dev/null 2>&1; then
    git checkout "tags/${GODOT_TAG}-stable"
  else
    log_warn "Tag ${GODOT_TAG}-stable not found, using current HEAD"
  fi

  log_info "Updating submodules..."
  git submodule update --init --recursive --depth 1 ||
    git submodule update --init --recursive

  log_info "Godot revision: $(git describe --tags --always 2>/dev/null || git rev-parse --short HEAD)"
}

setup_godot_repo

# --- MinGW POSIX fix applied here ---
setup_mingw_posix

## BUILD GODOT-BUILDER

build_builder() {
  log_info "Building godot-builder"

  if [[ ! -d "$BUILDER_DIR" ]]; then
    error_exit "godot-builder directory not found: $BUILDER_DIR"
  fi

  cd "$BUILDER_DIR"

  log_info "Running go build..."
  go build -o ./bin/godot-builder ./cmd/builder

  if [[ ! -f "./bin/godot-builder" ]]; then
    error_exit "Failed to build godot-builder"
  fi

  log_info "godot-builder built successfully"
}

build_builder

## RUN GODOT-BUILDER

run_builder() {
  log_info "Starting Godot build for platforms: $PLATFORMS"
  log_info "Output directory: $OUTPUT_DIR"

  rm -rf "$OUTPUT_DIR"
  mkdir -p "$OUTPUT_DIR"

  cd "$BUILDER_DIR"

  export SOURCE_DIR="$GODOT_DIR"

  local extra_args="${GODOT_BUILDER_ARGS:-}"

  log_info "Executing: ./bin/godot-builder -v $extra_args"

  if ! ./bin/godot-builder -v $extra_args; then
    error_exit "godot-builder failed"
  fi
}

run_builder

## VERIFY BUILD OUTPUT

verify_output() {
  log_info "Verifying build artifacts"

  echo
  echo "========================================"
  echo " Build completed successfully"
  echo "========================================"
  echo
  echo "Artifacts:"
  find "$OUTPUT_DIR" -type f -maxdepth 4 -print | head -20

  echo "BUILD_PLATFORMS=$PLATFORMS" >>"$OUTPUT_DIR/build_metadata.env"
  echo "BUILD_TAG=$GODOT_TAG" >>"$OUTPUT_DIR/build_metadata.env"
  echo "BUILD_TIMESTAMP=$(date -u +%Y%m%d_%H%M%S)" >>"$OUTPUT_DIR/build_metadata.env"
}

verify_output

log_info "All done!"
