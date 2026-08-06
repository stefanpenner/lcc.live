---------------------------- MODULE OverlayGesture ----------------------------
\* Fullscreen camera viewer — gesture policy (grid ↔ fullscreen, pinch zoom, swipe).
\*
\* Product map (script.mjs FullscreenViewer):
\*   mode=Grid       — canyon list; click thumb → open
\*   mode=Fullscreen — overlay open; click image → close (minimize)
\*   zoomed          — scale abstracted to BOOLEAN (Fit vs Zoomed)
\*   fingers         — 0..maxFingers (TLC tiny; product unbounded multi-touch)
\*   pinching        — multi-touch zoom session (sticky until all fingers up)
\*   suppressNav     — after pinch, block gallery next/prev (not dismiss)
\*   swipeCandidate  — pure 1-finger stroke; false once multi-touch joined
\*
\* Plain-language status:
\*   Grid + fingers=0     — browsing thumbs; page pinch blocked in product UI
\*   Fullscreen + Fit     — click closes; swipe H next/prev; swipe V down closes
\*   Fullscreen + Zoomed  — pinch/pan; no gallery nav; swipe V down may close
\*   pinching=TRUE        — multi-touch session; only zoom; never change idx
\*   suppressNav=TRUE     — just finished/in pinch; block next/prev only
\*   swipeCandidate=TRUE  — this stroke never saw 2+ fingers (gates SwipeClose)
\*
\* Reduce:
\*   lids on State: maxCams, maxFingers (TLC 3/2; product higher OK)
\*   zoomed BOOLEAN not continuous scale
\*   suppress / swipeCandidate BOOL not timer ticks
\*   pinching sticky to fingers=0 (no separate PinchEnd needed for leave)
\*   CanSwipe = gallery only; CanSwipeClose = dismiss (orthogonal)

EXTENDS Integers, FiniteSets, Sequences

CONSTANTS
  MaxCams,      \* gallery size lid (TLC tiny e.g. 3)
  MaxFingers    \* simultaneous touches lid (TLC 2)

ASSUME MaxCams \in Nat /\ MaxCams >= 2
ASSUME MaxFingers \in Nat /\ MaxFingers >= 2

VARIABLES
  mode,            \* "Grid" | "Fullscreen"
  zoomed,          \* BOOLEAN — Fit=FALSE, Zoomed=TRUE
  idx,             \* gallery index 0..maxCams-1
  fingers,         \* 0..maxFingers
  pinching,        \* BOOLEAN — multi-touch zoom session active
  suppressNav,     \* BOOLEAN — block gallery swipe after pinch
  swipeCandidate,  \* BOOLEAN — pure 1-finger stroke (product swipeCandidate)
  maxCams,         \* lid
  maxFingers       \* lid

Mode == {"Grid", "Fullscreen"}

TypeInvariant ==
  /\ mode \in Mode
  /\ zoomed \in BOOLEAN
  /\ idx \in 0..(maxCams - 1)
  /\ fingers \in 0..maxFingers
  /\ pinching \in BOOLEAN
  /\ suppressNav \in BOOLEAN
  /\ swipeCandidate \in BOOLEAN
  /\ maxCams = MaxCams
  /\ maxFingers = MaxFingers

LidsStable ==
  /\ maxCams = MaxCams
  /\ maxFingers = MaxFingers

-----------------------------------------------------------------------------
\* Derived gates (policy)

InGrid == mode = "Grid"
InFullscreen == mode = "Fullscreen"
AtFit == InFullscreen /\ ~zoomed

\* Gallery next/prev only — Fit, not pinching, not post-pinch suppress
CanSwipe ==
  /\ AtFit
  /\ ~pinching
  /\ ~suppressNav
  /\ fingers <= 1

