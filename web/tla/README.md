# TLA+ models — store / fetch / UDOT

Adversarial specs for concurrent state in `web/store` + `web/udot`.

| Spec | What | TLC (tiny lids) |
|------|------|-----------------|
| `StoreReady` | product CAS ready gate | PASS |
| `StoreReadyBuggy` | bait: Load+Store + WaitGroup Done | **FAIL** `WgNonNeg` |
| `ImageFetch` | per-entry HEAD→GET→Write, dual ETag, ready | PASS core; bait `ReadyImpliesImage` **FAIL** |
| `UdotSnapshot` | independent domain gens + torn canyon page | PASS weak; bait `CoherentPage` **FAIL** |

```bash
tlc StoreReady.tla -c StoreReady.cfg          # expect pass
tlc StoreReady.tla -c StoreReadyBuggy.cfg     # expect fail
tlc ImageFetch.tla -c ImageFetch.cfg
tlc ImageFetch.tla -c ImageFetchBait.cfg      # expect fail
tlc UdotSnapshot.tla -c UdotSnapshot.cfg
tlc UdotSnapshot.tla -c UdotSnapshotBait.cfg  # expect fail
```

Lids: `MaxCycles=2`, `MaxVersions=2`, `MaxGen=2` — product may run more cycles; policy is the same.

See also `web/docs/concurrency.md`.
