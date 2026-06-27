#!/usr/bin/env bash
set -euo pipefail

REPO="${JAVBOSS_REPO:-Solr159/JavBoss}"
VERSION="${JAVBOSS_VERSION:-latest}"
INSTALL_DIR="${JAVBOSS_INSTALL_DIR:-}"
START_AFTER_INSTALL=1
TMP_DIR=""

prefers_chinese() {
  local locale_text
  locale_text="${JAVBOSS_LANG:-} ${LANGUAGE:-} ${LC_ALL:-} ${LC_MESSAGES:-} ${LANG:-}"
  case "$locale_text" in
    *zh*|*ZH*) return 0 ;;
    *) return 1 ;;
  esac
}

usage() {
  cat <<'EOF'
JavBoss installer for Linux and macOS.

Usage:
  install.sh [--version latest|v1.8.1] [--dir PATH] [--repo OWNER/REPO] [--no-start]

Environment:
  JAVBOSS_VERSION      Release tag to install. Default: latest
  JAVBOSS_INSTALL_DIR  Install directory.
  JAVBOSS_REPO         GitHub repository. Default: Solr159/JavBoss
EOF
}

log() {
  printf '[javboss] %s\n' "$*"
}

die() {
  printf '[javboss] ERROR: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n "${TMP_DIR:-}" ]]; then
    rm -rf "$TMP_DIR"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || die "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --dir)
      [[ $# -ge 2 ]] || die "--dir requires a value"
      INSTALL_DIR="$2"
      shift 2
      ;;
    --repo)
      [[ $# -ge 2 ]] || die "--repo requires a value"
      REPO="$2"
      shift 2
      ;;
    --no-start)
      START_AFTER_INSTALL=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

http_get() {
  local url="$1"
  if command_exists curl; then
    curl -fsSL "$url"
    return
  fi
  if command_exists wget; then
    wget -qO- "$url"
    return
  fi
  die "curl or wget is required"
}

download_file() {
  local url="$1"
  local dest="$2"
  if command_exists curl; then
    curl -fL --retry 3 -o "$dest" "$url"
    return
  fi
  if command_exists wget; then
    wget --tries=3 -O "$dest" "$url"
    return
  fi
  die "curl or wget is required"
}

detect_platform() {
  local os arch
  os="$(uname -s)"
  arch="$(uname -m)"

  case "$os" in
    Linux)
      case "$arch" in
        x86_64|amd64) printf 'linux-x86_64' ;;
        *) die "unsupported Linux architecture: $arch" ;;
      esac
      ;;
    Darwin)
      case "$arch" in
        x86_64|amd64) printf 'macos-x86_64' ;;
        arm64|aarch64) printf 'macos-arm64' ;;
        *) die "unsupported macOS architecture: $arch" ;;
      esac
      ;;
    *)
      die "unsupported OS: $os. Use scripts/install.ps1 on Windows"
      ;;
  esac
}

latest_tag() {
  local api json tag
  api="https://api.github.com/repos/${REPO}/releases/latest"
  json="$(http_get "$api")"
  tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  [[ -n "$tag" ]] || die "failed to read latest release tag from GitHub"
  printf '%s' "$tag"
}

normalize_tag() {
  local tag="$1"
  if [[ "$tag" == "latest" ]]; then
    latest_tag
    return
  fi
  if [[ "$tag" == v* ]]; then
    printf '%s' "$tag"
  else
    printf 'v%s' "$tag"
  fi
}

default_install_dir() {
  case "$(uname -s)" in
    Darwin) printf '%s/Applications/JavBoss' "$HOME" ;;
    *) printf '%s/.local/share/javboss' "$HOME" ;;
  esac
}

version_file_path() {
  printf '%s/.version' "$1"
}

installed_version() {
  local install_dir="$1"
  local version_file
  version_file="$(version_file_path "$install_dir")"
  if [[ -f "$install_dir/javboss" && -f "$version_file" ]]; then
    head -n 1 "$version_file" | tr -d '[:space:]'
  fi
}

write_installed_version() {
  local install_dir="$1"
  local tag="$2"
  printf '%s\n' "$tag" >"$(version_file_path "$install_dir")"
}

canonical_path() {
  local target="$1"
  local dir base
  dir="$(dirname "$target")"
  base="$(basename "$target")"
  if [[ -d "$dir" ]]; then
    (cd "$dir" && printf '%s/%s' "$(pwd -P)" "$base")
  else
    printf '%s' "$target"
  fi
}