\* Swipe-down dismiss — Fit or Zoomed; suppressNav OK; pure 1-finger stroke
\* Product evaluates at fingers=0 after last lift (canSwipeClose)
CanSwipeClose ==
  /\ InFullscreen
  /\ swipeCandidate
  /\ ~pinching
  /\ fingers = 0

\* Zoomed only makes sense fullscreen
ZoomedOnlyFullscreen ==
  zoomed => InFullscreen

\* Pinch only while fullscreen
PinchOnlyFullscreen ==
  pinching => InFullscreen

Init ==
  /\ mode = "Grid"
  /\ zoomed = FALSE
  /\ idx = 0
  /\ fingers = 0
  /\ pinching = FALSE
  /\ suppressNav = FALSE
  /\ swipeCandidate = FALSE
  /\ maxCams = MaxCams
  /\ maxFingers = MaxFingers

-----------------------------------------------------------------------------
\* Actions — reduced user/OS events

\* --- finger accounting (abstract multi-touch) ---

FingerDown ==
  /\ fingers < maxFingers
  /\ fingers' = fingers + 1
  /\ IF InFullscreen /\ fingers + 1 >= 2
     THEN \* multi-touch joins → pinch session; stroke is no longer pure-1
          /\ pinching' = TRUE
          /\ suppressNav' = TRUE
          /\ swipeCandidate' = FALSE
     ELSE IF InFullscreen /\ fingers + 1 = 1
          THEN \* first finger of a stroke
               /\ swipeCandidate' = TRUE
               /\ UNCHANGED <<pinching, suppressNav>>
          ELSE UNCHANGED <<pinching, suppressNav, swipeCandidate>>
  /\ UNCHANGED <<mode, zoomed, idx, maxCams, maxFingers>>

FingerUp ==
  /\ fingers > 0
  /\ fingers' = fingers - 1
  /\ IF fingers' = 0
     THEN \* last finger up: end pinch session; keep swipeCandidate for Swipe*
          /\ pinching' = FALSE
          /\ IF pinching THEN suppressNav' = TRUE ELSE UNCHANGED suppressNav
          /\ UNCHANGED swipeCandidate
     ELSE IF pinching
          THEN \* leftover finger after multi-touch — not a swipe-close candidate
               /\ UNCHANGED pinching
               /\ suppressNav' = TRUE
               /\ swipeCandidate' = FALSE
          ELSE UNCHANGED <<pinching, suppressNav, swipeCandidate>>
  /\ UNCHANGED <<mode, zoomed, idx, maxCams, maxFingers>>

\* --- open / close ---

\* Click small thumb on grid → open fullscreen at Fit
ClickThumb ==
  /\ InGrid
  /\ fingers = 0
  /\ ~pinching
  /\ mode' = "Fullscreen"
  /\ zoomed' = FALSE
  /\ suppressNav' = FALSE
  /\ pinching' = FALSE
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<idx, fingers, maxCams, maxFingers>>

\* Click fullscreen image → minimize (product: blocked while pinching or suppressNav)
ClickFullscreen ==
  /\ InFullscreen
  /\ fingers <= 1
  /\ ~pinching
  /\ ~suppressNav
  /\ mode' = "Grid"
  /\ zoomed' = FALSE
  /\ pinching' = FALSE
  /\ suppressNav' = FALSE
  /\ swipeCandidate' = FALSE
  /\ fingers' = 0
  /\ UNCHANGED <<idx, maxCams, maxFingers>>

\* Escape / programmatic close — always allowed when open (product Escape)
CloseAlways ==
  /\ InFullscreen
  /\ mode' = "Grid"
  /\ zoomed' = FALSE
  /\ pinching' = FALSE
  /\ suppressNav' = FALSE
  /\ swipeCandidate' = FALSE
  /\ fingers' = 0
  /\ UNCHANGED <<idx, maxCams, maxFingers>>

\* --- zoom via pinch (abstract in / out) ---

