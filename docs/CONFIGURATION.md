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
| **Fast**       | Streaming requests, only if `enable_streaming_scenario_routing: false` | `qwen3.6-plus` |

Keywords are matched as whole words in the user's most recent request (not the whole history or tool output).

Routing priority: **Long Context** > **Think** > **Complex** > **Background** > **Default**

## Claude and OpenCode side by side

By default `occb on` is **mixed mode**: requests for `claude-*` models are forwarded
untouched to Anthropic using your own claude.ai login or API key, and the OpenCode Go
models are added to Claude Code's `/model` picker (through
`CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY`, served from the proxy's `/v1/models`).
Your Claude login and model settings are not modified.

```yaml
anthropic:
  passthrough: true                    # set to false to never contact Anthropic
  base_url: "https://api.anthropic.com"
```

`occb on --exclusive` pins every Claude tier to an OpenCode model and uses a dummy
credential, so nothing reaches Anthropic.

When you switch from an OpenCode model back to a Claude model in the same
conversation, thinking blocks written by the OpenCode model (they carry no
signature) are removed from the request, and that request's `thinking` setting is
dropped, because Anthropic rejects unsigned thinking blocks.

## Respecting Explicit Model Choices

`respect_requested_model` (default `true`) controls whether the model chosen in
`/model` or `--model` is sent upstream as-is. Claude's own request limits
(`max_tokens`, `temperature`) are kept in that case.

- `true` forwards the exact OpenCode model Claude requested.
- `false` lets the scenario router pick the model instead.

`enable_streaming_scenario_routing` (default `true`) must stay on for scenario
routing to work: Claude Code always streams, and with it off every request would
go to the `fast` model.

## Fallback Chains

When a model request fails, the proxy tries the next model in the fallback chain. Each model also has a circuit breaker that skips it after 3 consecutive failures for 30 seconds.