running_javboss_pids() {
  local install_dir="$1"
  local exe
  exe="$(canonical_path "$install_dir/javboss")"

  if [[ "$(uname -s)" == "Linux" && -d /proc ]]; then
    local pid proc_exe
    for pid in $(pgrep -x javboss 2>/dev/null || true); do
      proc_exe="$(readlink "/proc/$pid/exe" 2>/dev/null || true)"
      if [[ "$proc_exe" == "$exe" ]]; then
        printf '%s\n' "$pid"
      fi
    done
    return
  fi

  if command_exists pgrep; then
    pgrep -f "$exe" 2>/dev/null || true
  fi
}

ensure_not_running() {
  local install_dir="$1"
  local pids
  if [[ ! -e "$install_dir/javboss" ]]; then
    return
  fi
  pids="$(running_javboss_pids "$install_dir" | tr '\n' ' ' | sed 's/[[:space:]]*$//')"
  if [[ -n "$pids" ]]; then
    if prefers_chinese; then
      die "JavBoss 已经从 $install_dir 启动（PID：$pids）。安装或升级前请先退出 JavBoss。"
    fi
    die "JavBoss is already running from $install_dir (pid: $pids). Please exit JavBoss before installing or upgrading."
  fi
}

copy_release_files() {
  local src="$1"
  local dest="$2"
  local protected_config=""

  mkdir -p "$dest"
  if [[ -f "$dest/config.toml" ]]; then
    protected_config="$(mktemp)"
    cp "$dest/config.toml" "$protected_config"
  fi

  rm -rf "$dest/internal" "$dest/web" "$dest/modernz"
  rm -f "$dest/javboss" "$dest/javboss.exe" "$dest/javboss.command"
  cp -R "$src"/. "$dest"/

  if [[ -n "$protected_config" ]]; then
    cp "$protected_config" "$dest/config.toml"
    rm -f "$protected_config"
  fi
}

create_command_link() {
  local install_dir="$1"
  local link_dir="$HOME/.local/bin"
  mkdir -p "$link_dir"
  ln -sf "$install_dir/javboss" "$link_dir/javboss"
  log "command installed: $link_dir/javboss"
  case ":$PATH:" in
    *":$link_dir:"*) ;;
    *) log "add $link_dir to PATH if the javboss command is not found" ;;
  esac
}

start_javboss() {
  local install_dir="$1"
  if [[ "$(uname -s)" == "Darwin" && -f "$install_dir/javboss.command" ]]; then
    if open "$install_dir/javboss.command" >/dev/null 2>&1; then
      return
    fi
  fi
  "$install_dir/javboss"
}

main() {
  command_exists unzip || die "unzip is required"

  local platform tag filename url zip_file extract_dir release_dir
  platform="$(detect_platform)"
  INSTALL_DIR="${INSTALL_DIR:-$(default_install_dir)}"
  ensure_not_running "$INSTALL_DIR"

  tag="$(normalize_tag "$VERSION")"
  filename="javboss-${tag}-${platform}.zip"
  url="https://github.com/${REPO}/releases/download/${tag}/${filename}"

  if [[ "$(installed_version "$INSTALL_DIR")" == "$tag" ]]; then
    log "JavBoss $tag is already installed; no update needed"
    return
  fi

  TMP_DIR="$(mktemp -d)"
  trap cleanup EXIT

  zip_file="$TMP_DIR/$filename"
  extract_dir="$TMP_DIR/extract"
  mkdir -p "$extract_dir"

  log "downloading $url"
  download_file "$url" "$zip_file"

  log "extracting release package"
  unzip -q "$zip_file" -d "$extract_dir"
  release_dir="$(find "$extract_dir" -mindepth 1 -maxdepth 1 -type d -print -quit)"
  [[ -n "$release_dir" && -f "$release_dir/javboss" ]] || die "release package layout is invalid"

  log "installing to $INSTALL_DIR"
  copy_release_files "$release_dir" "$INSTALL_DIR"
  chmod +x "$INSTALL_DIR/javboss" 2>/dev/null || true
  [[ ! -f "$INSTALL_DIR/javboss.command" ]] || chmod +x "$INSTALL_DIR/javboss.command" 2>/dev/null || true

  if command_exists xattr; then
    xattr -dr com.apple.quarantine "$INSTALL_DIR" >/dev/null 2>&1 || true
  fi

  create_command_link "$INSTALL_DIR"
  write_installed_version "$INSTALL_DIR" "$tag"

  log "installed JavBoss $tag"
  if [[ "$START_AFTER_INSTALL" == "1" ]]; then
    log "starting JavBoss"
    start_javboss "$INSTALL_DIR"
  else
    log "start later with: $INSTALL_DIR/javboss"
  fi
}

main "$@"
