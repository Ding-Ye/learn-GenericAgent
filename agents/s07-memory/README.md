# s07-memory — Layered memory + working checkpoint

Five tiers from hardcoded → archived:

- **L0** hardcoded base prompt (in `memory.go`)
- **L1** routing index → `memory/global_mem_insight.txt`
- **L2** stable accumulated knowledge → `memory/global_mem.txt`
- **L3** per-topic SOP markdown → `memory/<topic>_sop.md`
- **L4** archived session records → `memory/sessions/<microsec>.txt`

Plus a **working checkpoint** that lives across turns within one task.

`AssembleSystemPrompt()` glues L0 + L1 + L2 + checkpoint into the system
prompt sent to the model each turn.

```bash
go run . -dir ./mem -op append -value "rule: never delete files"
go run . -dir ./mem -op show
go test -count=1 ./...
```
