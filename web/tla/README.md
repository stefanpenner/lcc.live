# TLA+ models — store / fetch / UDOT

Adversarial specs for concurrent state in `web/store` + `web/udot`.

| Spec | What | TLC (tiny lids) |
|------|------|-----------------|
| `StoreReady` | product CAS ready gate | PASS |
| `StoreReadyBuggy` | bait: Load+Store + WaitGroup Done | **FAIL** `WgNonNeg` |
| `ImageFetch` | per-entry HEAD→GET→Write, dual ETag, ready | PASS core; bait `ReadyImpliesImage` **FAIL** |
| `UdotSnapshot` | independent domain gens + torn canyon page | PASS weak; bait `CoherentPage` **FAIL** |
| `OverlayGesture` | grid↔fullscreen, pinch zoom, swipe nav gates | PASS (~39 states) |
| `OverlayGestureBait` | swipe allowed under `suppressNav` | **FAIL** `NavSafe` |
| `OverlayGestureBaitPinchNav` | pinch-out changes `idx` | **FAIL** `NavSafe` |

```bash
tlc StoreReady.tla -c StoreReady.cfg          # expect pass
tlc StoreReady.tla -c StoreReadyBuggy.cfg     # expect fail
tlc ImageFetch.tla -c ImageFetch.cfg
tlc ImageFetch.tla -c ImageFetchBait.cfg      # expect fail
tlc UdotSnapshot.tla -c UdotSnapshot.cfg
tlc UdotSnapshot.tla -c UdotSnapshotBait.cfg  # expect fail
tlc -c OverlayGesture.cfg OverlayGesture.tla                 # expect pass
tlc -c OverlayGestureBait.cfg OverlayGesture.tla             # expect fail
tlc -c OverlayGestureBaitPinchNav.cfg OverlayGesture.tla     # expect fail
```

Lids: `MaxCycles=2`, `MaxVersions=2`, `MaxGen=2`, Overlay `MaxCams=3` `MaxFingers=2` — product may run more cycles / more cams; policy is the same.

See also `web/docs/concurrency.md`.
