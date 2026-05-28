# Models & Routing

## Available OpenCode Go Models (Updated May 2026)

| Model ID           | Endpoint Type        | Best For                      | Cost/1M input |
| ------------------ | -------------------- | ----------------------------- | ------------- |
| `qwen3.5-plus`     | Anthropic-compatible | Background tasks (cheapest)   | $0.20         |
| `qwen3.6-plus`     | Anthropic-compatible | Fast responses                | $0.50         |
| `qwen3.7-max`      | Anthropic-compatible | Maximum quality Qwen          | $2.50         |
| `minimax-m2.5`     | Anthropic-compatible | Long context (1M tokens)      | $0.30         |
| `minimax-m2.7`     | Anthropic-compatible | Ultra-long context (1M)       | $0.30         |
| `glm-5`            | OpenAI-compatible    | General thinking, planning    | $1.00         |
| `glm-5.1`          | OpenAI-compatible    | Complex reasoning (best)      | $1.40         |
| `kimi-k2.5`        | OpenAI-compatible    | Balanced quality/cost         | $0.60         |
| `kimi-k2.6`        | OpenAI-compatible    | Default, good quality         | $0.95         |
| `deepseek-v4-pro`  | OpenAI-compatible    | Deep reasoning with thinking  | $3.45         |
| `deepseek-v4-flash`| OpenAI-compatible    | Fast reasoning (cheap)        | low cost      |
| `mimo-v2.5`        | OpenAI-compatible    | Very cheap general tasks      | low cost      |
| `mimo-v2.5-pro`    | OpenAI-compatible    | Code generation               | $3.25         |

**Note:** Models marked `Anthropic-compatible` use the `/v1/messages` endpoint.  
Models marked `OpenAI-compatible` use the `/v1/chat/completions` endpoint.

## Routing Recommendations

For cost efficiency with OpenCode Go ($5 first month, then $10/month):

| Scenario       | Default Model   | Approx. Requests/5hr |
| -------------- | --------------- | -------------------- |
| **Background** | qwen3.5-plus    | ~10,200              |
| **Default**    | kimi-k2.6       | ~1,150               |
| **Think**      | glm-5           | ~1,150               |
| **Complex**    | glm-5.1         | ~880                 |
| **Long Context**| minimax-m2.5   | ~6,300               |
| **Fast**       | qwen3.6-plus    | ~3,300               |

## Customizing Routes

Edit `~/.config/occb/config.yaml` to change models for each scenario:

```yaml
models:
  default:
    provider: "opencode-go"
    model_id: "your-preferred-model"
    temperature: 0.7
    max_tokens: 4096
```

## Adding New Models

OpenCode Go may add new models over time. You can use any model ID in your config without waiting for an occb update — just set the `model_id` to the new model name. If the model uses the Anthropic endpoint (like MiniMax or Qwen), occb will route it correctly based on the model name pattern.
