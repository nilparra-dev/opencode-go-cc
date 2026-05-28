# Models & Routing

## Available OpenCode Go Models

| Model ID          | Endpoint Type      | Best For                      |
| ----------------- | ------------------ | ----------------------------- |
| `glm-5.1`         | OpenAI-compatible  | Complex reasoning, architecture |
| `glm-5`           | OpenAI-compatible  | General thinking, planning    |
| `kimi-k2.6`       | OpenAI-compatible  | Default, balanced quality/cost |
| `kimi-k2.5`       | OpenAI-compatible  | General tasks                 |
| `mimo-v2.5-pro`   | OpenAI-compatible  | Code generation               |
| `mimo-v2.5`       | OpenAI-compatible  | General tasks                 |
| `mimo-v2-pro`     | OpenAI-compatible  | Code tasks                    |
| `mimo-v2-omni`    | OpenAI-compatible  | Multimodal                    |
| `minimax-m2.7`    | Anthropic-compatible | Ultra-long context (1M)     |
| `minimax-m2.5`    | Anthropic-compatible | Long context                  |
| `deepseek-v4-pro` | OpenAI-compatible  | Deep reasoning with thinking  |
| `deepseek-v4-flash`| OpenAI-compatible | Fast reasoning                |
| `qwen3.6-plus`    | OpenAI-compatible  | Fast responses                |
| `qwen3.5-plus`    | OpenAI-compatible  | Background/cheap tasks        |

## Routing Recommendations

For cost efficiency, the default configuration uses:
- **Qwen3.5 Plus** for background tasks (~10K req/5hr)
- **Kimi K2.6** for default tasks (~1.8K req/5hr)
- **GLM-5.1** only for complex architectural work
- **MiniMax M2.5/M2.7** only when context exceeds 80K tokens

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
