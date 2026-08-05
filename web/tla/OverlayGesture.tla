---------------------------- MODULE OverlayGesture ----------------------------
\* Fullscreen camera viewer — gesture policy (grid ↔ fullscreen, pinch zoom, swipe).
\*
\* Product map (script.mjs FullscreenViewer):
\*   mode=Grid       — canyon list; click thumb → open
\*   mode=Fullscreen — overlay open; click image → close (minimize)
\*   zoomed          — scale abstracted to BOOLEAN (Fit vs Zoomed)
\*   fingers         — 0..maxFingers (TLC tiny; product unbounded multi-touch)
\*   pinching        — multi-touch zoom session in progress
\*   suppressNav     — after pinch ends, block swipe next/prev for a beat
\*
\* Plain-language status:
\*   Grid + fingers=0     — browsing thumbs; page pinch blocked in product UI
\*   Fullscreen + Fit     — image fit; click closes; single swipe may next/prev
\*   Fullscreen + Zoomed  — pinch/pan; no gallery nav
\*   pinching=TRUE        — 2+ fingers; only zoom; never change photo index
\*   suppressNav=TRUE     — just finished pinch; finger-lift must not swipe
\*
\* Reduce:
\*   lids on State: maxCams, maxFingers (TLC 3/2; product higher OK)
\*   zoomed BOOLEAN not continuous scale
\*   suppress as BOOL not timer ticks
\*   swipe = one discrete Next/Prev when allowed

EXTENDS Integers, FiniteSets, Sequences

CONSTANTS
  MaxCams,      \* gallery size lid (TLC tiny e.g. 3)
  MaxFingers    \* simultaneous touches lid (TLC 2)

ASSUME MaxCams \in Nat /\ MaxCams >= 2
ASSUME MaxFingers \in Nat /\ MaxFingers >= 2

VARIABLES
  mode,         \* "Grid" | "Fullscreen"
  zoomed,       \* BOOLEAN — Fit=FALSE, Zoomed=TRUE
  idx,          \* gallery index 0..maxCams-1
  fingers,      \* 0..maxFingers
  pinching,     \* BOOLEAN — multi-touch zoom session active
  suppressNav,  \* BOOLEAN — block swipe after pinch (product: time window)
  maxCams,      \* lid
  maxFingers    \* lid

Mode == {"Grid", "Fullscreen"}

TypeInvariant ==
  /\ mode \in Mode
  /\ zoomed \in BOOLEAN
  /\ idx \in 0..(maxCams - 1)
  /\ fingers \in 0..maxFingers
  /\ pinching \in BOOLEAN
  /\ suppressNav \in BOOLEAN
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
CanSwipe ==
  /\ AtFit
  /\ ~pinching
  /\ ~suppressNav
  /\ fingers <= 1

\* Zoomed only makes sense fullscreen
ZoomedOnlyFullscreen ==
  zoomed => InFullscreen

\* Pinch only while fullscreen (product attaches zoom only in overlay)
PinchOnlyFullscreen ==
  pinching => InFullscreen

\* While pinching or suppressNav, gallery index must not move — checked via action guards
\* + temporal: see NavOnlyWhenAllowed

Init ==
  /\ mode = "Grid"
  /\ zoomed = FALSE
  /\ idx = 0
  /\ fingers = 0
  /\ pinching = FALSE
  /\ suppressNav = FALSE
  /\ maxCams = MaxCams
  /\ maxFingers = MaxFingers

-----------------------------------------------------------------------------
\* Actions — reduced user/OS events

\* --- finger accounting (abstract multi-touch) ---

FingerDown ==
  /\ fingers < maxFingers
  /\ fingers' = fingers + 1
  /\ IF InFullscreen /\ fingers + 1 >= 2
     THEN /\ pinching' = TRUE
          /\ suppressNav' = TRUE   \* enter pinch → arm suppress
     ELSE UNCHANGED <<pinching, suppressNav>>
  /\ UNCHANGED <<mode, zoomed, idx, maxCams, maxFingers>>

