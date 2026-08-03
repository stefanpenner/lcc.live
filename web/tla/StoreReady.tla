----------------------------- MODULE StoreReady -----------------------------
\* Models store.FetchImages ready-gate (isWaitingOnFirstImageReady + imagesReady WaitGroup).
\*
\* Product code (store.go ~465-468):
\*   if s.isWaitingOnFirstImageReady.Load() {
\*       s.isWaitingOnFirstImageReady.Store(false)
\*       s.imagesReady.Done()
\*   }
\* Load then Store is NOT a CAS — two overlapping FetchImages can both Done().
\*
\* Readers: Get / GetWeatherStation wait on imagesReady; healthcheck uses IsReady().
\*
\* Plain-language states:
\*   waiting=TRUE  — first sync not finished; Get blocks; IsReady=false
\*   waiting=FALSE — first sync finished; Get proceeds; IsReady=true
\*   wgCount       — WaitGroup counter (Init=1); Done decrements; must never go < 0

EXTENDS Integers, FiniteSets

CONSTANTS
  MaxCycles,   \* concurrent FetchImages cycles (TLC tiny; product unbounded)
  Cameras      \* abstract camera ids (only need cardinality for fetch work)

ASSUME MaxCycles \in Nat /\ MaxCycles >= 1
ASSUME Cameras # {}

VARIABLES
  waiting,     \* BOOLEAN — isWaitingOnFirstImageReady
  wgCount,     \* Nat — imagesReady counter; Done → -1; panic if < 0
  cycle,       \* [1..MaxCycles -> cycle phase]
  sawWaiting,  \* [1..MaxCycles -> BOOLEAN] — non-atomic Load result
  maxCycles    \* lid on State (stable)

CycleId == 1..maxCycles

\* Cycle phases (plain language):
\*   Idle      — not running a FetchImages
\*   Fetching  — in-flight (per-camera work abstracted as one step)
\*   Observed  — finished wg.Wait(); Load()'d waiting into sawWaiting
\*   (then ActReady or skip)
Phase == {"Idle", "Fetching", "Observed"}

TypeInvariant ==
  /\ waiting \in BOOLEAN
  /\ wgCount \in Int
  /\ cycle \in [CycleId -> Phase]
  /\ sawWaiting \in [CycleId -> BOOLEAN]
  /\ maxCycles = MaxCycles

LidsStable == maxCycles = MaxCycles

Init ==
  /\ waiting = TRUE
  /\ wgCount = 1
  /\ maxCycles = MaxCycles
  /\ cycle = [c \in CycleId |-> "Idle"]
  /\ sawWaiting = [c \in CycleId |-> FALSE]

-----------------------------------------------------------------------------
\* Actions

\* Start a FetchImages cycle if under lid and idle
StartCycle(c) ==
  /\ c \in CycleId
  /\ cycle[c] = "Idle"
  /\ cycle' = [cycle EXCEPT ![c] = "Fetching"]
  /\ UNCHANGED <<waiting, wgCount, sawWaiting, maxCycles>>

\* Abstract: all cameras finished (wg.Wait returns). Move to observe ready flag.
FinishFetch(c) ==
  /\ cycle[c] = "Fetching"
  /\ cycle' = [cycle EXCEPT ![c] = "Observed"]
  \* Non-atomic Load of waiting — snapshot into local sawWaiting
  /\ sawWaiting' = [sawWaiting EXCEPT ![c] = waiting]
  /\ UNCHANGED <<waiting, wgCount, maxCycles>>

\* Act on observed Load (product: if Load then Store(false); Done())
\* BUG MODEL: uses sawWaiting, not re-check under CAS
ActReady_Buggy(c) ==
  /\ cycle[c] = "Observed"
  /\ cycle' = [cycle EXCEPT ![c] = "Idle"]
  /\ IF sawWaiting[c]
     THEN /\ waiting' = FALSE
          /\ wgCount' = wgCount - 1
     ELSE /\ UNCHANGED <<waiting, wgCount>>
  /\ UNCHANGED <<sawWaiting, maxCycles>>

\* FIX MODEL: CAS — only one transition waiting TRUE→FALSE does Done
ActReady_CAS(c) ==
  /\ cycle[c] = "Observed"
  /\ cycle' = [cycle EXCEPT ![c] = "Idle"]
  /\ IF waiting
     THEN /\ waiting' = FALSE
          /\ wgCount' = wgCount - 1
     ELSE /\ UNCHANGED <<waiting, wgCount>>
  /\ UNCHANGED <<sawWaiting, maxCycles>>

\* Product Next = CAS (matches store.go CompareAndSwap ready gate)
Next ==
  \/ \E c \in CycleId: StartCycle(c)
  \/ \E c \in CycleId: FinishFetch(c)
  \/ \E c \in CycleId: ActReady_CAS(c)

\* Historical buggy Load+Store (bait — must FAIL WgNonNeg)
Next_Buggy ==
  \/ \E c \in CycleId: StartCycle(c)
  \/ \E c \in CycleId: FinishFetch(c)
  \/ \E c \in CycleId: ActReady_Buggy(c)

Spec == Init /\ [][Next]_<<waiting, wgCount, cycle, sawWaiting, maxCycles>>
SpecBuggy == Init /\ [][Next_Buggy]_<<waiting, wgCount, cycle, sawWaiting, maxCycles>>

-----------------------------------------------------------------------------
\* Invariants

\* WaitGroup must never go negative (Go panics)
WgNonNeg == wgCount >= 0

\* Done at most once: counter is 0 or 1 (Init 1, single Done)
WgAtMostOnce == wgCount \in {0, 1}

\* Once ready, stays ready
ReadySticky == [](waiting = FALSE => [](waiting = FALSE))

\* Eventually ready if some cycle completes (weak fairness needed for liveness)
\* Safety: if any cycle is Idle after Observed+Act, waiting false if someone saw true
ConsistentReady ==
  (waiting = FALSE) => (wgCount = 0)

\* IsReady ≡ !waiting; blocked readers only when waiting
\* After ready: wgCount must be 0 (Done happened exactly once)
ReadyIffDone ==
  waiting = (wgCount = 1)

-----------------------------------------------------------------------------
\* State reduction notes (for product + TLC):
\* - maxCycles lid on State; TLC MaxCycles=2 explores overlap; product has
\*   initial FetchImages + ticker FetchImages (and tests spawn 5).
\* - Camera set not needed for ready-gate; Cameras constant is documentation.
\* - Bool waiting not "N images left"; product already uses atomic.Bool.
=============================================================================
