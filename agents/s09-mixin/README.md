# s09-mixin — Multi-provider failover

`MixinProvider` wraps multiple `Provider`s and tries them in primary-first
order with **spring-back**: a successful fallback "sticks" for `stickyMS`
ms, after which the next call retries the primary.

```bash
go run .       # canned demo: primary fails (rate limit) → falls to fallback
go test -count=1 ./...
```

## Lessons

- Composition: `MixinProvider` *implements* `Provider` itself; transparent to
  the loop
- Retryable error heuristic via substring match (the upstream pattern)
- Sticky window with mutex to be safe under concurrent Chat calls
- 7 unit tests cover: success, fallback, sticky, spring-back, non-retryable,
  all-fail, and the retryable predicate