FingerUp ==
  /\ fingers > 0
  /\ fingers' = fingers - 1
  /\ IF pinching /\ fingers - 1 < 2
     THEN \* leave multi-touch: end pinch session, keep suppress, snap near-fit → Fit
          /\ pinching' = FALSE
          /\ suppressNav' = TRUE
          /\ zoomed' = IF zoomed THEN TRUE ELSE FALSE
          \* product snaps scale<1.08 to Fit; abstract: if was pinching toward fit,
          \* ZoomOut can clear zoomed; here FingerUp alone does not force Fit
     ELSE UNCHANGED <<pinching, suppressNav, zoomed>>
  /\ UNCHANGED <<mode, idx, maxCams, maxFingers>>

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
  /\ UNCHANGED <<idx, fingers, maxCams, maxFingers>>

\* Click fullscreen image → minimize (close overlay) to grid
\* User rule: click on full-screen image closes, regardless of zoom
ClickFullscreen ==
  /\ InFullscreen
  /\ fingers <= 1
  /\ ~pinching
  /\ mode' = "Grid"
  /\ zoomed' = FALSE
  /\ pinching' = FALSE
  /\ suppressNav' = FALSE
  /\ fingers' = 0
  /\ UNCHANGED <<idx, maxCams, maxFingers>>

\* --- zoom via pinch (abstract in / out) ---

\* Pinch increases zoom (must be fullscreen; typically 2 fingers)
PinchIn ==
  /\ InFullscreen
  /\ fingers >= 2
  /\ pinching = TRUE
  /\ zoomed' = TRUE
  /\ suppressNav' = TRUE
  /\ UNCHANGED <<mode, idx, fingers, pinching, maxCams, maxFingers>>

\* Pinch toward fit — may clear zoomed; must NOT change idx
PinchOut ==
  /\ InFullscreen
  /\ fingers >= 2
  /\ pinching = TRUE
  /\ zoomed' = FALSE   \* abstract: can reach Fit in one step (product continuous)
  /\ suppressNav' = TRUE
  /\ UNCHANGED <<mode, idx, fingers, pinching, maxCams, maxFingers>>

\* Explicit end-of-pinch settle (product: last finger up + settleZoomAfterGesture)
PinchEnd ==
  /\ InFullscreen
  /\ pinching = TRUE
  /\ fingers <= 1
  /\ pinching' = FALSE
  /\ suppressNav' = TRUE
  \* snap to Fit if product would (abstract nondet already via PinchOut)
  /\ UNCHANGED <<mode, zoomed, idx, fingers, maxCams, maxFingers>>

\* Clear suppress after cooldown (product: 450ms timer)
ClearSuppress ==
  /\ suppressNav = TRUE
  /\ ~pinching
  /\ fingers = 0
  /\ suppressNav' = FALSE
  /\ UNCHANGED <<mode, zoomed, idx, fingers, pinching, maxCams, maxFingers>>

\* Double-tap toggle Fit ↔ Zoomed (fullscreen only)
DoubleTap ==
  /\ InFullscreen
  /\ fingers <= 1
  /\ ~pinching
  /\ ~suppressNav
  /\ zoomed' = ~zoomed
  /\ UNCHANGED <<mode, idx, fingers, pinching, suppressNav, maxCams, maxFingers>>

\* --- gallery navigation (swipe) — only when allowed ---

SwipeNext ==
  /\ CanSwipe
  /\ idx < maxCams - 1
  /\ idx' = idx + 1
  /\ zoomed' = FALSE
  /\ UNCHANGED <<mode, fingers, pinching, suppressNav, maxCams, maxFingers>>

SwipePrev ==
  /\ CanSwipe
  /\ idx > 0
  /\ idx' = idx - 1
  /\ zoomed' = FALSE
  /\ UNCHANGED <<mode, fingers, pinching, suppressNav, maxCams, maxFingers>>

\* BAIT / bug models (not in Next_Good) ------------------------------------

\* Bug: allow swipe immediately after pinch (old product behavior)
SwipeNext_Buggy ==
  /\ AtFit
  /\ ~pinching
  \* missing: ~suppressNav
  /\ fingers <= 1
  /\ idx < maxCams - 1
  /\ idx' = idx + 1
  /\ zoomed' = FALSE
  /\ UNCHANGED <<mode, fingers, pinching, suppressNav, maxCams, maxFingers>>

