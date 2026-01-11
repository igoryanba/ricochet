#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
CORE_DIR="$ROOT_DIR/core"
EXT_DIR="$ROOT_DIR/extension-vscode"

echo "🔧 Building Ricochet Core..."

# Build for current platform
cd "$CORE_DIR"

PLATFORMS=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64" "windows/amd64")

for PLATFORM in "${PLATFORMS[@]}"; do
    OS="${PLATFORM%/*}"
    ARCH="${PLATFORM#*/}"
    
    # Map to Node.js naming convention
    NODE_OS=$OS
    NODE_ARCH=$ARCH
    
    if [ "$OS" = "windows" ]; then
        NODE_OS="win32"
    fi
    
    if [ "$ARCH" = "amd64" ]; then
        NODE_ARCH="x64"
    fi
    
    OUTPUT_DIR="$EXT_DIR/bin/${NODE_OS}-${NODE_ARCH}"
    OUTPUT_NAME="ricochet-core"
    
    if [ "$OS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo "  Building for $OS/$ARCH -> $OUTPUT_DIR..."
    mkdir -p "$OUTPUT_DIR"
    
    GOOS=$OS GOARCH=$ARCH go build -ldflags="-s -w" -o "$OUTPUT_DIR/$OUTPUT_NAME" ./cmd/ricochet
done

echo "✅ Core builds complete!"
echo ""
echo "📦 Building Webview..."
cd "$ROOT_DIR/webview"
npm run build

echo "✅ Webview build complete!"
echo ""
echo "🎉 All builds finished. Binaries are in extension-vscode/bin/"
