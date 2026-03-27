# Grafana Exporter
![GitHub](https://img.shields.io/github/languages/top/fulgerx2007/grafana-exporter)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/fulgerx2007/grafana-exporter)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/fulgerx2007/grafana-exporter)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/fulgerx2007/grafana-exporter/test.yml)
![Docker Publish](https://img.shields.io/github/actions/workflow/status/fulgerx2007/grafana-exporter/docker-publish.yml?label=docker)
![GitHub](https://img.shields.io/github/license/fulgerx2007/grafana-exporter)
![GitHub](https://img.shields.io/github/downloads/fulgerx2007/grafana-exporter/total)
![GitHub](https://img.shields.io/github/stars/fulgerx2007/grafana-exporter?style=social)
![GitHub](https://img.shields.io/github/forks/fulgerx2007/grafana-exporter?style=social)
![GitHub](https://img.shields.io/github/watchers/fulgerx2007/grafana-exporter?style=social)

![logo](images/logo.png)

A Go application with a web UI to export Grafana dashboards and their linked libraries.

## Features

- Web-based interface for selecting dashboards to export
- **Multi-organization support** — switch between Grafana organizations from the UI
- **Dual authentication** — Bearer token (API key) or Basic auth (username/password)
- Automatically exports linked library panels
- Preserves folder structure for dashboards and libraries
- Supports nested folder hierarchies (clicking a folder shows all sub-folder dashboards)
- Sort dashboards by name or last updated date with version tracking
- Export as individual JSON files or ZIP archive
- Alert rules export (modern and legacy Grafana API)
- Search and filter capabilities
- Works with Grafana v10.x / v11.x

## Requirements

- Go 1.22 or higher
- Access to a Grafana instance
- Grafana API key with viewer permissions, **or** username/password with basic auth enabled

## Installation

### From Packages

Download the appropriate package for your system from the [releases page](https://github.com/fulgerx2007/grafana-exporter/releases):

- `.deb` package for Debian/Ubuntu
- `.rpm` package for RHEL/CentOS/Fedora
- `.exe` for Windows
- Binary files for other Linux distributions

### From Source

```bash
git clone https://github.com/yourusername/grafana-exporter.git
cd grafana-exporter
go mod download
go build
```

## Configuration

Create a `.env` file based on `.env.example`:

```bash
# Grafana connection
GRAFANA_URL=https://your-grafana-instance

# Authentication (choose one method):
# Option 1: API key (Bearer token) — single org
GRAFANA_API_KEY=your-api-key-here

# Option 2: Basic auth (username/password) — supports multi-org switching
GRAFANA_USER=admin
GRAFANA_PASSWORD=your-password

# General settings
SKIP_TLS_VERIFY=true
GRAFANA_VERSION=11.1
EXPORT_DIRECTORY=./exported
SERVER_HOST=127.0.0.1
SERVER_PORT=8080
FORCE_ENABLE_ZIP_EXPORT=false
```

### Authentication Methods

| Method | Env Vars | Multi-Org | Notes |
|--------|----------|-----------|-------|
| **API Key** | `GRAFANA_API_KEY` | No | Default. Create via Grafana > Administration > Service accounts |
| **Basic Auth** | `GRAFANA_USER` + `GRAFANA_PASSWORD` | Yes | Auto-detected when both are set. Requires `[auth.basic] enabled = true` in Grafana config |

When using basic auth, the organization selector appears in the UI allowing you to switch between orgs.

## Usage

1. Start the application:

```bash
./grafana-exporter
```

2. Open your web browser and navigate to:

```
http://localhost:8080
```

3. Use the web interface to:
   - Switch between organizations (if using basic auth)
   - Browse folders and dashboards (clicking a folder includes subfolders)
   - Sort by name or last updated date
   - Select dashboards and alert rules to export
   - Export as JSON files or ZIP archive with linked library panels

## Creating a Grafana API Key

1. In your Grafana instance, go to:
   - Configuration > API Keys (Grafana < v9)
   - Administration > API Keys (Grafana v9+)
   - Administration > Service accounts (Grafana v10+)

2. Add a new API key with `Viewer` role permission

3. Copy the generated API key to your `.env` file

## Export Format

Exported files maintain the original folder structure:

```
exported/
  ├── 20240228_123045/          # Timestamp of export
  │   ├── dashboards/
  │   │   ├── General/          # General folder
  │   │   │   └── Dashboard1.json
  │   │   └── FolderName/       # Named folders
  │   │       └── Dashboard2.json
  │   └── libraries/
  │       ├── General/          
  │       │   └── Panel1.json
  │       └── FolderName/       
  │           └── Panel2.json
```

## Docker Support

### Building Locally

```bash
docker build -t grafana-exporter .
docker run -p 8080:8080 --env-file .env -v $(pwd)/exported:/app/exported grafana-exporter
```

### Using GitHub Container Registry

You can pull the pre-built image from GitHub Container Registry:

```bash
docker pull ghcr.io/fulgerx2007/grafana-exporter:latest
docker run -p 8080:8080 --env-file .env -v $(pwd)/exported:/app/exported ghcr.io/fulgerx2007/grafana-exporter:latest
```

Or use a specific version:

```bash
docker pull ghcr.io/fulgerx2007/grafana-exporter:v1.0.0
docker run -p 8080:8080 --env-file .env -v $(pwd)/exported:/app/exported ghcr.io/fulgerx2007/grafana-exporter:v1.0.0
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
