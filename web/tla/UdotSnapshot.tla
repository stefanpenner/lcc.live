--------------------------- MODULE UdotSnapshot ----------------------------
\* Models UDOT poller write + HTTP read for road conditions / events / weather.
\*
\* Product:
\*   UpdateRoadConditions: lock; map[canyon] = slice (no copy in)
\*   GetRoadConditions: lock; copy slice out
\*   StoreWeatherStationsById: lock; replace whole map with pointers into new slice
\*   GetWeatherStation: returns *WeatherStation without copy (alias into map)
\*
\* Plain-language data gen:
\*   gen N — generation counter; readers should only see a complete gen
\*
\* Gaps:
\*   - Weather station pointer escape: reader holds *T across map replace
\*     (OK if T immutable; product treats as frozen — handshake only)
\*   - Three independent mutexes (road / weather / events) → cross-domain
\*     torn snapshot on canyon page (road gen 5 + weather gen 3)
\*   - Filter can assign one condition to both LCC and BCC (overlapping match)
\*   - 304 path: nil result leaves store gen unchanged (correct)
\*   - Client etag map shared across three pollers (safe: different endpoint keys)

EXTENDS Naturals, FiniteSets

CONSTANTS
  MaxGen,
  Canyons      \* {LCC, BCC}

ASSUME MaxGen \in Nat /\ MaxGen >= 1

VARIABLES
  roadGen,       \* [canyon -> Nat] generation installed
  weatherGen,    \* Nat global (map replace)
  eventsGen,     \* [canyon -> Nat]
  \* in-flight poll
  pollKind,      \* "none" | "road" | "weather" | "events"
  pollFetched,   \* Nat gen being installed (0 = 304 / skip)
  \* reader observations (last seen) for torn check
  lastRoad,      \* [canyon -> Nat]
  lastWeather,
  lastEvents,
  maxGen,
  canyons

Canyon == canyons
Kind == {"none", "road", "weather", "events"}

TypeInvariant ==
  /\ roadGen \in [Canyon -> 0..maxGen]
  /\ weatherGen \in 0..maxGen
  /\ eventsGen \in [Canyon -> 0..maxGen]
  /\ pollKind \in Kind
  /\ pollFetched \in 0..maxGen
  /\ lastRoad \in [Canyon -> 0..maxGen]
  /\ lastWeather \in 0..maxGen
  /\ lastEvents \in [Canyon -> 0..maxGen]
  /\ maxGen = MaxGen
  /\ canyons = Canyons

Init ==
  /\ maxGen = MaxGen
  /\ canyons = Canyons
  /\ roadGen = [c \in Canyon |-> 0]
  /\ weatherGen = 0
  /\ eventsGen = [c \in Canyon |-> 0]
  /\ pollKind = "none"
  /\ pollFetched = 0
  /\ lastRoad = [c \in Canyon |-> 0]
  /\ lastWeather = 0
  /\ lastEvents = [c \in Canyon |-> 0]

-----------------------------------------------------------------------------

StartPoll(k) ==
  /\ pollKind = "none"
  /\ k \in Kind \ {"none"}
  /\ pollKind' = k
  \* Adversary: new gen or 0 (304)
  /\ pollFetched' \in 0..maxGen
  /\ UNCHANGED <<roadGen, weatherGen, eventsGen, lastRoad, lastWeather,
                 lastEvents, maxGen, canyons>>

CommitPoll ==
  /\ pollKind # "none"
  /\ IF pollFetched = 0
     THEN \* 304 — no store write
          UNCHANGED <<roadGen, weatherGen, eventsGen>>
     ELSE
          IF pollKind = "road"
          THEN /\ roadGen' = [c \in Canyon |-> pollFetched]
               /\ UNCHANGED <<weatherGen, eventsGen>>
          ELSE IF pollKind = "weather"
          THEN /\ weatherGen' = pollFetched
               /\ UNCHANGED <<roadGen, eventsGen>>
          ELSE /\ eventsGen' = [c \in Canyon |-> pollFetched]
               /\ UNCHANGED <<roadGen, weatherGen>>
  /\ pollKind' = "none"
  /\ pollFetched' = 0
  /\ UNCHANGED <<lastRoad, lastWeather, lastEvents, maxGen, canyons>>

\* Canyon page read: samples road + weather + events without one global lock
\* (product: three separate mutex acquisitions → possible torn gens)
ReadCanyonPage(c) ==
  /\ c \in Canyon
  /\ lastRoad' = [lastRoad EXCEPT ![c] = roadGen[c]]
  /\ lastWeather' = weatherGen
  /\ lastEvents' = [lastEvents EXCEPT ![c] = eventsGen[c]]
  /\ UNCHANGED <<roadGen, weatherGen, eventsGen, pollKind, pollFetched,
                 maxGen, canyons>>

Next ==
  \/ \E k \in Kind \ {"none"}: StartPoll(k)
  \/ CommitPoll
  \/ \E c \in Canyon: ReadCanyonPage(c)

Spec == Init /\ [][Next]_<<roadGen, weatherGen, eventsGen, pollKind, pollFetched,
                           lastRoad, lastWeather, lastEvents, maxGen, canyons>>

-----------------------------------------------------------------------------
\* Invariants

\* Per-domain gens only increase or stay (replace with same gen allowed)
\* Abstract: gen 0 = empty; non-zero installs are complete snapshots
RoadComplete ==
  \A c \in Canyon: roadGen[c] \in 0..maxGen

\* STRONG (product does NOT provide): atomic multi-domain snapshot
\* If lastRoad and lastWeather both non-zero, they came from same poll wave —
\* FALSE because independent pollers / independent locks.
CoherentPage ==
  \A c \in Canyon:
    (lastRoad[c] # 0 /\ lastWeather # 0) => (lastRoad[c] = lastWeather)

\* Weaker: each field is a value that was once committed (no partial domain)
\* True by construction of CommitPoll (whole map replace)

=============================================================================
