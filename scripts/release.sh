#!/usr/bin/env bash
# Tag and push a release. The tag-push workflow then builds both arches,
# publishes tarballs + checksums + @oma/runtime, and auto-bumps
# packaging/aur/PKGBUILD.
#
#   mise run release 0.1.6
set -euo pipefail

VERSION="${1:-}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "usage: mise run release <X.Y.Z>" >&2
	exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
	echo "error: working tree not clean" >&2
	exit 1
fi

git fetch origin -q
if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
	echo "error: local main is not up to date with origin/main" >&2
	exit 1
fi

TAG="v${VERSION}"
git tag -a "$TAG" -m "$TAG"
git push origin "$TAG"
echo "pushed ${TAG} - the release workflow is building, publishing, and bumping the AUR PKGBUILD"
