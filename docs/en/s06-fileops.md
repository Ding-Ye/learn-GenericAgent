# s06 — File Read / Write / Patch Tools

## Problem

Agents can't directly edit files. They need three tools:
- **`file_read`** — by line range, or by keyword search (with context window)
- **`file_write`** — write, append, prepend
- **`file_patch`** — find a unique block and replace it

The third is the interesting one. A `sed`-style global replace is too
dangerous: if the agent wants to rename a variable that appears 10 times,
replacing all 10 corrupts 9 unrelated places. So `file_patch` enforces:
**old block must match exactly once**. Zero or two-or-more matches return
an error, forcing the model to retry with a more contextual `old` block.

That rule is the heart of upstream's `ga.py:file_patch` — and the bare
minimum for a usable patch tool.

## Solution

```go
func FilePatch(path, old, new string) error {
    s, _ := os.ReadFile(path)
    count := strings.Count(string(s), old)
    switch count {
    case 0: return errors.New("old block not found; reread the file")
    case 1: return os.WriteFile(path, strings.Replace(s, old, new, 1), 0o644)
    default: return fmt.Errorf("matches %d places; supply more context", count)
    }
}
```

`FileRead`'s soul is **keyword + 5-line context**:

```go
hits := map[int]struct{}{}
for i, line := range lines {
    if strings.Contains(line, keyword) {
        for k := i-2; k <= i+2; k++ {
            if k >= 0 && k < len(lines) { hits[k] = struct{}{} }
        }
    }
}
```

Each match accumulates ±2 surrounding lines (5 total). If matches sit far
apart, output inserts a `...\n` separator so the model knows there's content
in between it didn't read.

`FileWrite` has three trivial modes: `os.WriteFile` / `O_APPEND` / read-old
+ concatenate + write-back.

## How It Works

Three functions ~80 lines total. Notable design choices:

1. **`file_patch` failure semantics** — returns error rather than
   "succeeded with 0 changes." This forces the loop to feed the error to the
   model, which will retry with more `old` context.
2. **`file_read`'s `...\n` separator** — when keyword mode finds three
   scattered matches, the output looks like:
   ```
   2: bravo
   3: charlie (match)
   ...
   12: gamma
   13: delta (match)
   ```
   The `...` tells the model "more content exists between" so it doesn't
   assume the file is just six lines.
3. **`FileWrite` auto-creates parent dirs** — `os.MkdirAll(filepath.Dir(p), 0755)`.
   The model often writes `a/b/c.txt` with `a/b/` missing; saves a mkdir
   round-trip.

## What Changed

s06 is another **standalone tool module** (like s05). Becomes useful only
once mounted in s_full. New files:

| File | Content |
|------|---------|
| `fileops.go` | three functions: `FileRead/FileWrite/FilePatch` |
| `main.go` | CLI: `go run . -op read -path X -keyword Y` |
| `fileops_test.go` | 8 tests covering all branches |

## Try It

```bash
cd agents/s06-fileops

# Range read
printf "a\nb\nc\nd\ne\n" > /tmp/x.txt
go run . -op read -path /tmp/x.txt -start 2 -count 2
# Output: 2: b\n3: c

# Keyword read (with context)
go run . -op read -path /tmp/x.txt -keyword c
# Includes lines 1..5 (c is on line 3, ±2 → 1..5)

# Patch
go run . -op patch -path /tmp/x.txt -old "c" -new "C-NEW"
go run . -op read -path /tmp/x.txt
```

Experiment: write 3 identical `x = 1` lines into `/tmp/x.txt`, patch
old="x = 1" new="x = 2", and watch the "matches 3 places" error.

## Upstream Source Reading

Upstream's three `do_*` methods are all in `ga.py`, all short, all worth
reading:

```python
# ga.py:file_patch
def file_patch(path: str, old_content: str, new_content: str):
    with open(path, encoding='utf-8') as f: text = f.read()
    occurrences = text.count(old_content)
    if occurrences == 0:
        raise ValueError(f"old_content not found: {old_content[:60]!r}")
    elif occurrences > 1:
        raise ValueError(f"old_content matches {occurrences} places; add more context")
    new_text = text.replace(old_content, new_content, 1)
    with open(path, 'w', encoding='utf-8') as f: f.write(new_text)
```

```python
# ga.py:file_read
def file_read(path, start=1, keyword=None, count=200, show_linenos=True):
    lines = open(path, encoding='utf-8').readlines()
    if keyword:
        keep = set()
        for i, ln in enumerate(lines):
            if keyword in ln:
                for k in range(max(0, i-2), min(len(lines), i+3)): keep.add(k)
        # ... emit with ... separators
    else:
        # range mode
```

```python
# ga.py:do_file_write
def do_file_write(self, args, response):
    path, content, mode = args['path'], args['content'], args.get('mode', 'write')
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if mode == 'write': open(path, 'w').write(content)
    elif mode == 'append': open(path, 'a').write(content)
    elif mode == 'prepend':
        old = open(path).read() if os.path.exists(path) else ''
        open(path, 'w').write(content + old)
    yield f'[Write] {path} ({mode}) {len(content)} chars\n'
    return StepOutcome(data='success')
```

Three tools in ~50 lines — proof this chapter is "small but complete."

Next: [s07-memory](s07-memory.md) — layered memory L0-L4 + working
checkpoint.
