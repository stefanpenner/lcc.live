---------------------------- MODULE ImageFetch -----------------------------
\* Models one camera Entry under concurrent FetchImages cycles + readers.
\*
\* Product concurrency (store.go):
\*   1. entry.Read → copy src + HTTPHeaders.ETag
\*   2. HEAD (no lock); compare upstream ETag to copy
\*   3. GET body (no lock); content-hash → Image.ETag
\*   4. entry.Write → replace Image + HTTPHeaders
\*
\* Dual ETags:
\*   headETag  — upstream HEAD ETag (HTTPHeaders.ETag); skip GET if match
\*   bodyETag  — xxhash of bytes (Image.ETag); served to clients
\*
\* Plain-language Entry:
\*   Empty     — never successfully fetched (Status≠OK; ImageRoute → 404)
\*   Fresh(v)  — has body version v
\*   headETag  — last seen upstream validator (may be None)
\*
\* Bugs / gaps modeled:
\*   - Lost-update under overlapping cycles (last write wins; OK if same camera)
\*   - Stale skip: cycle A reads headETag=X; cycle B writes X→Y; A still GETs
\*     (wasteful, not corrupt)
\*   - No conditional GET (If-None-Match) — always full GET when head differs
\*   - ContentLength from header may ≠ len(bytes) — abstracted as lenOk BOOLEAN
\*   - Ready can be true while Entry Empty (all fetches failed)

EXTENDS Naturals, FiniteSets

CONSTANTS
  MaxCycles,
  MaxVersions   \* abstract body versions 0..MaxVersions (0 = empty/none)

ASSUME MaxCycles \in Nat /\ MaxCycles >= 1
ASSUME MaxVersions \in Nat /\ MaxVersions >= 1

VARIABLES
  \* Entry state (one camera)
  bodyVer,       \* 0 = empty; else content version
  headETag,      \* 0 = none; else validator version (abstract)
  statusOK,      \* BOOLEAN — HTTPHeaders.Status == 200
  lenOk,         \* BOOLEAN — ContentLength matches bytes (product can be false)
  \* Cycles
  phase,         \* [CycleId -> Phase]
  snapHead,      \* [CycleId -> Nat] snapshot of headETag at Read
  fetchBody,     \* [CycleId -> Nat] body version obtained from origin
  fetchHead,     \* [CycleId -> Nat] head etag from origin this cycle
  \* Origin (adversarial)
  originBody,    \* current origin body version
  originHead,    \* current origin head etag
  \* Ready gate (collapsed from StoreReady)
  ready,         \* BOOLEAN
  \* lids
  maxCycles,
  maxVersions

CycleId == 1..maxCycles
Ver == 0..maxVersions

\* Phases: Idle | ReadSnap | HeadDone | GotBody | Writing
Phase == {"Idle", "ReadSnap", "HeadDone", "GotBody", "Writing"}

TypeInvariant ==
  /\ bodyVer \in Ver
  /\ headETag \in Ver
  /\ statusOK \in BOOLEAN
  /\ lenOk \in BOOLEAN
  /\ phase \in [CycleId -> Phase]
  /\ snapHead \in [CycleId -> Ver]
  /\ fetchBody \in [CycleId -> Ver]
  /\ fetchHead \in [CycleId -> Ver]
  /\ originBody \in Ver
  /\ originHead \in Ver
  /\ ready \in BOOLEAN
  /\ maxCycles = MaxCycles
  /\ maxVersions = MaxVersions

LidsStable ==
  /\ maxCycles = MaxCycles
  /\ maxVersions = MaxVersions

Init ==
  /\ bodyVer = 0
  /\ headETag = 0
  /\ statusOK = FALSE
  /\ lenOk = TRUE
  /\ maxCycles = MaxCycles
  /\ maxVersions = MaxVersions
  /\ phase = [c \in CycleId |-> "Idle"]
  /\ snapHead = [c \in CycleId |-> 0]
  /\ fetchBody = [c \in CycleId |-> 0]
  /\ fetchHead = [c \in CycleId |-> 0]
  /\ originBody = 1
  /\ originHead = 1
  /\ ready = FALSE

-----------------------------------------------------------------------------

Start(c) ==
  /\ phase[c] = "Idle"
  /\ phase' = [phase EXCEPT ![c] = "ReadSnap"]
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, snapHead, fetchBody,
                 fetchHead, originBody, originHead, ready, maxCycles, maxVersions>>

\* entry.Read: copy head etag
DoRead(c) ==
  /\ phase[c] = "ReadSnap"
  /\ snapHead' = [snapHead EXCEPT ![c] = headETag]
  /\ phase' = [phase EXCEPT ![c] = "HeadDone"]
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, fetchBody, fetchHead,
                 originBody, originHead, ready, maxCycles, maxVersions>>

