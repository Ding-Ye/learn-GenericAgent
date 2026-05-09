# s01 — The Minimal Agent Loop

## Problem

Upstream GenericAgent has ~3,500 lines of code, 9 tools, 5 memory tiers, and 4
provider implementations. But open `agent_loop.py` and you'll find **the core
loop is only 50 lines**. Everything else is an extension of those 50 lines.

If we read `agentmain.py` (500 lines) + `ga.py` (900 lines) + `llmcore.py`
(1500 lines) up front, our brains drown in streaming SSE, protocol-tool
parsing, and memory injection — and we miss the essence of the agent.

**So s01's job: pull the loop out and run it standalone.** No tools, no real
model, no memory. Just a mock provider and a `for` loop. This is the scaffold
we'll come back to in all 9 subsequent sessions.

## Solution

```
   ┌──────────────────────────────────────────┐
   │ messages = [system, user]                │
   └────────────┬─────────────────────────────┘
                ▼
        ┌────────────────┐    ◀───┐ turn += 1
        │ provider.Chat( │        │
        │   ctx, msgs,   │        │
        │   chunks)      │        │
        └────────┬───────┘        │
                 ▼                │
           response.Content       │
                 │                │
                 ▼                │
        append assistant msg ─────┘
                 │
                 ▼
            [TASK_DONE]
```

s01 always runs exactly one turn. Why: the mock provider returns no tool calls,
so a single reply ends the task. From s02 onward, with dispatch wired in, the
loop will actually iterate.

## How It Works

The `Run` function:

```go
func Run(ctx context.Context,
    provider Provider,
    sysPrompt, userInput string,
    maxTurns int,
    chunks chan<- string,
) (ExitInfo, error)
```

Three responsibilities:

1. **Build initial messages** — `[{system, sysPrompt}, {user, userInput}]`.
   This `(role, content)` pair is the lingua franca of LLM APIs. Don't invent
   a new shape.

2. **Poll the provider** — One `provider.Chat` per turn. `chunks` is a
   `chan<- string`; the provider pushes text fragments into it as it
   generates, and the caller can print them live (streaming UX).

3. **Decide whether to exit** — In s01: "model replied → done." This is
   placeholder logic; the real exit is decided in s03 by
   `StepOutcome.next_prompt == ""`.

`Provider` is an interface, not a struct. This matters: s01 uses
`MockProvider`, s04 swaps in `AnthropicProvider`, s09 swaps in `MixinProvider`,
and **`Run` doesn't change a single line**.

## What Changed

s01 is the first session — no diff to show. But here are contracts later
sessions will break:

| Contract | Broken in |
|----------|-----------|
| `Response` has only `Content` | s02 adds `ToolCalls []ToolCall` |
| Loop exits after one turn | s02 decides exit by tool_calls presence |
| `Provider.Chat` takes no tool list | s02 adds `tools []ToolSpec` |
| `MockProvider` returns plain text | s02 also has to mock tool_use blocks |

## Try It

```bash
cd agents/s01-loop
go run . -user "Hello?"
```

Output:

```
--- Turn 1 ---
Hi! I'm a mock agent. You said: Hello?. (s01 has no tools, so I'm done.)

[exit] reason=TASK_DONE turns=1
```

Run tests:

```bash
go test -count=1 ./...
# ok  github.com/Ding-Ye/learn-GenericAgent/agents/s01-loop  0.3s
```

Experiment:

```bash
go run . -user "test" -max-turns 0
# [exit] reason=MAX_TURNS_EXCEEDED turns=0
```

`-max-turns 0` means the loop body never executes; verifies the exit branch.

## Upstream Source Reading

Pair our 50-line `Run` with [`agent_loop.py:agent_runner_loop`](https://github.com/lsdefine/GenericAgent/blob/main/agent_loop.py#L33-L88):

```python
# agent_loop.py:38-43  ── messages init (same as ours)
messages = [
    {"role": "system", "content": system_prompt},
    {"role": "user", "content": initial_user_content if initial_user_content is not None else user_input}
]
turn = 0; handler.max_turns = max_turns
```

```python
# agent_loop.py:44-52  ── main loop skeleton
while turn < handler.max_turns:
    turn += 1; turnstr = f'LLM Running (Turn {turn}) ...'
    ...
    response_gen = client.chat(messages=messages, tools=tools_schema)
    if verbose:
        response = yield from response_gen   # ← Python yield from = our chunks <-
        yield '\n\n'
```

Note `yield from response_gen` — Python's generator composer. It re-yields
every yield from the inner generator out through the outer one. Go has no
such syntax, so we pass `chan<- string` explicitly: provider pushes chunks in,
caller drains them.

```python
# agent_loop.py:54-58  ── tool_calls decision (s01 cuts this, s02 restores it)
if not response.tool_calls:
    tool_calls = [{'tool_name': 'no_tool', 'args': {}}]
else:
    tool_calls = [...]
```

Our s01 "immediate TASK_DONE" maps to the upstream's `not response.tool_calls`
branch — except upstream has one more step, a `no_tool` placeholder. We'll add
that fallback in s03.

**Exercise**: open the link above and find the line `client.last_tools = ''`
(around line 47). Why reset the tool description every 10 turns? Answer in
s07.

---

Next up: [s02-tools](s02-tools.md) — give the loop tools.
