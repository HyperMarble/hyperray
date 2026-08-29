#!/bin/sh
# Install the latest ray release binary.
#   curl -fsSL https://raw.githubusercontent.com/HyperMarble/ray/main/install.sh | sh
# Picks the right build for this machine and puts it on your PATH.
set -eu

REPO="HyperMarble/ray"
VERSION="${RAY_VERSION:-v0.1.0}"

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

# Prefer /usr/local/bin when writable, otherwise ~/.local/bin.
install_dir="/usr/local/bin"
if [ ! -w "$install_dir" ]; then
    install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"
fi

url="https://github.com/$REPO/releases/download/$VERSION/ray_${VERSION}_${os}_${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "downloading $url"
curl -fsSL "$url" -o "$tmp/ray.tar.gz"
tar -xzf "$tmp/ray.tar.gz" -C "$tmp"
install -m 0755 "$tmp/ray" "$install_dir/ray"

echo "installed: $install_dir/ray"
"$install_dir/ray" --help >/dev/null 2>&1 && echo "ray $VERSION is ready" || true
case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) echo "note: add $install_dir to your PATH" ;;
esac
