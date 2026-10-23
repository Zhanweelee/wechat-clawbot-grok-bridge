# Contributing

Thank you for helping improve **wechat-clawbot-grok-bridge**.

## Ground rules

- This project is an independent clean-room implementation. Do not copy code, file layouts, or license text from other WeChat bot projects.
- Keep secrets out of git. Use `config.json` (gitignored) or `WCG_*` environment variables.
- Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `chore:`, `test:`).

## Development

```bash
go test ./...
go build -o bin/wechat-clawbot-grok-bridge ./cmd/wechat-clawbot-grok-bridge
```

## Pull requests

- One focused change per PR when possible.
- Include tests for protocol/client behavior when you change logic.
- Update README configuration docs if behavior or flags change.

Copyright © 2026 Jonathan Lee.
