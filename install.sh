#!/bin/sh
# Install the latest ray release binary.
#   curl -fsSL https://raw.githubusercontent.com/HyperMarble/ray/main/install.sh | sh
# Shows progress step by step; falls back to plain lines when not on a terminal.
set -eu

REPO="HyperMarble/ray"
VERSION="${RAY_VERSION:-v0.1.0}"

# Only animate on a real terminal; piped output gets plain lines.
if [ -t 1 ]; then
    FANCY=1; CHECK="✓"; DIM="\033[2m"; BOLD="\033[1m"; GREEN="\033[32m"; RESET="\033[0m"
else
    FANCY=0; CHECK="ok"; DIM=""; BOLD=""; GREEN=""; RESET=""
fi

# step "message" command...: runs the command behind a spinner, then marks it done.
step() {
    message="$1"; shift
    if [ "$FANCY" = 1 ]; then
        "$@" >/dev/null 2>&1 &
        pid=$!
        frames='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
        i=1
        while kill -0 "$pid" 2>/dev/null; do
            frame=$(printf '%s' "$frames" | cut -c $i-$i)
            printf "\r  ${DIM}%s${RESET} %s" "$frame" "$message"
            i=$((i % 10 + 1))
            sleep 0.08
        done
        wait "$pid" || { printf "\r  ✗ %s\n" "$message"; exit 1; }
        printf "\r  ${GREEN}%s${RESET} %s\n" "$CHECK" "$message"
    else
        "$@" >/dev/null 2>&1 || { echo "  failed: $message" >&2; exit 1; }
        echo "  $CHECK $message"
    fi
}

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
    x86_64) arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac
case "$os" in
    darwin | linux) ;;
    *) echo "unsupported OS: $os (build from source: go build ./cmd/ray)" >&2; exit 1 ;;
esac

install_dir="/usr/local/bin"
if [ ! -w "$install_dir" ]; then
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
fi

url="https://github.com/$REPO/releases/download/$VERSION/ray_${VERSION}_${os}_${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

printf "%b\n" "${BOLD}ray installer${RESET} ${DIM}$VERSION · $os/$arch${RESET}"
step "downloading release"      curl -fsSL "$url" -o "$tmp/ray.tar.gz"
step "unpacking"                tar -xzf "$tmp/ray.tar.gz" -C "$tmp"
step "installing to $install_dir" install -m 0755 "$tmp/ray" "$install_dir/ray"
step "verifying"                "$install_dir/ray" --help

printf "%b\n" "\n${GREEN}${CHECK}${RESET} ${BOLD}ray $VERSION is ready${RESET} — try: ${BOLD}ray init my-task${RESET}"
case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) printf "%b\n" "${DIM}note: add $install_dir to your PATH${RESET}" ;;
esac
