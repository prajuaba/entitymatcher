#!/usr/bin/env bash
# ==============================================================================
# Entity Matcher Windows Runtime Package Builder
# Builds self-contained Windows server + UI packages for amd64/arm64
# ==============================================================================

set -euo pipefail

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Global state
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TMPDIRS=()
PG_STAGING_DIR=""

# Arguments
ARCH_TARGETS="both"
POSTGRES_ENABLED=true
GIT_REF=""
DIRTY_BUILD=false
OUTPUT_DIR="$REPO_ROOT/dist-windows"
CACHE_DIR="$REPO_ROOT/.build-cache"

# --- Parsing ---
usage() {
  echo "Usage: $0 [options]"
  echo ""
  echo "Options:"
  echo "  --arch <amd64|arm64|both>   default: both"
  echo "  --no-postgres               omit the bundled database (smaller package)"
  echo "  --ref <git-ref>             build from this ref instead of HEAD"
  echo "  --dirty                     build from the working tree, including uncommitted changes"
  echo "  --output <dir>              default: <repo>/dist-windows"
  echo "  --cache <dir>               default: <repo>/.build-cache"
  echo "  -h | --help"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --arch)
      case "$2" in
        amd64|arm64|both) ARCH_TARGETS="$2" ;;
        *) echo -e "${RED}Error: --arch must be amd64, arm64, or both${NC}" >&2; usage; exit 1 ;;
      esac
      shift 2
      ;;
    --no-postgres) POSTGRES_ENABLED=false; shift ;;
    --ref) GIT_REF="$2"; shift 2 ;;
    --dirty) DIRTY_BUILD=true; shift ;;
    --output) OUTPUT_DIR="$2"; shift 2 ;;
    --cache) CACHE_DIR="$2"; shift 2 ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo -e "${RED}Error: Unknown option: $1${NC}" >&2
      usage
      exit 1
      ;;
  esac
done

# --- Step 1: paths ---
mkdir -p "$OUTPUT_DIR"
mkdir -p "$CACHE_DIR"
if [[ ! -f "$CACHE_DIR/.gitignore" ]]; then
  echo '*' > "$CACHE_DIR/.gitignore"
fi

cleanup_all() {
  local dir
  for dir in "${TMPDIRS[@]+"${TMPDIRS[@]}"}"; do
    if [[ -d "$dir" ]]; then
      git -C "$REPO_ROOT" worktree remove --force "$dir" 2>/dev/null || rm -rf "$dir"
    fi
  done
}
trap cleanup_all EXIT

# --- Step 2: get a clean source tree ---
BUILD_ROOT=""
BUILD_COMMIT=""
BUILD_COMMIT_SUBJECT=""
BUILD_DIRTY_SUFFIX=""

if [[ "$DIRTY_BUILD" == true ]]; then
  echo -e "${YELLOW}====${NC}"
  echo -e "${YELLOW}! WARNING: Building from working tree including uncommitted changes!${NC}"
  echo -e "${YELLOW}! These local changes will be baked into the shipped binaries.${NC}"
  echo -e "${YELLOW}====${NC}"
  BUILD_ROOT="$REPO_ROOT"
  BUILD_COMMIT="$(git -C "$BUILD_ROOT" rev-parse HEAD)"
  BUILD_COMMIT_SUBJECT="$(git -C "$BUILD_ROOT" log -1 --format=%s)"
  if [[ -n "$(git -C "$BUILD_ROOT" status --porcelain)" ]]; then
    BUILD_DIRTY_SUFFIX="-dirty"
  fi
else
  echo -e "${BLUE}==>${NC} Preparing clean source tree..."
  BUILD_REF="${GIT_REF:-HEAD}"
  BUILD_WORKTREE="$(mktemp -d)"
  TMPDIRS+=("$BUILD_WORKTREE")
  git worktree add --detach "$BUILD_WORKTREE" "$BUILD_REF"
  BUILD_ROOT="$BUILD_WORKTREE"
  BUILD_COMMIT="$(git -C "$BUILD_ROOT" rev-parse HEAD)"
  BUILD_COMMIT_SUBJECT="$(git -C "$BUILD_ROOT" log -1 --format=%s)"
fi

# --- Step 3: frontend build ---
build_frontend() {
  echo -e "${BLUE}==>${NC} Building frontend..."
  if ! command -v npm >/dev/null 2>&1; then
    echo -e "${RED}Error: 'npm' not found. Please install Node.js and npm.${NC}"
    exit 1
  fi
  cd "$BUILD_ROOT/frontend"
  echo -e "${BLUE}==>${NC} Installing frontend dependencies..."
  npm ci
  echo -e "${BLUE}==>${NC} Building frontend SPA..."
  npm run build
}