\* HEAD response: either match snap → skip, or take origin head and proceed to GET
\* Origin may also fail (head stays 0 → force GET path with failure option)
HeadMatchSkip(c) ==
  /\ phase[c] = "HeadDone"
  /\ originHead # 0
  /\ originHead = snapHead[c]
  /\ snapHead[c] # 0
  /\ phase' = [phase EXCEPT ![c] = "Idle"]
  \* mark ready if first completion (simplified single-step)
  /\ ready' = TRUE
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, snapHead, fetchBody,
                 fetchHead, originBody, originHead, maxCycles, maxVersions>>

HeadMiss(c) ==
  /\ phase[c] = "HeadDone"
  /\ ~(originHead # 0 /\ originHead = snapHead[c] /\ snapHead[c] # 0)
  /\ fetchHead' = [fetchHead EXCEPT ![c] = originHead]
  /\ phase' = [phase EXCEPT ![c] = "GotBody"]
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, snapHead, fetchBody,
                 originBody, originHead, ready, maxCycles, maxVersions>>

\* GET success: body = originBody, optional len mismatch
GetOK(c) ==
  /\ phase[c] = "GotBody"
  /\ originBody # 0
  /\ fetchBody' = [fetchBody EXCEPT ![c] = originBody]
  /\ phase' = [phase EXCEPT ![c] = "Writing"]
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, snapHead, fetchHead,
                 originBody, originHead, ready, maxCycles, maxVersions>>

\* GET failure: leave entry unchanged; still can mark ready
GetFail(c) ==
  /\ phase[c] = "GotBody"
  /\ phase' = [phase EXCEPT ![c] = "Idle"]
  /\ ready' = TRUE
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, snapHead, fetchBody,
                 fetchHead, originBody, originHead, maxCycles, maxVersions>>

\* entry.Write
DoWrite(c) ==
  /\ phase[c] = "Writing"
  /\ bodyVer' = fetchBody[c]
  /\ headETag' = fetchHead[c]
  /\ statusOK' = TRUE
  \* Adversarial: Content-Length header may not match bytes
  /\ lenOk' \in BOOLEAN
  /\ phase' = [phase EXCEPT ![c] = "Idle"]
  /\ ready' = TRUE
  /\ UNCHANGED <<snapHead, fetchBody, fetchHead, originBody, originHead,
                 maxCycles, maxVersions>>

\* Origin changes (adversary)
OriginChange ==
  /\ originBody' \in Ver
  /\ originHead' \in Ver
  /\ originBody' # 0  \* origin always has some body when up
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, phase, snapHead,
                 fetchBody, fetchHead, ready, maxCycles, maxVersions>>

\* Reader snapshot (ShallowSnapshot) — only when ready (Get waits)
\* Abstract: reader observes bodyVer/statusOK/lenOk atomically under RLock
ReadOK ==
  /\ ready = TRUE
  /\ UNCHANGED <<bodyVer, headETag, statusOK, lenOk, phase, snapHead,
                 fetchBody, fetchHead, originBody, originHead, ready,
                 maxCycles, maxVersions>>

Next ==
  \/ \E c \in CycleId: Start(c)
  \/ \E c \in CycleId: DoRead(c)
  \/ \E c \in CycleId: HeadMatchSkip(c)
  \/ \E c \in CycleId: HeadMiss(c)
  \/ \E c \in CycleId: GetOK(c)
  \/ \E c \in CycleId: GetFail(c)
  \/ \E c \in CycleId: DoWrite(c)
  \/ OriginChange
  \/ ReadOK

Spec == Init /\ [][Next]_<<bodyVer, headETag, statusOK, lenOk, phase, snapHead,
                            fetchBody, fetchHead, originBody, originHead, ready,
                            maxCycles, maxVersions>>

-----------------------------------------------------------------------------
\* Invariants

\* Entry never shows statusOK without a body version
StatusImpliesBody ==
  statusOK => bodyVer # 0

\* Served body etag identity: bodyVer is the content id (Image.ETag)
\* headETag may lag/differ — not required equal (dual-etag design)

\* BAD property if claimed: ready ⇒ statusOK (FALSE in product — ready after
\* all-error first cycle). Bait invariant:
ReadyImpliesImage ==
  ready => statusOK

\* Content-Length integrity if we claim headers match body
LenAlwaysOk ==
  lenOk = TRUE

\* bodyVer only changes via write to a version from a cycle's fetch
\* (structural via actions)

\* No torn read under entry mutex: statusOK and bodyVer updated together in DoWrite
NoTorn ==
  statusOK <=> (bodyVer # 0)

=============================================================================
