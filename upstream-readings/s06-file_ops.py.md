# Upstream reading for s06 — file ops

All three are in [`ga.py`](https://github.com/lsdefine/GenericAgent/blob/main/ga.py).

## Read these by searching the file

- `def file_read(`
- `def file_patch(`
- `def do_file_write(`
- `def _scan_files(`  ← bonus: directory scan helper

## Key invariants

1. **`file_patch` fails on ambiguous match.** The agent must be specific.
   If it's not, `next_prompt` carries the failure back; the model adds more
   context lines and retries.

2. **`file_read` always shows line numbers by default.** This means the
   agent's subsequent `file_patch` arguments can be reasoned about with line
   numbers in mind, even though `file_patch` actually matches by string.

3. **`do_file_write` creates parent dirs.** No "directory does not exist"
   tool errors — the agent rarely cares about that.

4. **No backup on overwrite.** If the agent destroys content with
   `mode="write"`, that's its problem. Trust the model; this is consistent
   with the upstream "minimum-friction" philosophy.

## What we deliberately omit

- The `expand_file_refs` helper that lets agents reference files via inline
  `@file:path` syntax — useful in a chat UI but out of scope for the learn
  version
- The `_scan_files` directory walker (a depth-2 ls) — easy to add as
  homework

## Read the test cases too

In our learn version `fileops_test.go` exercises the same edge cases that
upstream's manual tests catch (see `memory/verify_sop.md` in the upstream
repo for their checklist).
