# OpenCode Claude Bridge (occb)

A transparent proxy that lets you use your [OpenCode Go](https://opencode.ai/docs/go/) subscription with [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

`occb` sits between Claude Code and OpenCode Go, intercepting Anthropic API requests, transforming them to OpenAI format, and forwarding them to OpenCode Go's endpoint. Switch between your normal Claude subscription and OpenCode models with two simple commands.

## Features

- **Two-Command Toggle** — `occb on` to use OpenCode, `occb off` to go back to Anthropic
- **No Shell Hacks** — Uses Claude Code's native `settings.json` instead of environment variables or aliases
- **Transparent Proxy** — Full Anthropic ↔ OpenAI format conversion (requests, responses, and streaming)
- **Smart Model Routing** — Automatically picks the best OpenCode model for the task
- **Fallback Chains** — If a model fails, automatically tries the next one
- **Circuit Breaker** — Skips unhealthy models to avoid latency spikes
- **Real-time Streaming** — Full SSE streaming with live format transformation
- **Tool Calling** — Complete Anthropic tool_use/tool_result ↔ OpenAI function calling translation
- **Single Binary** — One cross-platform binary, no dependencies

## Quick Start

### Windows

```powershell
# 1. Clone the repository
git clone https://github.com/nilparra-dev/opencode-go-cc.git
cd opencode-go-cc

# 2. Run the installer (requires Go installed)
.\scripts\install-windows.ps1

# 3. Close and reopen your terminal, then:
occb init

# 4. Add your OpenCode Go API key to the config
notepad $env:USERPROFILE\.config\occb\config.yaml

# 5. Activate OpenCode mode
occb on

# 6. Launch Claude Code
claude

# 7. To go back to Anthropic
occb off
```

### macOS / Linux

```bash
# Install (requires Go 1.23+)
git clone https://github.com/nilparra-dev/opencode-go-cc.git
cd opencode-go-cc
make build
make install

# Setup
occb init

# Add your OpenCode Go API key:
# Edit ~/.config/occb/config.yaml or set OCB_API_KEY env var

# Use
occb on
claude
occb off
```

## Requirements

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) installed
- An active [OpenCode Go](https://opencode.ai/docs/go/) subscription and API key
- Go 1.23+ (for building from source)

## How It Works

```
┌─────────────┐     Anthropic API      ┌─────────────┐     OpenAI API       ┌─────────────┐
│  Claude Code ├──────────────────────►│    occb      ├────────────────────►│  OpenCode Go │
│  (CLI)       │  POST /v1/messages   │  (Proxy)     │  /chat/completions  │  (Upstream)  │
│              │◄──────────────────────┤              │◄────────────────────┤              │
└─────────────┘   Anthropic SSE        └─────────────┘   OpenAI SSE          └─────────────┘
```

1. `occb on` starts a local proxy server and configures Claude Code's `settings.json` to route through it
2. Claude Code sends requests in Anthropic format to `http://127.0.0.1:3456`
3. `occb` transforms the request to OpenAI format and forwards to OpenCode Go
4. The response is transformed back to Anthropic format and returned to Claude Code
5. `occb off` stops the proxy and restores Claude Code's original configuration

## CLI Commands

```
occb init          # Create default configuration file
occb on            # Activate OpenCode mode (start proxy + configure Claude)
occb off           # Deactivate OpenCode mode (stop proxy + restore Claude)
occb status        # Show current status
occb serve         # Start proxy server (foreground)
occb stop          # Stop proxy server
occb run           # Run Claude Code with temporary proxy
occb validate      # Validate configuration
occb models        # List available OpenCode Go models
```

## Model Routing

The proxy automatically detects the type of request and routes to the appropriate model:

| Scenario       | Trigger                                              | Default Model  |
| -------------- | ---------------------------------------------------- | -------------- |
| **Default**    | Standard chat                                        | `kimi-k2.6`    |
| **Think**      | "think", "plan", "reason" in prompt                  | `glm-5`        |
| **Complex**    | "architect", "refactor", "complex" in prompt         | `glm-5.1`      |
| **Long Context** | >80K tokens                                        | `minimax-m2.5` |
| **Background** | Read/list operations                                 | `qwen3.5-plus` |
| **Fast**       | Streaming requests (unless scenario routing enabled) | `qwen3.6-plus` |

## Configuration

Config file: `~/.config/occb/config.yaml`

Override with `OCB_CONFIG` environment variable.

## Documentation

- [Installation](docs/INSTALLATION.md)
- [Configuration](docs/CONFIGURATION.md)
- [Models & Routing](docs/MODELS.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

## License

[MIT](LICENSE)
