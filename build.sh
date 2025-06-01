#!/bin/bash

# Build script for yutemal

echo "Building yutemal..."

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.23"

if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

echo "Go version: $GO_VERSION"

# Set environment variable for proper character width calculation
export RUNEWIDTH_EASTASIAN=0

# Change to the go directory
cd "$(dirname "$0")"

# Download dependencies
echo "Downloading dependencies..."
go mod download

# Get version information
VERSION=${VERSION:-dev}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Build the binary with version information
echo "Building binary (version: $VERSION)..."
go build -ldflags "-X github.com/haryoiro/yutemal/internal/version.Version=$VERSION \
    -X github.com/haryoiro/yutemal/internal/version.Commit=$COMMIT \
    -X github.com/haryoiro/yutemal/internal/version.Date=$DATE" \
    -o yutemal cmd/yutemal/main.go

if [ $? -eq 0 ]; then
    echo "Build successful! Binary created: ./yutemal"
    echo ""
    echo "To run: ./yutemal"
    echo "For help: ./yutemal --help"
else
    echo "Build failed!"
    exit 1
fi
