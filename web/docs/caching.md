# Caching Strategy

lcc.live sits behind Cloudflare (CF) → Fly.io (origin). This document
explains the Cache-Control headers we set and why.

## TL;DR

| Route | Cache-Control | Why |
|---|---|---|
| Images (`/image/:id`) | `public, max-age=3, stale-while-revalidate=120` | Match 3s poll cadence; CF absorbs spikes |
| Pages & JSON (`/`, `/lcc`, `/bcc`, `*.json`) | `public, max-age=30, stale-while-revalidate=120, must-revalidate` | Content changes infrequently; long SWR for spikes |
| Static assets (`/s/*`) | `public, max-age=86400, immutable` | Fingerprinted filenames |

## How stale-while-revalidate works with Cloudflare

With `max-age=N, stale-while-revalidate=M`:

1. For the first N seconds, CF serves the cached response as **fresh** — zero
   requests to origin.
2. After N seconds, the response becomes **stale**. The next request triggers a
   **background revalidation** — CF serves the stale cached response instantly
   to the user and fetches a fresh copy from origin asynchronously.
3. While a revalidation is in-flight (~50ms RTT), CF **coalesces** concurrent
   requests — all of them get the stale cached response, only one hits origin.
4. After N+M seconds with no revalidation, the cached entry expires entirely
   and the next request blocks on origin.

## Why max-age=3 for images

The server polls upstream cameras every ~3 seconds. We tested `max-age` values
of 0, 1, 3, 5, 10, and 30 and modeled origin load at different traffic levels.

The formula for origin load with `max-age > 0` is:

    Origin RPS = (number of cached resources) x (active CF POPs) / max-age

This is **independent of traffic volume** — that's the key insight. CF could be
handling 100k rps and origin load stays the same.

With `max-age=0`, every request is immediately stale, so origin load scales
with traffic and only benefits from the ~50ms coalescing window.

### Modeled origin load (34 resources)

| max-age | CF @ 100 rps | CF @ 10k rps | CF @ 100k rps |
|---------|-------------|-------------|--------------|
|         | ~3 POPs     | ~10 POPs    | ~20 POPs     |
| **0**   | ~100        | ~3,400      | ~13,600      |
| **1**   | 102         | 340         | 680          |
| **3**   | **34**      | **113**     | **227**      |
| **5**   | 20          | 68          | 136          |
| **10**  | 10          | 34          | 68           |
| **30**  | 3           | 11          | 23           |

**max-age=3** is the sweet spot:

- Matches the upstream poll interval — images can't be fresher than 3s anyway.
- At 100k rps, origin sees only ~227 rps (99.8% absorbed by CF).
- Going lower (0 or 1) sacrifices spike protection for no real freshness gain.
- Going higher (10, 30) adds staleness users can perceive on a "live" camera page.

## Why stale-while-revalidate=120

The SWR window determines how long CF will serve stale content during a spike
before giving up and blocking on origin. 120 seconds means:

- A sudden traffic spike (e.g. Reddit/HN) is fully absorbed for 2 minutes per
  POP even if origin is slow or overwhelmed.
- Normal traffic patterns never hit the 120s boundary because revalidation
  completes in ~50ms and resets the timer.

## Load test results (siege, 2026-03-01)

Tested against production (CF → Fly.io DFW) with 34 URLs covering all route
types. All image routes return 120KB-545KB JPEGs from in-memory cache.

| Concurrency | Availability | Req/sec | Avg Response | Longest  | Failures |
|-------------|-------------|---------|-------------|----------|----------|
| 25          | 100.00%     | 56.7    | 439ms       | 2,450ms  | 0        |
| 50          | 100.00%     | 60.6    | 633ms       | 13,680ms | 0        |
| 100         | 100.00%     | 87.4    | 988ms       | 22,780ms | 0        |
| 125         | 98.72%      | 86.7    | 1,197ms     | 16,430ms | 34       |
| 150         | 99.84%      | 120.3   | 1,094ms     | 13,920ms | 6        |
| 200         | 99.09%      | 123.6   | 1,349ms     | 15,510ms | 35       |

- **Clean up to 100 concurrent** users with zero failures.
- Failures above 100 are SSL connection resets from CF rate limiting, not
  application errors.
- Response time degrades gracefully (440ms → 1.3s) rather than cliff-diving.
- Peak throughput plateaus at ~124 req/sec.
