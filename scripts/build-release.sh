#!/usr/bin/env bash

set -Eeuo pipefail

DIST="dist"
PROGRAM="http-tester"

VERSION="${GITHUB_REF_NAME:-$(git describe --tags --always 2>/dev/null || echo dev)}"

PLATFORMS=(
    "linux amd64"
    "linux arm64"
    "windows amd64"
    "darwin amd64"
    "darwin arm64"
)

echo "==> Running tests..."
go test ./...

echo "==> Cleaning..."
rm -rf "$DIST"
mkdir -p "$DIST"

for platform in "${PLATFORMS[@]}"; do
    read -r GOOS GOARCH <<< "$platform"

    WORKDIR="$(mktemp -d)"

    EXT=""
    ARCHIVE_EXT="tar.gz"

    if [[ "$GOOS" == "windows" ]]; then
        EXT=".exe"
        ARCHIVE_EXT="zip"
    fi

    ARCHIVE_NAME="${PROGRAM}-${VERSION}-${GOOS}-${GOARCH}.${ARCHIVE_EXT}"

    echo "==> Building ${GOOS}/${GOARCH}"

    GOOS="$GOOS" GOARCH="$GOARCH" \
        go build \
        -trimpath \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o "${WORKDIR}/${PROGRAM}${EXT}" .

    cp README.md "${WORKDIR}/"

    if [[ -f LICENSE ]]; then
        cp LICENSE "${WORKDIR}/"
    fi

    pushd "$WORKDIR" >/dev/null

    if [[ "$ARCHIVE_EXT" == "zip" ]]; then
        zip -q -r "${OLDPWD}/${DIST}/${ARCHIVE_NAME}" .
    else
        tar -czf "${OLDPWD}/${DIST}/${ARCHIVE_NAME}" .
    fi

    popd >/dev/null

    rm -rf "$WORKDIR"
done

echo
echo "Release artifacts:"
ls -lh "$DIST"