build_frontend

# --- Step 4: backend build per architecture ---
build_backend() {
  local arch="$1"
  local out_dir="$2"
  echo -e "${BLUE}==>${NC} Building backend for $arch..."

  if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}Error: 'go' not found. Please install Go.${NC}"
    exit 1
  fi

  cd "$BUILD_ROOT/backend"
  GOOS=windows GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$out_dir/server.exe" .
}

# --- Step 5: PostgreSQL ---
PG_URL="https://get.enterprisedb.com/postgresql/postgresql-16.10-1-windows-x64-binaries.zip"
PG_SHA256="ebb3b6af4fa69dea9951b66855bc4d42dc04e56ccb9aa7024ce3c58bd89d6b0c"
PG_SIZE_BYTES=322530154

fetch_postgres() {
  if ! command -v curl >/dev/null 2>&1; then
    echo -e "${RED}Error: 'curl' not found. Please install curl.${NC}"
    exit 1
  fi

  local zip_path="$CACHE_DIR/postgresql-16.10-1-windows-x64-binaries.zip"
  if [[ ! -f "$zip_path" ]]; then
    echo -e "${BLUE}==>${NC} Downloading PostgreSQL 16.10 (Windows x64)..."
    curl -fL -o "$zip_path" "$PG_URL"
  else
    echo -e "${BLUE}==>${NC} PostgreSQL archive found in cache."
  fi

  # Check file size first (cheaper than checksum)
  local actual_size
  if [[ "$(uname)" == "Darwin" ]]; then
    actual_size="$(stat -f%z "$zip_path")"
  else
    actual_size="$(stat -c%s "$zip_path")"
  fi
  if [[ "$actual_size" != "$PG_SIZE_BYTES" ]]; then
    echo -e "${RED}Error: PostgreSQL archive size mismatch. Expected ${PG_SIZE_BYTES}, got ${actual_size}.${NC}"
    exit 1
  fi

  # Always verify
  echo -e "${BLUE}==>${NC} Verifying PostgreSQL archive integrity..."
  echo "${PG_SHA256}  ${zip_path}" | sha256sum -c - || {
    echo -e "${RED}Error: PostgreSQL archive checksum mismatch.${NC}"
    exit 1
  }
}

extract_and_trim_postgres() {
  local dest_dir="$1"
  local tmp_extract="$(mktemp -d)"
  TMPDIRS+=("$tmp_extract")

  if ! command -v unzip >/dev/null 2>&1; then
    echo -e "${RED}Error: 'unzip' not found. Please install unzip.${NC}"
    exit 1
  fi

  echo -e "${BLUE}==>${NC} Extracting PostgreSQL archive..."
  unzip -q "$CACHE_DIR/postgresql-16.10-1-windows-x64-binaries.zip" -d "$tmp_extract"

  # The EDB archive puts pgsql/ at its own root. The nested postgresql-*/pgsql
  # form is tried only as a fallback, in case a future archive is repackaged.
  local src_pgsql=""
  if [[ -d "$tmp_extract/pgsql" ]]; then
    src_pgsql="$tmp_extract/pgsql"
  else
    for dir in "$tmp_extract"/postgresql-*; do
      if [[ -d "$dir/pgsql" ]]; then
        src_pgsql="$dir/pgsql"
        break
      fi
    done
  fi

  [[ -z "$src_pgsql" ]] && { echo -e "${RED}Error: pgsql directory not found in PostgreSQL archive.${NC}"; exit 1; }

  # Copy to dest
  rm -rf "$dest_dir"
  cp -r "$src_pgsql" "$dest_dir"

  echo -e "${BLUE}==>${NC} Trimming unnecessary PostgreSQL components..."

  # GUI/admin tools, docs, C headers, extensions
  rm -rf "$dest_dir/pgAdmin 4/"        # GUI admin tool
  rm -rf "$dest_dir/doc/"             # documentation
  rm -rf "$dest_dir/include/"         # C headers
  rm -rf "$dest_dir/StackBuilder/"    # GUI installer/extension-manager

  # Unneeded localized data
  rm -rf "$dest_dir/share/locale/"    # translated UI strings

  # Static libs (used only to compile extensions, not to run)
  rm -rf "$dest_dir/lib/"*.lib

  # wxWidgets DLLs (only used by removed GUI tools)
  rm -rf "$dest_dir/bin/wx"*.dll

  # NOTE: DO NOT DELETE bin/icudt*.dll (~28MB) even though it appears unused:
  # ICU locale data is loaded dynamically at runtime and deletion breaks startup.
}

