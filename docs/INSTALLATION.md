# Installation

## Quick Install

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/nilparra-dev/opencode-go-cc/main/scripts/install.sh | bash
```

### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/nilparra-dev/opencode-go-cc/main/scripts/install.ps1 | iex
```

After installation, update to the latest release anytime with:

```bash
occb update
```

## Manual Installation

### From Source

Requires Go 1.23 or later.

```bash
git clone https://github.com/nilparra-dev/opencode-go-cc.git
cd opencode-go-cc
make build
make install

# Later, install the newest published release
occb update
```

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/nilparra-dev/opencode-go-cc/releases).

Available platforms:
- macOS (Intel & Apple Silicon)
- Linux (x86_64 & ARM64)
- Windows (x86_64 & ARM64)

## Requirements

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) installed and working
- An active [OpenCode Go](https://opencode.ai/) subscription with an API key
