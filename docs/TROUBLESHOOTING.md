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
   
   You should see:
   ```json
   {
     "env": {
       "ANTHROPIC_BASE_URL": "http://127.0.0.1:3456",
       "ANTHROPIC_AUTH_TOKEN": "unused"
     }
   }
   ```

3. Restart Claude Code. It watches `settings.json` for changes, but a restart ensures it picks up the new environment.

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