# --- Step 6: assemble the package tree per architecture ---
assemble_package() {
  local arch="$1"
  local pkg_dir="$OUTPUT_DIR/entitymatcher-windows-${arch}"

  echo -e "${BLUE}==>${NC} Assembling package for $arch..."

  rm -rf "$pkg_dir"
  mkdir -p "$pkg_dir"

  # CRITICAL: backend/ and frontend/ must be siblings under the package root
  # because server.exe resolves the SPA via `../frontend/dist` relative to its
  # current working directory (which is always `backend/` at runtime).
  # DO NOT rearrange these folders later.

  # Backend executable
  mkdir -p "$pkg_dir/backend"
  build_backend "$arch" "$pkg_dir/backend"

  # Frontend dist (built once, reused for all archs)
  mkdir -p "$pkg_dir/frontend"
  cp -r "$BUILD_ROOT/frontend/dist" "$pkg_dir/frontend/"

  # connector-data (copy verbatim)
  if [[ -d "$BUILD_ROOT/connector-data" ]]; then
    cp -r "$BUILD_ROOT/connector-data" "$pkg_dir/"
  else
    echo -e "${YELLOW}! Warning: connector-data/ not found in source tree.${NC}"
  fi

  # PostgreSQL (if enabled)
  if [[ "$POSTGRES_ENABLED" == true ]]; then
    if [[ -z "$PG_STAGING_DIR" ]]; then
      PG_STAGING_DIR="$(mktemp -d)"
      TMPDIRS+=("$PG_STAGING_DIR")
      extract_and_trim_postgres "$PG_STAGING_DIR/pgsql"
    else
      echo -e "${BLUE}==>${NC} Reusing trimmed PostgreSQL staging tree..."
    fi

    # For arm64: PostgreSQL x64 runs under Windows ARM64 emulation; this is supported
    # because the bundled pgsql is x64-only and EDB provides no native ARM64 build.
    if [[ "$arch" == "arm64" ]]; then
      echo -e "${YELLOW}==>${NC} Using x64 PostgreSQL in arm64 package (runs under Windows ARM64 emulation)..."
    fi

    cp -r "$PG_STAGING_DIR/pgsql" "$pkg_dir/pgsql"
  fi

  # Copy PowerShell packaging scripts (from repo root, not worktree)
  mkdir -p "$pkg_dir"
  cp "$REPO_ROOT/packaging/windows/Start-EntityMatcher.ps1" "$pkg_dir/"
  cp "$REPO_ROOT/packaging/windows/Stop-EntityMatcher.ps1" "$pkg_dir/"
  cp "$REPO_ROOT/packaging/windows/Start-Postgres.ps1" "$pkg_dir/"
  cp "$REPO_ROOT/packaging/windows/Stop-Postgres.ps1" "$pkg_dir/"
  cp "$REPO_ROOT/packaging/windows/README.txt" "$pkg_dir/"
  cp "$REPO_ROOT/packaging/windows/docker-compose.postgres.yml" "$pkg_dir/"
}

# --- Step 7: VERSION.txt ---
write_version_file() {
  local arch="$1"
  local pkg_dir="$OUTPUT_DIR/entitymatcher-windows-${arch}"

  local go_version=""
  # Why not `go version`? Because the launcher may be older (e.g., 1.24.x)
  # while the *toolchain* used is 1.25.x (Go auto-downloads on build).
  # The only reliable way is to inspect the built binary metadata.
  go_version="$(go version -m "$pkg_dir/backend/server.exe" 2>/dev/null | awk 'NR==1{print $2}')"

  local node_version="$(node --version)"
  local sp_files=()
  if [[ ! -d "$pkg_dir/frontend/dist/assets" ]]; then
    echo -e "${RED}Error: frontend assets directory missing.${NC}"
    exit 1
  fi
  while IFS= read -r f; do
    sp_files+=("$f")
  done < <(cd "$pkg_dir/frontend/dist/assets" && find . -type f -printf '%f\n')

  local pg_version=""
  if [[ "$POSTGRES_ENABLED" == true ]]; then
    pg_version="PostgreSQL 16.10"
  else
    pg_version="PostgreSQL: not bundled (--no-postgres)"
  fi

  cat > "$pkg_dir/VERSION.txt" << EOF
Commit: ${BUILD_COMMIT}${BUILD_DIRTY_SUFFIX}
Subject: ${BUILD_COMMIT_SUBJECT}
Build Time (UTC): $(date -u +"%Y-%m-%dT%H:%M:%SZ")
Target: windows/${arch}
Go Toolchain: ${go_version}
Node: ${node_version}
SPA Bundle Files: ${sp_files[*]}
PostgreSQL: ${pg_version}
EOF
}

