# Configuration

## Config File

Location: `~/.config/occb/config.yaml`

Override with `OCB_CONFIG` environment variable.

## Quick Setup

Run `occb init` to create a default configuration file.

## Environment Variables

Environment variables override config file values.

| Variable          | Description                          | Default                                          |
| ----------------- | ------------------------------------ | ------------------------------------------------ |
| `OCB_API_KEY`     | OpenCode Go API key (**required**)   | —                                                |
| `OCB_CONFIG`      | Custom config file path              | `~/.config/occb/config.yaml`                     |
| `OCB_HOST`        | Proxy listen host                    | `127.0.0.1`                                      |
| `OCB_PORT`        | Proxy listen port                    | `3456`                                           |
| `OCB_LOG_LEVEL`   | Log level: `debug`, `info`, `warn`   | `info`                                           |

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

Routing priority: **Long Context** > **Think** > **Complex** > **Background** > **Default**

## Fallback Chains

When a model request fails, the proxy tries the next model in the fallback chain. Each model also has a circuit breaker that skips it after 3 consecutive failures for 30 seconds.