PinchIn ==
  /\ InFullscreen
  /\ fingers >= 2
  /\ pinching = TRUE
  /\ zoomed' = TRUE
  /\ suppressNav' = TRUE
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<mode, idx, fingers, pinching, maxCams, maxFingers>>

\* Pinch toward fit — may clear zoomed; must NOT change idx
PinchOut ==
  /\ InFullscreen
  /\ fingers >= 2
  /\ pinching = TRUE
  /\ zoomed' = FALSE
  /\ suppressNav' = TRUE
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<mode, idx, fingers, pinching, maxCams, maxFingers>>

\* Clear suppress after cooldown (product: 450ms; only when idle)
ClearSuppress ==
  /\ suppressNav = TRUE
  /\ ~pinching
  /\ fingers = 0
  /\ suppressNav' = FALSE
  /\ UNCHANGED <<mode, zoomed, idx, fingers, pinching, swipeCandidate, maxCams, maxFingers>>

\* Double-tap toggle Fit ↔ Zoomed (fullscreen only; not mid-pinch/suppress)
DoubleTap ==
  /\ InFullscreen
  /\ fingers <= 1
  /\ ~pinching
  /\ ~suppressNav
  /\ zoomed' = ~zoomed
  /\ UNCHANGED <<mode, idx, fingers, pinching, suppressNav, swipeCandidate, maxCams, maxFingers>>

\* --- gallery navigation (horizontal swipe) — only when CanSwipe ---

SwipeNext ==
  /\ CanSwipe
  /\ idx < maxCams - 1
  /\ idx' = idx + 1
  /\ zoomed' = FALSE
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<mode, fingers, pinching, suppressNav, maxCams, maxFingers>>

SwipePrev ==
  /\ CanSwipe
  /\ idx > 0
  /\ idx' = idx - 1
  /\ zoomed' = FALSE
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<mode, fingers, pinching, suppressNav, maxCams, maxFingers>>

\* --- dismiss (vertical swipe down) — CanSwipeClose; not gallery nav ---

SwipeClose ==
  /\ CanSwipeClose
  /\ mode' = "Grid"
  /\ zoomed' = FALSE
  /\ pinching' = FALSE
  /\ suppressNav' = FALSE
  /\ swipeCandidate' = FALSE
  /\ fingers' = 0
  /\ UNCHANGED <<idx, maxCams, maxFingers>>

\* BAIT / bug models (not in Next_Good) ------------------------------------

\* Bug: allow gallery swipe immediately after pinch (old product behavior)
SwipeNext_Buggy ==
  /\ AtFit
  /\ ~pinching
  \* missing: ~suppressNav
  /\ fingers <= 1
  /\ idx < maxCams - 1
  /\ idx' = idx + 1
  /\ zoomed' = FALSE
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<mode, fingers, pinching, suppressNav, maxCams, maxFingers>>

\* Bug: pinch-out advances gallery
PinchOut_BuggyNav ==
  /\ InFullscreen
  /\ fingers >= 2
  /\ pinching = TRUE
  /\ zoomed' = FALSE
  /\ idx' = IF idx < maxCams - 1 THEN idx + 1 ELSE idx
  /\ suppressNav' = TRUE
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<mode, fingers, pinching, maxCams, maxFingers>>

\* Bug: treat last-finger-up after pinch as dismiss (no pure 1-finger stroke).
\* Leaves suppress stuck on Grid (product must clear on close) — fails Inv_GridIdle.
SwipeClose_Buggy ==
  /\ InFullscreen
  /\ ~pinching
  /\ fingers = 0
  /\ ~swipeCandidate   \* missing pure-stroke gate
  /\ mode' = "Grid"
  /\ zoomed' = FALSE
  /\ pinching' = FALSE
  /\ UNCHANGED suppressNav
  /\ swipeCandidate' = FALSE
  /\ UNCHANGED <<idx, fingers, maxCams, maxFingers>>

-----------------------------------------------------------------------------

