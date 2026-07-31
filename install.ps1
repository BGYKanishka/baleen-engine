$ErrorActionPreference = "Stop"

Write-Host "Welcome to Baleen Engine Installer" -ForegroundColor Cyan

# Check for required dependencies
if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed. Please install Go first." -ForegroundColor Red
    exit 1
}

if (-not (Get-Command "docker" -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Docker is not installed. Please install Docker Desktop first." -ForegroundColor Red
    exit 1
}

Write-Host "Building Baleen CLI for your machine..."
if (-not (Test-Path "build")) {
    New-Item -ItemType Directory -Path "build" | Out-Null
}
# Build the executable for Windows
go build -o build/docker-baleen.exe ./cmd/baleen

Write-Host "Installing Docker CLI plugin..."
$PluginDir = "$HOME\.docker\cli-plugins"
if (-not (Test-Path $PluginDir)) {
    New-Item -ItemType Directory -Path $PluginDir -Force | Out-Null
}
Copy-Item "build\docker-baleen.exe" -Destination "$PluginDir\docker-baleen.exe" -Force

Write-Host "Building Docker Extension image..."
docker build -t baleen-extension:latest .

Write-Host "Installing Docker Extension in Docker Desktop..."
# Check if the extension is already installed to decide between install or update
$extensionExists = docker extension ls | Select-String "baleen-extension" -Quiet
if ($extensionExists) {
    Write-Host "Extension is already installed. Updating it instead..."
    docker extension update baleen-extension:latest -f
} else {
    docker extension install baleen-extension:latest -f
}

Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "You can now use 'docker baleen' in your terminal and open the Baleen Extension in Docker Desktop." -ForegroundColor Green
