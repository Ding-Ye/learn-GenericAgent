# s06 — 文件读写补丁工具

## Problem

Agent 不能直接编辑文件。它需要 3 个工具：
- **`file_read`**：按行号范围读，或按关键字搜索（带上下文窗口）
- **`file_write`**：写、追加、前置 3 种模式
- **`file_patch`**：找到一段唯一匹配的旧内容，替换为新内容

第 3 个最有意思——直接 `sed`-style 的全局替换太危险。如果 agent 想改一个变量名，
而该名字在文件里出现 10 次，全部替换会破坏 9 处无关代码。所以 `file_patch` 强制：
**old block 必须在文件中**有且仅有一处**完全匹配**。出现 0 次或 ≥ 2 次都返回错误，
让模型把 old block 加更多上下文重试。

这一规则是上游 `ga.py:file_patch` 的核心——也是好用的 patch 工具的最低要求。

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

`FileRead` 的核心是**关键字 + 5 行上下文**：

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

匹配行前后各 2 行（共 5 行）入 hits 集合。如果匹配点之间有间隙，输出时插入 `...\n` 作为
"省略标记"，让模型知道中间还有内容没读到。

`FileWrite` 三模式都很普通：`os.WriteFile` / `O_APPEND` / 读旧 + 拼新写回。

## How It Works

3 个函数总共 ~80 行 Go。最有意思的设计点：

1. **`file_patch` 的失败语义**——返回 error 而非"成功 0 改动"。这迫使 loop 把错误回灌给模型。
   模型下次会主动加更多上下文行进 old block。
2. **`file_read` 的 `...\n` 标记**——按 keyword 模式查到 3 处分散匹配时，输出
   ```
   2: bravo
   3: charlie (match)
   ...
   12: gamma
   13: delta (match)
   ```
   `...` 让模型知道"中间还有内容"，不会误以为文件就这么 6 行。
3. **`FileWrite` 自动建目录**——`os.MkdirAll(filepath.Dir(p), 0755)`。模型经常会写 `a/b/c.txt`
   而 `a/b/` 还不存在；省一次 mkdir 工具调用。

## What Changed

s06 是另一个**独立工具模块**（像 s05）。装到 s_full 才有用。新增：

| 文件 | 内容 |
|------|------|
| `fileops.go` | `FileRead/FileWrite/FilePatch` 三函数 |
| `main.go` | CLI: `go run . -op read -path X -keyword Y` |
| `fileops_test.go` | 8 个测试覆盖所有分支 |

## Try It

```bash
cd agents/s06-fileops

# 范围读
echo -e "a\nb\nc\nd\ne" > /tmp/x.txt
go run . -op read -path /tmp/x.txt -start 2 -count 2
# 输出: 2: b\n3: c

# 关键字读（含上下文）
go run . -op read -path /tmp/x.txt -keyword c
# 输出包括 line 1..5 都 match 了（因为 c 在 line 3，前后 2 行 = 1..5）

# patch
go run . -op patch -path /tmp/x.txt -old "c" -new "C-NEW"
go run . -op read -path /tmp/x.txt
```

实验：在 `/tmp/x.txt` 里写 3 行相同的 `x = 1`，patch old="x = 1" new="x = 2"
看到 "matches 3 places" 错误。

## Upstream Source Reading

上游 3 个 `do_*` 方法在 `ga.py`，都很短，建议都读一遍：

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
        # ... 输出带 ... 间隔的行
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
    elif mode == 'prepend': old = open(path).read() if os.path.exists(path) else ''; open(path, 'w').write(content + old)
    yield f'[Write] {path} ({mode}) {len(content)} chars\n'
    return StepOutcome(data='success')
```

3 个工具加起来约 50 行——证明这一节"小但完整"。

下一节 [s07-memory](s07-memory.md)：分层记忆 L0-L4 + working checkpoint。