\* Bug: pinch-out advances gallery
PinchOut_BuggyNav ==
  /\ InFullscreen
  /\ fingers >= 2
  /\ pinching = TRUE
  /\ zoomed' = FALSE
  /\ idx' = IF idx < maxCams - 1 THEN idx + 1 ELSE idx
  /\ suppressNav' = TRUE
  /\ UNCHANGED <<mode, fingers, pinching, maxCams, maxFingers>>

-----------------------------------------------------------------------------

Next_Good ==
  \/ FingerDown
  \/ FingerUp
  \/ ClickThumb
  \/ ClickFullscreen
  \/ PinchIn
  \/ PinchOut
  \/ PinchEnd
  \/ ClearSuppress
  \/ DoubleTap
  \/ SwipeNext
  \/ SwipePrev

Next_BuggySwipeAfterPinch ==
  \/ FingerDown
  \/ FingerUp
  \/ ClickThumb
  \/ ClickFullscreen
  \/ PinchIn
  \/ PinchOut
  \/ PinchEnd
  \/ ClearSuppress
  \/ DoubleTap
  \/ SwipeNext
  \/ SwipePrev
  \/ SwipeNext_Buggy

Next_BuggyPinchNav ==
  \/ FingerDown
  \/ FingerUp
  \/ ClickThumb
  \/ ClickFullscreen
  \/ PinchIn
  \/ PinchOut
  \/ PinchOut_BuggyNav
  \/ PinchEnd
  \/ ClearSuppress
  \/ DoubleTap
  \/ SwipeNext
  \/ SwipePrev

\* Default Next for TLC good path
Next == Next_Good

Spec == Init /\ [][Next]_<<mode, zoomed, idx, fingers, pinching, suppressNav, maxCams, maxFingers>>

-----------------------------------------------------------------------------
\* Safety invariants

\* Zoomed implies fullscreen
Inv_ZoomedImpliesFullscreen == ZoomedOnlyFullscreen

\* Pinch only in fullscreen
Inv_PinchImpliesFullscreen == PinchOnlyFullscreen

\* Pinching implies suppress (policy: always arm suppress when pinching)
Inv_PinchImpliesSuppress ==
  pinching => suppressNav

\* Cannot be pinching with fewer than 2 fingers (after reduction: pinching sticky until FingerUp/PinchEnd)
\* Allow pinching with fingers<2 briefly via PinchEnd path — product has one-finger leftover.
\* Strong form: pinching => fingers >= 1 \/ FALSE — soft:
Inv_PinchHasFingers ==
  pinching => fingers >= 1

\* If pinching with 2+ fingers, must not be able to swipe (gate)
Inv_NoSwipeWhilePinching ==
  pinching => ~CanSwipe

\* Suppress blocks swipe gate
Inv_SuppressBlocksSwipe ==
  suppressNav => ~CanSwipe

\* Grid never zoomed
Inv_GridNotZoomed ==
  InGrid => (~zoomed /\ ~pinching)

TypeOK == TypeInvariant /\ LidsStable

Safe ==
  /\ TypeOK
  /\ Inv_ZoomedImpliesFullscreen
  /\ Inv_PinchImpliesFullscreen
  /\ Inv_PinchImpliesSuppress
  /\ Inv_PinchHasFingers
  /\ Inv_NoSwipeWhilePinching
  /\ Inv_SuppressBlocksSwipe
  /\ Inv_GridNotZoomed

-----------------------------------------------------------------------------
\* Action properties as invariants over transitions (encoded as state inv + guards)
\* Temporal: idx only changes when CanSwipe held in pre-state of a nav action.
\* TLC: check via [][IdxChangeOnlyWhenCanSwipe]_vars

vars == <<mode, zoomed, idx, fingers, pinching, suppressNav, maxCams, maxFingers>>

IdxChangeOnlyWhenCanSwipe ==
  (idx' # idx) =>
    \* pre-state allowed nav (CanSwipe uses unprimed)
    (/\ AtFit /\ ~pinching /\ ~suppressNav /\ fingers <= 1)

\* Safety Spec (good path):
SpecSafe == Init /\ [][Next_Good]_vars /\ []Safe

\* Temporal: index changes only when CanSwipe held before the step
NavSafe == [][IdxChangeOnlyWhenCanSwipe]_vars

=============================================================================
