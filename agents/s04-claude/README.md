# s04-claude — Real Anthropic Claude provider

Replaces the mock provider with raw HTTP calls to
`https://api.anthropic.com/v1/messages` (SSE streaming). Same `Provider`
interface from s01 — drop-in.

```bash
export ANTHROPIC_API_KEY=sk-ant-...
go run . -user "List 3 fruits."
```

## Tests (offline)

```bash
go test -count=1 ./...
```

All tests use `httptest.NewServer` — no network, no API key. Five cases:
text streaming, tool_use streaming, role conversion (system fold +
tool_result→user), API error event, HTTP 429.
