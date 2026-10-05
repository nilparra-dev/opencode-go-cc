# Troubleshooting

## Proxy won't start

### Port already in use

```bash
# Find what's using port 3456
lsof -i :3456
# or
netstat -tlnp | grep 3456

# Use a different port
occb serve --port 8080
```

### Config file not found

Run `occb init` to create the default configuration.

## Claude Code still using Anthropic after `occb on`

1. Check that the proxy is running:
   ```bash
   occb status
   ```

2. Verify Claude Code settings:
   ```bash
   cat ~/.claude/settings.json
   ```
   
   You should see (mixed mode, the default):
   ```json
   {
     "env": {
       "ANTHROPIC_BASE_URL": "http://127.0.0.1:3456",
       "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
     }
   }
   ```
   With `occb on --exclusive` you will also see `ANTHROPIC_AUTH_TOKEN: "unused"` and the pinned model variables.

3. Restart Claude Code. It watches `settings.json` for changes, but a restart ensures it picks up the new environment.

## OpenCode models do not appear in `/model`

Restart Claude Code after `occb on` (it reads `settings.json` at startup), and check that
`occb status` shows the proxy running. The list comes from the proxy's `/v1/models`.

## Claude models fail with 401 or 502 in mixed mode

Claude requests are forwarded to Anthropic with your own credentials, so you must be logged in
(`claude /login`) or have your own API key configured. A 502 means Anthropic was unreachable;
check `anthropic.base_url` in the occb config.

## API errors

### 401 Unauthorized

Your OpenCode Go API key is missing or invalid. Check:
- `OCB_API_KEY` environment variable is set
- `api_key` in `~/.config/occb/config.yaml` is correct

### All models failed

This usually means OpenCode Go's API is down or your API key has rate limits. Check:
- Your internet connection
- OpenCode Go status page
- Your subscription limits

## Reset everything

```bash
# Stop proxy
occb off

# Remove config
rm -rf ~/.config/occb

# Remove Claude Code proxy settings
occb off
```
