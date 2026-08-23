#!/usr/bin/env bash
# oma installer - fetches the latest release from GitHub.
#
#   curl -fsSL https://raw.githubusercontent.com/ussego/oma/main/install.sh | bash
#
# Installs to ${PREFIX:-$HOME/.local/bin}/oma.
set -euo pipefail

REPO="ussego/oma"
PREFIX="${PREFIX:-$HOME/.local/bin}"

case "$(uname -m)" in
x86_64) ARCH="amd64" ;;
aarch64 | arm64) ARCH="arm64" ;;
*) echo "error: unsupported arch $(uname -m)" >&2; exit 1 ;;
esac

# Resolve the latest release tag via the API redirect (no jq needed).
# `|| true` lets the friendly error below fire instead of set -e killing us.
TAG=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
	"https://github.com/${REPO}/releases/latest" |
	sed 's|.*/tag/||') || true
[ -n "$TAG" ] || { echo "error: could not resolve latest release" >&2; exit 1; }

TARBALL="oma-${TAG}-linux-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${TARBALL}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "downloading ${TARBALL}..."
curl -fsSL -o "${TMP}/${TARBALL}" "$URL"

# Verify against the published checksums.
curl -fsSL -o "${TMP}/sha256sums.txt" \
	"https://github.com/${REPO}/releases/download/${TAG}/sha256sums.txt"
(cd "$TMP" && grep " ${TARBALL}\$" sha256sums.txt | sha256sum -c -) ||
	{ echo "error: checksum mismatch for ${TARBALL}" >&2; exit 1; }

tar -xzf "${TMP}/${TARBALL}" -C "$TMP"

mkdir -p "$PREFIX"
install -m 0755 "${TMP}/oma-${TAG}-linux-${ARCH}/oma" "${PREFIX}/oma"

echo "installed: ${PREFIX}/oma ($("$PREFIX/oma" --version))"
case ":$PATH:" in
*":$PREFIX:"*) ;;
*) echo "note: $PREFIX is not on your PATH" ;;
esac
