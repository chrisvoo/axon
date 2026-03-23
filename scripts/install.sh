#!/usr/bin/env sh
# Install Axon to ~/.local/bin (no sudo). Uses curl or wget.
set -e

REPO="${AXON_REPO:-chrisvoo/axon}"
BASE="https://github.com/${REPO}/releases/latest/download"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

case "$os" in
  linux) name="axon-linux-${arch}.tar.gz" ;;
  darwin) name="axon-darwin-${arch}.tar.gz" ;;
  *)
    echo "use scripts/install.ps1 on Windows" >&2
    exit 1
    ;;
esac

tmp="/tmp/${name}"
url="${BASE}/${name}"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp"
elif command -v wget >/dev/null 2>&1; then
  wget -q "$url" -O "$tmp"
else
  echo "need curl or wget" >&2
  exit 1
fi

mkdir -p "${HOME}/.local/bin"
tar xzf "$tmp" -C "${HOME}/.local/bin"
chmod +x "${HOME}/.local/bin/axon"
rm -f "$tmp"
echo "Installed axon to ${HOME}/.local/bin/axon"
echo "Ensure ${HOME}/.local/bin is on your PATH."
