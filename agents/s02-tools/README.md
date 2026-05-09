# s02-tools — Tool registry & dispatch

The agent loop now dispatches tool calls. We add a `Registry` that maps tool
name → Go func, and the loop:

1. Sends the registered tool specs to the provider
2. On each turn, reads `resp.ToolCalls`
3. Dispatches each one, packaging the result as a `tool` message
4. Loops until the model replies in plain text

## Run

```bash
go run .
```

The mock provider scripts a 3-turn run: `echo` → `upper` → "all done".

## Tests

```bash
go test -count=1 ./...
```

Four cases: two-tool happy path, unknown tool feeds back error, registry
unknown-tool error message, MarshalToolResult string passthrough vs JSON.

## Diff vs s01

- `Provider.Chat` now takes `[]ToolSpec`
- `Response.ToolCalls` added
- `Registry` (new file `registry.go`) — `Register`/`Specs`/`Dispatch`
- Loop adds tool dispatch and tool-result message accumulation
- Exit decided by "no tool calls" instead of "always after 1 turn"
