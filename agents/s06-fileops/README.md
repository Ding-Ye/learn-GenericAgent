# s06-fileops — File read / write / patch

Three small functions match upstream's `do_file_read`, `do_file_write`, and
`do_file_patch`:

- `FileRead(path, start, count, keyword, showLineNos)` — range or keyword
  mode, with 5-line context window and `...` separators between gaps
- `FileWrite(path, content, mode)` — `write`/`append`/`prepend`
- `FilePatch(path, old, new)` — strict-uniqueness replace; ambiguous matches
  fail loudly so the LLM rewrites with more context

```bash
go run . -op read -path README.md -keyword Solution
go run . -op patch -path /tmp/x.txt -old "v1" -new "v2"
go test -count=1 ./...
```
