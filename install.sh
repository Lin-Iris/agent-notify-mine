#!/usr/bin/env bash
# install.sh — 一键安装 agent-notify（自动检测平台，从 GitHub Releases 下载）
set -euo pipefail

OWNER="Lin-Iris"
REPO="agent-notify-mine"
VERSION="${AGENT_NOTIFY_VERSION:-v0.10.2}"
INSTALL_DIR="${AGENT_NOTIFY_INSTALL_DIR:-$HOME/.local/bin}"
BIN_NAME="agent-notify"

# ── 颜色 ──────────────────────────────────────────────
BOLD="\033[1m"
GREEN="\033[32m"
YELLOW="\033[33m"
RED="\033[31m"
RESET="\033[0m"

banner() {
    echo ""
    echo -e "${BOLD}╔══════════════════════════════════════╗${RESET}"
    echo -e "${BOLD}║   agent-notify 一键安装              ║${RESET}"
    echo -e "${BOLD}╚══════════════════════════════════════╝${RESET}"
    echo ""
}

info()  { echo -e "${GREEN}✅${RESET} $1"; }
warn()  { echo -e "${YELLOW}⚠️${RESET}  $1"; }
err()   { echo -e "${RED}❌${RESET} $1"; }

# ── 平台检测 ──────────────────────────────────────────
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Darwin)  os="darwin" ;;
        Linux)   os="linux" ;;
        MINGW*|MSYS*|CYGWIN*) os="windows" ;;
        *)
            err "不支持的操作系统: $(uname -s)"
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)
            err "不支持的架构: $(uname -m)"
            exit 1
            ;;
    esac

    echo "${os}/${arch}"
}

# ── 下载安装 ──────────────────────────────────────────
install_binary() {
    local platform="$1"
    local os="${platform%/*}"
    local arch="${platform#*/}"
    local ext=""

    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    local asset="agent-notify-${VERSION}-${os}-${arch}.tar.gz"
    local binary_in_archive="agent-notify-${VERSION}-${os}-${arch}${ext}"
    local url="https://github.com/${OWNER}/${REPO}/releases/download/${VERSION}/${asset}"

    echo "  平台: ${os}/${arch}"
    echo "  版本: ${VERSION}"
    echo "  下载: ${url}"
    echo ""

    # 创建临时目录
    local tmpdir
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    # 下载
    echo "  下载中..."
    if ! curl -fsSL --progress-bar -o "${tmpdir}/${asset}" "$url"; then
        err "下载失败: ${url}"
        echo "  请确认版本 ${VERSION} 的 Release 已发布："
        echo "  https://github.com/${OWNER}/${REPO}/releases"
        exit 1
    fi
    info "下载完成"

    # 解压
    echo "  解压中..."
    if ! tar -xzf "${tmpdir}/${asset}" -C "${tmpdir}"; then
        err "解压失败，文件可能已损坏"
        exit 1
    fi

    # 安装
    mkdir -p "$INSTALL_DIR"
    mv "${tmpdir}/${binary_in_archive}" "${INSTALL_DIR}/${BIN_NAME}"
    chmod +x "${INSTALL_DIR}/${BIN_NAME}"
    info "已安装到 ${INSTALL_DIR}/${BIN_NAME}"
}

# ── PATH 检查 ──────────────────────────────────────────
ensure_path() {
    if command -v "$BIN_NAME" &>/dev/null; then
        return 0
    fi

    warn "${INSTALL_DIR} 不在 PATH 中"

    # 检测 shell
    local shell_rc
    case "$(basename "$SHELL")" in
        zsh)  shell_rc="$HOME/.zshrc" ;;
        bash) shell_rc="$HOME/.bashrc" ;;
        fish) shell_rc="$HOME/.config/fish/config.fish" ;;
        *)    shell_rc="$HOME/.profile" ;;
    esac

    cat >> "$shell_rc" <<EOF

# agent-notify
export PATH="${INSTALL_DIR}:\$PATH"
EOF

    info "已将 ${INSTALL_DIR} 加入 PATH（写入 ${shell_rc}）"
    echo ""
    echo -e "  ${BOLD}请运行以下命令使 PATH 生效，或重新打开终端：${RESET}"
    echo -e "  ${YELLOW}source ${shell_rc}${RESET}"
    echo ""
}

# ── 验证 ──────────────────────────────────────────────
verify() {
    echo ""
    echo "  验证安装..."
    "${INSTALL_DIR}/${BIN_NAME}" version
    echo ""
}

# ── 入口 ──────────────────────────────────────────────
main() {
    banner

    # 依赖检查
    if ! command -v curl &>/dev/null; then
        err "需要 curl，请先安装"
        exit 1
    fi

    local platform
    platform="$(detect_platform)"

    install_binary "$platform"
    ensure_path
    verify

    echo "══════════════════════════════════════"
    echo "  安装完成！"
    echo ""
    echo "  下一步:"
    echo "  agent-notify init     # 初始化配置"
    echo "  agent-notify test     # 测试通知"
    echo "══════════════════════════════════════"
    echo ""
}

main "$@"