Next_Good ==
  \/ FingerDown
  \/ FingerUp
  \/ ClickThumb
  \/ ClickFullscreen
  \/ CloseAlways
  \/ PinchIn
  \/ PinchOut
  \/ ClearSuppress
  \/ DoubleTap
  \/ SwipeNext
  \/ SwipePrev
  \/ SwipeClose

Next_BuggySwipeAfterPinch ==
  \/ Next_Good
  \/ SwipeNext_Buggy

Next_BuggyPinchNav ==
  \/ FingerDown
  \/ FingerUp
  \/ ClickThumb
  \/ ClickFullscreen
  \/ CloseAlways
  \/ PinchIn
  \/ PinchOut
  \/ PinchOut_BuggyNav
  \/ ClearSuppress
  \/ DoubleTap
  \/ SwipeNext
  \/ SwipePrev
  \/ SwipeClose

Next_BuggySwipeCloseAfterPinch ==
  \/ Next_Good
  \/ SwipeClose_Buggy

\* Default Next for TLC good path
Next == Next_Good

vars == <<mode, zoomed, idx, fingers, pinching, suppressNav, swipeCandidate, maxCams, maxFingers>>

Spec == Init /\ [][Next]_vars

-----------------------------------------------------------------------------
\* Safety invariants

Inv_ZoomedImpliesFullscreen == ZoomedOnlyFullscreen

Inv_PinchImpliesFullscreen == PinchOnlyFullscreen

\* Pinching implies suppress (always arm suppress when pinching)
Inv_PinchImpliesSuppress ==
  pinching => suppressNav

\* Sticky pinch: session may hold with 1 leftover finger; not with 0
Inv_PinchHasFingers ==
  pinching => fingers >= 1

Inv_NoSwipeWhilePinching ==
  pinching => ~CanSwipe

Inv_SuppressBlocksSwipe ==
  suppressNav => ~CanSwipe

\* Multi-touch stroke cannot be a swipe-close candidate
Inv_PinchNotSwipeCandidate ==
  pinching => ~swipeCandidate

Inv_GridNotZoomed ==
  InGrid => (~zoomed /\ ~pinching /\ ~swipeCandidate)

\* Grid must not carry post-pinch suppress (product clears on open/close)
\* Note: fingers may be >0 on Grid (page multi-touch tracking); not an error.
Inv_GridIdle ==
  InGrid => ~suppressNav

TypeOK == TypeInvariant /\ LidsStable

Safe ==
  /\ TypeOK
  /\ Inv_ZoomedImpliesFullscreen
  /\ Inv_PinchImpliesFullscreen
  /\ Inv_PinchImpliesSuppress
  /\ Inv_PinchHasFingers
  /\ Inv_NoSwipeWhilePinching
  /\ Inv_SuppressBlocksSwipe
  /\ Inv_PinchNotSwipeCandidate
  /\ Inv_GridNotZoomed
  /\ Inv_GridIdle

-----------------------------------------------------------------------------
\* Temporal: idx only changes when CanSwipe held in pre-state

IdxChangeOnlyWhenCanSwipe ==
  (idx' # idx) =>
    (/\ AtFit /\ ~pinching /\ ~suppressNav /\ fingers <= 1)

\* mode Grid←Fullscreen via close paths may free-form; idx must not move on close
NavSafe == [][IdxChangeOnlyWhenCanSwipe]_vars

\* SwipeClose only when pure stroke (temporal over mode leave without idx change is soft)
\* Strong: leaving Fullscreen with SwipeClose-like step requires swipeCandidate ∨ click paths
\* Encoded as: if we go Fullscreen→Grid in one step with fingers staying 0 and
\* suppress was true and swipeCandidate false, must not be only "accidental" —
\* bait SwipeClose_Buggy exercises missing gate; good path never has that action.

SpecSafe == Init /\ [][Next_Good]_vars /\ []Safe

=============================================================================