# --- Step 8: archive ---
archive_package() {
  local arch="$1"
  local pkg_dir="$OUTPUT_DIR/entitymatcher-windows-${arch}"

  echo -e "${BLUE}==>${NC} Creating archive for $arch..."
  if ! command -v zip >/dev/null 2>&1; then
    echo -e "${RED}Error: 'zip' not found. Please install zip.${NC}"
    exit 1
  fi

  cd "$OUTPUT_DIR"
  zip -qr "entitymatcher-windows-${arch}.zip" "entitymatcher-windows-${arch}/"
}

# --- Step 9: PowerShell verification ---
verify_powershell_scripts() {
  if ! command -v pwsh >/dev/null 2>&1; then
    echo -e "${YELLOW}! Skipping PowerShell verification: 'pwsh' not found on PATH.${NC}"
    return 0
  fi

  local arch
  for arch in $ARCH_LIST; do
    local pkg_dir="$OUTPUT_DIR/entitymatcher-windows-${arch}"

    echo -e "${BLUE}==>${NC} Verifying PowerShell scripts for $arch..."

    local file
    for file in "$pkg_dir"/*.ps1; do
      [[ -f "$file" ]] || continue

      # Check UTF-8 BOM
      local bom
      bom="$(head -c 3 "$file" | od -An -tx1 | tr -d ' \n')"
      if [[ "$bom" != "efbbbf" ]]; then
        echo -e "${RED}Error: PowerShell script '$file' lacks UTF-8 BOM (need EF BB BF). Windows PowerShell 5.1 decodes BOM-less scripts using the system ANSI codepage, corrupting special characters like checkmarks/warnings.${NC}"
        exit 1
      fi

      # Parse syntax check
      if ! pwsh -NoProfile -Command "\$errors = \$null; [System.Management.Automation.Language.Parser]::ParseFile('${file//\'/\'\'}', [ref]\$null, [ref]\$errors) | Out-Null; exit \$errors.Count" 2>/dev/null; then
        echo -e "${RED}Error: PowerShell syntax error in '$file'.${NC}"
        exit 1
      fi
    done
  done
}

# --- Step 10: summary ---
print_summary() {
  echo ""
  echo -e "${GREEN}--------------------------------------------------------${NC}"
  echo -e "${GREEN}                    BUILD COMPLETE${NC}"
  echo -e "${GREEN}--------------------------------------------------------${NC}"

  # Generate SHA256SUMS.txt
  cd "$OUTPUT_DIR"
  local sha_file="$OUTPUT_DIR/SHA256SUMS.txt"
  : > "$sha_file"

  local arch
  for arch in $ARCH_LIST; do
    local zip_path="entitymatcher-windows-${arch}.zip"
    if [[ -f "$zip_path" ]]; then
      sha256sum "$zip_path" >> "$sha_file"
    fi
  done

  for arch in $ARCH_LIST; do
    local zip_path="entitymatcher-windows-${arch}.zip"
    if [[ -f "$zip_path" ]]; then
      local size_human
      size_human="$(du -h "$zip_path" | cut -f1)"
      local sha
      sha="$(grep "  $zip_path\$" "$sha_file" | cut -c1-64)"
      echo -e " Package: ${GREEN}${zip_path}${NC}"
      echo -e "   Size:  ${size_human}"
      echo -e "   SHA256: ${sha}"
    fi
  done

  echo -e "--------------------------------------------------------"
  echo -e " Output directory: ${OUTPUT_DIR}"
  echo -e " Cache directory:  ${CACHE_DIR}"
  echo -e "--------------------------------------------------------"
}

# --- MAIN EXECUTION ---
# Determine target arch list
ARCH_LIST=""
case "$ARCH_TARGETS" in
  amd64) ARCH_LIST="amd64" ;;
  arm64) ARCH_LIST="arm64" ;;
  both)  ARCH_LIST="amd64 arm64" ;;
esac

# Step 5: PostgreSQL fetch/trim (run only once, before arch loop)
if [[ "$POSTGRES_ENABLED" == true ]]; then
  fetch_postgres
fi

# Loop over architectures
for arch in $ARCH_LIST; do
  # Assemble package
  assemble_package "$arch"
  # Write version
  write_version_file "$arch"
  # Archive
  archive_package "$arch"
done

# Verify PowerShell scripts (per package)
verify_powershell_scripts

# Final summary
print_summary

exit 0
