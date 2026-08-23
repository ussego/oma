#!/usr/bin/env bash
# Rewrites packaging/aur/PKGBUILD for a release: pkgver + both sha256sums
# from the release's sha256sums.txt. Shared by the release workflow
# (bump-pkgbuild job) and local runs.
#
#   bash scripts/bump-pkgbuild.sh 0.1.6
#
# Exits 0 with "already current" when nothing changed (no commit needed).
set -euo pipefail

VERSION="${1:?usage: bump-pkgbuild.sh <version>}"
TAG="v${VERSION}"
PKGBUILD="packaging/aur/PKGBUILD"
SUM_FILE="$(mktemp)"
trap 'rm -f "$SUM_FILE"' EXIT

curl -fsSL "https://github.com/ussego/oma/releases/download/${TAG}/sha256sums.txt" -o "$SUM_FILE"

amd64="$(awk -v t="oma-${TAG}-linux-amd64.tar.gz" '$2 == t {print $1}' "$SUM_FILE")"
arm64="$(awk -v t="oma-${TAG}-linux-arm64.tar.gz" '$2 == t {print $1}' "$SUM_FILE")"
if [ -z "$amd64" ] || [ -z "$arm64" ]; then
	echo "error: checksums for ${TAG} not found in sha256sums.txt" >&2
	exit 1
fi

sed -i -E \
	-e "s/^pkgver=.*/pkgver=${VERSION}/" \
	-e "s/^sha256sums_x86_64=.*/sha256sums_x86_64=(\"${amd64}\")/" \
	-e "s/^sha256sums_aarch64=.*/sha256sums_aarch64=(\"${arm64}\")/" \
	"$PKGBUILD"

if git diff --quiet "$PKGBUILD"; then
	echo "PKGBUILD already current for ${TAG}"
else
	echo "updated packaging/aur/PKGBUILD for ${TAG}"
fi
