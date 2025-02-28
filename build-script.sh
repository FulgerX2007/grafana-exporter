#!/bin/bash
# Script to build the DNA Dashboard Exporter for multiple platforms

# Create a build directory
mkdir -p build

# Function to build for a specific OS and architecture
build() {
    local os=$1
    local arch=$2
    local extension=$3
    local output="build/dashboard-exporter-${os}-${arch}${extension}"
    
    echo "Building for ${os}/${arch}..."
    GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o "$output" .
    
    if [ $? -eq 0 ]; then
        echo "✅ Successfully built $output"
    else
        echo "❌ Failed to build for ${os}/${arch}"
    fi
}

# Build for Windows (amd64)
build windows amd64 .exe

# Build for Linux (amd64)
build linux amd64 ""

# Build for macOS (amd64)
build darwin amd64 ""

# Build for Linux (arm64) - for Raspberry Pi and other ARM devices
build linux arm64 ""

# Create .env.example in the build directory
cat > build/.env.example << EOF
# Grafana/DNA connection settings
GRAFANA_URL=http://localhost:3000
GRAFANA_API_KEY=your-api-key-here
SKIP_TLS_VERIFY=false
GRAFANA_VERSION=11.1

# Application settings
EXPORT_DIRECTORY=./exported
SERVER_PORT=8080
EOF

# Copy README to build directory
cp README.md build/

echo ""
echo "All builds completed! Files are in the 'build' directory."
echo "Remember to create a .env file based on .env.example before running the application."
