# OpenCode Claude Bridge (occb)

A transparent proxy that lets you use your [OpenCode Go](https://opencode.ai/) subscription with [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

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

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/nilparra-dev/opencode-go-cc/main/scripts/install.sh | bash

# Setup (asks for your OpenCode Go API key)
occb init

# Use OpenCode models with Claude Code
occb on
claude

# Go back to your normal Claude subscription
occb off
```

## Requirements

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) installed
- An active [OpenCode Go](https://opencode.ai/) subscription and API key

## Documentation

- [Installation](docs/INSTALLATION.md)
- [Configuration](docs/CONFIGURATION.md)
- [Models & Routing](docs/MODELS.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

## License

[MIT](LICENSE)
