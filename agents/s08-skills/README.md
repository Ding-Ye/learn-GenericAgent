# s08-skills — Skill tree & skill search

Loads `*_sop.md` files from a directory and exposes:

- `tree.List()` — index for the system prompt
- `tree.Search(query)` — substring filter for the `skill_search` tool
- `tree.Load(name)` — full body for the `load_skill` tool

```bash
mkdir /tmp/skills
cat > /tmp/skills/browser_sop.md <<EOF
# Browser SOP
Use this when driving Chrome via CDP.
EOF

go run . -dir /tmp/skills -op list
go run . -dir /tmp/skills -op search -q chrome
go run . -dir /tmp/skills -op load -q browser
go test -count=1 ./...
```
