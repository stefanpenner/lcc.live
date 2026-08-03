# Store concurrency

## Model

| Piece | Rule |
|-------|------|
| Camera indexes | Immutable after `NewStore` |
| Entry | Own `RWMutex`; values replaced (pointer swap), not mutated in place |
| `FetchImages` | **Single-flight** — overlapping calls skip |
| Ready gate | `CompareAndSwap(true→false)` then `imagesReady.Done()` **once** |
| Snapshots | `Get` / `ShallowSnapshot` are frozen handshakes |

## Dual ETag

| Field | Role |
|-------|------|
| `HTTPHeaders.ETag` | Upstream HEAD validator — skip GET when unchanged |
| `Image.ETag` | Content hash — served to clients / `If-None-Match` |

## Ready vs healthy

| Signal | Meaning |
|--------|---------|
| `IsReady()` | First fetch cycle finished (ok or all failed) |
| `HasAnyLiveImage()` | ≥1 non-iframe camera with Status 200 + body |
| `/healthcheck` | 503 until ready **and** ≥1 live image |

## TLA+

Specs under `web/tla/`. Product ready-gate matches `SpecFixed` (CAS).
Buggy Load+Store bait must still fail TLC.
