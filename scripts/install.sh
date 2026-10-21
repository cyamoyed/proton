#!/bin/sh
# Proton CLI 一键安装脚本
#
# 用法:
#   sh install.sh                      自动选择安装源: 优先使用本目录下与本机匹配的二进制,
#                                      否则从 GitHub Release 下载最新版本
#   sh install.sh <binary-file>        安装指定的二进制文件, 如: sh install.sh ~/Downloads/proton-darwin-arm64
#   PROTON_VERSION=v1.2.3 sh install.sh
#                                      从 GitHub Release 安装指定版本 (仅下载模式)
#
# 从网络安装:
#   curl -fsSL https://github.com/cyamoyed/proton/releases/latest/download/install.sh | bash

set -eu

REPO="${PROTON_REPO:-cyamoyed/proton}"
INSTALL_DIR="${PROTON_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '==> %s\n' "$1"; }
die() { printf '错误: %s\n' "$1" >&2; exit 1; }

# ---------------------------------------------------------------- 平台识别
case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "不支持的操作系统: $(uname -s)。Windows 请在 Release 页面下载并双击 proton-installer-windows-*.exe" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "不支持的 CPU 架构: $(uname -m)" ;;
esac

asset="proton-${os}-${arch}"
say "检测到平台: ${os}/${arch}"

# ---------------------------------------------------------------- 下载工具
fetch() {
  # fetch <url> <output-file>
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    die "需要 curl 或 wget 以下载文件"
  fi
}

checksum_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

# ---------------------------------------------------------------- 定位二进制
explicit="${1:-}"
if [ -n "$explicit" ]; then
  [ -f "$explicit" ] || die "文件不存在: $explicit"
  src="$explicit"
  say "使用指定文件: $src"
else
  # 脚本所在目录 (curl | bash 时 $0 不是文件路径, 跳过本地查找)
  src=""
  if [ -f "$0" ]; then
    dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
    if [ -f "$dir/$asset" ]; then
      src="$dir/$asset"
      say "在脚本目录发现本机平台的二进制: $src"
    fi
  fi
  if [ -z "$src" ]; then
    version="${PROTON_VERSION:-latest}"
    if [ "$version" = "latest" ]; then
      url="https://github.com/${REPO}/releases/latest/download/${asset}"
      sums_url="https://github.com/${REPO}/releases/latest/download/checksums.txt"
    else
      url="https://github.com/${REPO}/releases/download/${version}/${asset}"
      sums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
    fi
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp" 2>/dev/null || true' EXIT
    say "下载 $url"
    fetch "$url" "$tmp/proton" || die "下载失败, 请检查网络或版本号 (${version})。也可手动下载 ${asset} 后运行: sh install.sh <文件路径>"
    if fetch "$sums_url" "$tmp/checksums.txt" 2>/dev/null; then
      expected=$(awk -v f="$asset" '$2==f || $2=="*"f {print $1}' "$tmp/checksums.txt")
    else
      expected=""
    fi
    if [ -n "$expected" ]; then
      actual=$(checksum_sha256 "$tmp/proton")
      [ "$actual" = "$expected" ] || die "SHA-256 校验失败, 文件可能已被篡改"
      say "SHA-256 校验通过"
    else
      say "警告: 未找到 checksums.txt, 已跳过完整性校验"
    fi
    src="$tmp/proton"
  fi
fi

# ---------------------------------------------------------------- 安装
mkdir -p "$INSTALL_DIR"
mv "$src" "$INSTALL_DIR/proton"
chmod +x "$INSTALL_DIR/proton"
say "已安装到 $INSTALL_DIR/proton"

# ---------------------------------------------------------------- 配置 PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
      case "$SHELL" in
        */zsh) rc="$HOME/.zshrc" ;;
        */bash) rc="$HOME/.bashrc" ;;
        *) rc="$HOME/.profile" ;;
      esac
      line='export PATH="$HOME/.local/bin:$PATH"'
      if [ -f "$rc" ] && grep -qF 'HOME/.local/bin' "$rc"; then
        say "$rc 中已有 ~/.local/bin 的 PATH 配置, 未重复写入"
      else
        printf '\n# Proton CLI\n%s\n' "$line" >> "$rc"
        say "已将 $INSTALL_DIR 写入 $rc"
      fi
    else
      say "自定义安装目录 $INSTALL_DIR 未写入 shell 配置, 请自行将其加入 PATH"
    fi
    ;;
esac

say "安装完成。"
if ! "$INSTALL_DIR/proton" --version; then
  say "警告: 安装后无法运行 proton, 二进制可能与本机平台不匹配 (${os}/${arch})"
fi
case ":$PATH:" in
  *":$INSTALL_DIR:"*) say "现在可以直接使用 proton 命令。" ;;
  *) say "请重新打开终端, 或执行: export PATH=\"$INSTALL_DIR:\$PATH\", 然后使用 proton 命令。" ;;
esac
