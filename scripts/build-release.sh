#!/bin/bash
set -e

VERSION=${1:-dev}
DIST="dist"

echo "Building release $VERSION..."

# Clean
rm -rf "$DIST"
mkdir -p "$DIST"

# Linux amd64
echo "Building Linux amd64..."
mkdir -p "$DIST/linux-amd64"
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$DIST/linux-amd64/http-tester" .
cp README.md "$DIST/linux-amd64/"
cd "$DIST/linux-amd64" && tar -czf "../http-tester-${VERSION}-linux-amd64.tar.gz" * && cd ../..
rm -rf "$DIST/linux-amd64"

# Windows amd64
echo "Building Windows amd64..."
mkdir -p "$DIST/windows-amd64"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$DIST/windows-amd64/http-tester.exe" .
cp README.md "$DIST/windows-amd64/"
cd "$DIST/windows-amd64" && tar -czf "../http-tester-${VERSION}-windows-amd64.tar.gz" * && cd ../..
rm -rf "$DIST/windows-amd64"

echo "Done! Archives:"
ls -lh "$DIST"/*.tar.gz
