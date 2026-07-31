#!/bin/bash
set -e

echo "Welcome to Baleen Engine Installer"

# Detect OS to handle Windows (.exe) differences
EXE_EXT=""
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
    EXE_EXT=".exe"
fi

# Check for required dependencies
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go first."
    exit 1
fi

if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker Desktop first."
    exit 1
fi

echo "Building Baleen CLI for your machine..."
mkdir -p build
# Build for the native architecture of the machine running the script
go build -o build/docker-baleen${EXE_EXT} ./cmd/baleen

echo "Installing Docker CLI plugin..."
PLUGIN_DIR="$HOME/.docker/cli-plugins"
mkdir -p "${PLUGIN_DIR}"
cp build/docker-baleen${EXE_EXT} "${PLUGIN_DIR}/docker-baleen${EXE_EXT}"
chmod +x "${PLUGIN_DIR}/docker-baleen${EXE_EXT}" || true

echo "Building Docker Extension image..."
docker build -t baleen-extension:latest .

echo "Installing Docker Extension in Docker Desktop..."
# Check if the extension is already installed to decide between install or update
if docker extension ls | grep -q "baleen-extension"; then
    echo "Extension is already installed. Updating it instead..."
    docker extension update baleen-extension:latest -f
else
    docker extension install baleen-extension:latest -f
fi

echo "Installation complete!"
echo "You can now use 'docker baleen' in your terminal and open the Baleen Extension in Docker Desktop."
