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
    echo "Go is not installed. Attempting to install Go..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if command -v brew &> /dev/null; then
            brew install go
        else
            echo "Error: Homebrew is not installed. Please install Homebrew or Go manually."
            exit 1
        fi
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y golang
        elif command -v dnf &> /dev/null; then
            sudo dnf install -y golang
        elif command -v yum &> /dev/null; then
            sudo yum install -y golang
        elif command -v pacman &> /dev/null; then
            sudo pacman -S --noconfirm go
        else
            echo "Error: Could not find a suitable package manager. Please install Go manually."
            exit 1
        fi
    else
        echo "Error: Automatic Go installation is not supported on this OS ($OSTYPE). Please install Go manually."
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        echo "Error: Go installation finished, but 'go' command is still not found. Please check your PATH or install manually."
        exit 1
    fi
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

echo "Cross-compiling binaries for Docker Extension..."
GOOS=darwin GOARCH=amd64 go build -o build/baleen-darwin-amd64 ./cmd/baleen
GOOS=darwin GOARCH=arm64 go build -o build/baleen-darwin-arm64 ./cmd/baleen
GOOS=linux GOARCH=amd64 go build -o build/baleen-linux-amd64 ./cmd/baleen
GOOS=linux GOARCH=arm64 go build -o build/baleen-linux-arm64 ./cmd/baleen
GOOS=windows GOARCH=amd64 go build -o build/baleen-windows-amd64.exe ./cmd/baleen

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
