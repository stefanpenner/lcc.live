package store

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/stefanpenner/lcc-live/web/logger"
	"github.com/stefanpenner/lcc-live/web/metrics"
)

const (
	// HTTP client timeout for fetching images
	httpClientTimeout = 5 * time.Second
	// Timeout for HEAD requests to check image changes
	headRequestTimeout = 2 * time.Second
	// Timeout for GET requests to fetch images
	getRequestTimeout = 2 * time.Second
	// Maximum image size to prevent OOM from unexpectedly large responses
	maxImageSize = 10 * 1024 * 1024 // 10MB
	// User agent to mimic Chrome browser (helps with servers that block non-browser requests)
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// Store manages camera images and provides concurrent access.
//
// Concurrency model:
//   - Camera index maps are immutable after NewStore.
//   - Each Entry has its own RWMutex; values are replaced (pointer swap), not mutated in place.
//   - Readers use ShallowSnapshot / Get; treat snapshots as frozen.
//   - FetchImages is single-flight: overlapping calls skip.
//   - Ready gate: CAS on isWaitingOnFirstImageReady then imagesReady.Done() at most once.
//   - Dual ETag: HTTPHeaders.ETag = upstream HEAD validator (skip GET);
//     Image.ETag = content hash (served to clients).
type Store struct {
	client                     *http.Client
	canyons                    *Canyons
	index                      map[string]*Entry // Maps camera ID -> Entry
	nameIndex                  map[string]*Entry // Maps camera slug -> Entry
	entries                    []*Entry
	imagesReady                sync.WaitGroup
	isWaitingOnFirstImageReady atomic.Bool
	fetchInFlight              atomic.Bool
	syncCallback               func(duration time.Duration, changed, unchanged, errors int)
	syncCallbackMu             sync.Mutex
	roadConditions             map[string][]RoadCondition // Maps canyon -> road conditions
	roadConditionsMu           sync.RWMutex
	weatherStationsById        map[int]*WeatherStation    // UDOT station Id -> weather station
	weatherStationsByStid      map[string]*WeatherStation // MesoWest/Synoptic/NWS stid -> weather station
	weatherStationsMu          sync.RWMutex
	events                     map[string][]Event // Maps canyon -> events
	eventsMu                   sync.RWMutex
	avalancheDanger            *AvalancheDanger
	avalancheDangerMu          sync.RWMutex
	altaStatus                 *AltaStatus
	altaStatusMu               sync.RWMutex
}

// Entry represents a single camera's cached data
type Entry struct {
	Camera      *Camera
	Image       *Image
	HTTPHeaders *HTTPHeaders
	FetchedAt   time.Time
	ID          string
	mu          sync.RWMutex
}

// EntrySnapshot is an immutable snapshot of an Entry's state
type EntrySnapshot struct {
	Camera      *Camera
	Image       *Image
	HTTPHeaders *HTTPHeaders
	FetchedAt   time.Time
	ID          string
	ETag        string
}

// ShallowSnapshot returns a shallow snapshot of the entry's current state
// Concurrency Model Overview:
//
// - The Store is immutable after initialization except for its entry values.
// - Locking is managed at the entry level using RWMutex.
//
// To enable concurrent access to Entry structs, we follow this pattern:
//  1. Each Entry struct is mutable and contains its own RWMutex, but remains internal to the Store.
//  2. Each Entry holds references only to immutable values. When a value changes,
//     the original remains unchanged. A new value is created and then assigned to the stable Entry.
//  3. External access to entries is provided via snapshots of the Entry object.
//  4. Consumers treat the provided EntrySnapshot (and its descendant structs) as "deep frozen",
//     following a handshake agreement.
//
// TODO: Consider making private members and public getters for EntrySnapshot and its descendant structs.
func (e *Entry) ShallowSnapshot() EntrySnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Snapshot the member pointers, and drop the mutex.
	// The members are immutable, so this works great:
	// * we don't expose any mutable state, which includes mutex's and all the locking complexity
	// * we don't need to copy the image bytes, as all consumers of the camera will share the same underlying image bytes.
	// * once the images changes, the entry's image pointer is updated, but all existing EntrySnpashots remain unchanged.
	return EntrySnapshot{
		Camera:      e.Camera,
		Image:       e.Image,
		HTTPHeaders: e.HTTPHeaders,
		FetchedAt:   e.FetchedAt,
		ID:          e.ID,
	}
}

func (e *Entry) Read(fn func(*Entry)) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fn(e)
}

func (e *Entry) Write(fn func(*Entry)) {
	e.mu.Lock()
	defer e.mu.Unlock()

	fn(e)
}

// NewStoreFromFile creates a new store by loading canyon data from a file
func NewStoreFromFile(f fs.FS, filepath string) (*Store, error) {
	canyons := &Canyons{}
	err := canyons.Load(f, filepath)
	if err != nil {
		return nil, err
	}

	return NewStore(canyons), err
}

// NewStore creates a new store with the given canyons configuration
func NewStore(canyons *Canyons) *Store {
	// store initialization doesn't need to be threadsafe, as the store is only
	// accessed from a single thread during intializations.
	//
	// Only subsequent access must be
	//
	index := make(map[string]*Entry)
	nameIndex := make(map[string]*Entry)
	entries := []*Entry{}

	createEntry := func(camera *Camera) {
		camera.ID = base64.StdEncoding.EncodeToString([]byte(camera.Src))
		entry := &Entry{
			Camera:      camera,
			Image:       &Image{},
			HTTPHeaders: &HTTPHeaders{},
			ID:          camera.ID,
			mu:          sync.RWMutex{},
		}
		index[camera.ID] = entry

		// Also index by slug if camera has a name
		if camera.Alt != "" {
			slug := slugify(camera.Alt)
			if slug == "" {
				// Empty slug is invalid - camera name slugifies to nothing
				panic(fmt.Sprintf("camera '%s' (ID: %s) has name that produces empty slug", camera.Alt, camera.ID))
			}

			// Check for slug collisions
			if existingEntry, exists := nameIndex[slug]; exists {
				// Slug collision detected
				existingCamera := existingEntry.Camera
				panic(fmt.Sprintf("slug collision: cameras '%s' (ID: %s) and '%s' (ID: %s) both slugify to '%s'",
					existingCamera.Alt, existingCamera.ID, camera.Alt, camera.ID, slug))
			}

			// Check if slug collides with any other camera's ID
			if existingEntry, idCollision := index[slug]; idCollision && existingEntry != entry {
				existingCamera := existingEntry.Camera
				panic(fmt.Sprintf("slug collision: camera '%s' (ID: %s) has slug '%s' that matches another camera's ID (camera '%s', ID: %s)",
					camera.Alt, camera.ID, slug, existingCamera.Alt, existingCamera.ID))
			}

			nameIndex[slug] = entry
		}

		entries = append(entries, entry)
	}

	// Process status cameras if present
	if canyons.LCC.Status.Src != "" {
		canyons.LCC.Status.Canyon = "LCC" //nolint:goconst // Canyon name used for clarity
		createEntry(&canyons.LCC.Status)
	}
	if canyons.BCC.Status.Src != "" {
		canyons.BCC.Status.Canyon = "BCC" //nolint:goconst // Canyon name used for clarity
		createEntry(&canyons.BCC.Status)
	}

	// Process regular cameras
	for i := range canyons.LCC.Cameras {
		canyons.LCC.Cameras[i].Canyon = "LCC" //nolint:goconst // Canyon name used for clarity
		createEntry(&canyons.LCC.Cameras[i])
	}
	for i := range canyons.BCC.Cameras {
		canyons.BCC.Cameras[i].Canyon = "BCC" //nolint:goconst // Canyon name used for clarity
		createEntry(&canyons.BCC.Cameras[i])
	}

	// Create HTTP client with custom TLS config to handle camera servers
	// with self-signed or non-standard certificates
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // G402: Required for external camera servers with self-signed certs
		},
	}

	store := &Store{
		entries:             entries,
		index:               index,
		nameIndex:           nameIndex,
		canyons:             canyons,
		roadConditions:      make(map[string][]RoadCondition),
		weatherStationsById:   make(map[int]*WeatherStation),
		weatherStationsByStid: make(map[string]*WeatherStation),
		events:                make(map[string][]Event),
		client: &http.Client{
			Timeout:   httpClientTimeout,
			Transport: transport,
		},
	}

	store.imagesReady.Add(1) // wait for first signal
	store.isWaitingOnFirstImageReady.Store(true)

	// Set metrics
	metrics.StoreEntriesTotal.Set(float64(len(entries)))
	metrics.CamerasTotal.WithLabelValues("LCC").Set(float64(len(canyons.LCC.Cameras)))
	metrics.CamerasTotal.WithLabelValues("BCC").Set(float64(len(canyons.BCC.Cameras)))
	metrics.ImagesReady.Set(0)

	return store
}

// Canyon returns the canyon with the given name
func (s *Store) Canyon(canyon string) *Canyon {
	switch canyon {
	case "LCC":
		return &s.canyons.LCC
	case "BCC":
		return &s.canyons.BCC
	default:
		panic("invalid canyon: must be either 'LCC' or 'BCC'")
	}
}

// FetchImages fetches images for all cameras concurrently.
// Single-flight: if a fetch is already running, this call returns immediately.
// TODO: this should return a more detailed summary of what changed, so that we can:
// 1. provide a /status endpoint
// 2. provide "camera down" or "camera live" UI
// 3. provide image updates via push of some sort
func (s *Store) FetchImages(ctx context.Context) {
	if !s.fetchInFlight.CompareAndSwap(false, true) {
		return
	}
	defer s.fetchInFlight.Store(false)

	// Start timing for metrics
	timer := metrics.ImageFetchDuration
	startTime := time.Now()

	var wg sync.WaitGroup
	var (
		changedCount   int32
		errorCount     int32
		unchangedCount int32
	)

	for i := range s.entries {
		entry := s.entries[i]

		if entry.Camera.Kind == "iframe" {
			continue
		}
		wg.Add(1)

		go func(entry *Entry) {
			defer wg.Done()

			// Track concurrent fetches
			metrics.ConcurrentFetches.Inc()
			defer metrics.ConcurrentFetches.Dec()

			// Check if context is already cancelled before starting work
			if ctx.Err() != nil {
				return
			}

			// lock while reading
			// let's simply copy the structs we need for the long-lived function,
			// then unlock immediately after copying when we update, we will relock
			var src string
			var headers HTTPHeaders
			var camera *Camera

			entry.Read(func(entry *Entry) {
				src = entry.Camera.Src // Copy
				camera = entry.Camera  // Copy pointer (safe to use for reading)
				// TODO: explore option of an explicit copy via Copy() or Snapshot(), vs the current implicit approach
				headers = *entry.HTTPHeaders // Copy
			})

			// Extract origin and camera info for metrics
			origin := metrics.ExtractOrigin(src)
			cameraName := camera.Alt
			if cameraName == "" {
				cameraName = camera.ID
			}
			canyon := camera.Canyon

			// Start timing for per-camera metrics
			cameraStartTime := time.Now()

			headCtx, cancel := context.WithTimeout(ctx, headRequestTimeout)
			defer cancel()
			headReq, err := http.NewRequestWithContext(headCtx, "HEAD", src, nil)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				metrics.ImageFetchErrorsTotal.WithLabelValues("head_request").Inc()
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "head_request").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}

			// Set User-Agent to mimic Chrome browser
			headReq.Header.Set("User-Agent", userAgent)

			headResp, err := s.client.Do(headReq)
			if err != nil {
				// Check if error is due to context cancellation
				if ctx.Err() != nil {
					return
				}
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "connection").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}

			_ = headResp.Body.Close()

			newETag := headResp.Header.Get("ETag")

			if newETag != "" && newETag == headers.ETag {
				atomic.AddInt32(&unchangedCount, 1)
				// Record metrics for unchanged image
				cameraDuration := time.Since(cameraStartTime).Seconds()
				metrics.CameraFetchDuration.WithLabelValues(cameraName, canyon).Observe(cameraDuration)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "unchanged").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "success").Inc()
				metrics.OriginFetchDuration.WithLabelValues(origin).Observe(cameraDuration)
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(1)
				return
			}

			getCtx, cancel := context.WithTimeout(ctx, getRequestTimeout)
			defer cancel()
			getReq, err := http.NewRequestWithContext(getCtx, "GET", src, nil)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "get_request").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}

			// Set User-Agent to mimic Chrome browser
			getReq.Header.Set("User-Agent", userAgent)

			resp, err := s.client.Do(getReq)
			if err != nil {
				// Check if error is due to context cancellation
				if ctx.Err() != nil {
					return
				}
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "connection").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "bad_status").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}

			contentType := resp.Header.Get("Content-Type")

			imageBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "read_body").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}
			// Always use actual body length — upstream Content-Length can be wrong or -1.
			contentLength := int64(len(imageBytes))
			// Image.ETag = content hash (clients / If-None-Match).
			// HTTPHeaders.ETag = upstream HEAD ETag (next-cycle skip).
			etag := "\"" + strconv.FormatUint(xxhash.Sum64(imageBytes), 10) + "\""
			entry.Write(func(entry *Entry) {
				// Only update FetchedAt when image content actually changed
				if entry.Image.ETag != etag {
					entry.FetchedAt = time.Now()
				}
				// replace headers
				entry.HTTPHeaders = &HTTPHeaders{
					Status:        http.StatusOK,
					ContentType:   contentType,
					ContentLength: contentLength,
					ETag:          newETag,
				}
				// replace image
				entry.Image = &Image{
					Bytes: imageBytes,
					ETag:  etag,
					Src:   entry.Image.Src,
				}
			})
			atomic.AddInt32(&changedCount, 1)

			// Record success metrics
			cameraDuration := time.Since(cameraStartTime).Seconds()
			imageSize := float64(len(imageBytes))

			metrics.CameraFetchDuration.WithLabelValues(cameraName, canyon).Observe(cameraDuration)
			metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "success").Inc()
			metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(1)
			metrics.CameraLastSuccessTimestamp.WithLabelValues(cameraName, canyon).SetToCurrentTime()
			metrics.CameraImageSizeBytes.WithLabelValues(cameraName, canyon).Set(imageSize)

			metrics.OriginFetchTotal.WithLabelValues(origin, "success").Inc()
			metrics.OriginFetchDuration.WithLabelValues(origin).Observe(cameraDuration)
			metrics.ImageFetchSizeBytes.Observe(imageSize)
		}(entry)
	}
	wg.Wait()
	// CAS: Done() at most once (see web/tla/StoreReady.tla)
	if s.isWaitingOnFirstImageReady.CompareAndSwap(true, false) {
		s.imagesReady.Done()
		metrics.ImagesReady.Set(1)
	}
	duration := time.Since(startTime)

	// Record metrics
	timer.Observe(duration.Seconds())
	metrics.StoreFetchCyclesTotal.Inc()
	metrics.ImageFetchTotal.WithLabelValues("success").Add(float64(changedCount))
	metrics.ImageFetchTotal.WithLabelValues("unchanged").Add(float64(unchangedCount))
	metrics.ImageFetchTotal.WithLabelValues("error").Add(float64(errorCount))
	metrics.FetchCycleDurationSeconds.Set(duration.Seconds())

	// Update memory usage metrics
	metrics.RecordMemoryUsage()

	// Print summary
	summary := logger.FetchSummary{
		Duration:  duration,
		Changed:   int(changedCount),
		Unchanged: int(unchangedCount),
		Errors:    int(errorCount),
		Total:     int(changedCount + unchangedCount + errorCount),
	}
	summary.Print()

	// Call sync callback if set
	s.syncCallbackMu.Lock()
	if s.syncCallback != nil {
		s.syncCallback(duration, int(changedCount), int(unchangedCount), int(errorCount))
	}
	s.syncCallbackMu.Unlock()
}

// SetSyncCallback sets a callback to be called after each sync
func (s *Store) SetSyncCallback(cb func(duration time.Duration, changed, unchanged, errors int)) {
	s.syncCallbackMu.Lock()
	s.syncCallback = cb
	s.syncCallbackMu.Unlock()
}

// IsReady returns true if the store has completed its initial image fetch cycle
// (success or total failure). Get() unblocks when ready.
// Healthcheck also requires HasAnyLiveImage() — see server health route.
func (s *Store) IsReady() bool {
	return !s.isWaitingOnFirstImageReady.Load()
}

// HasAnyLiveImage reports whether any non-iframe camera has a successful image.
func (s *Store) HasAnyLiveImage() bool {
	for _, entry := range s.entries {
		if entry.Camera != nil && entry.Camera.Kind == "iframe" {
			continue
		}
		live := false
		entry.Read(func(e *Entry) {
			live = e.HTTPHeaders != nil && e.HTTPHeaders.Status == http.StatusOK &&
				e.Image != nil && len(e.Image.Bytes) > 0
		})
		if live {
			return true
		}
	}
	return false
}

// Get retrieves a snapshot of the camera entry with the given ID
// slugify converts a camera name to a URL-safe slug
func slugify(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces and common separators with hyphens
	slug = regexp.MustCompile(`[\s_]+`).ReplaceAllString(slug, "-")
	// Remove all non-alphanumeric characters except hyphens
	slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "")
	// Replace multiple consecutive hyphens with a single hyphen
	slug = regexp.MustCompile(`-+`).ReplaceAllString(slug, "-")
	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")
	return slug
}

func (s *Store) Get(cameraID string) (EntrySnapshot, bool) {
	s.imagesReady.Wait()

	// First try direct ID lookup
	entry, exists := s.index[cameraID]
	if exists {
		return entry.ShallowSnapshot(), true
	}

	// Then try slug-based lookup
	entry, exists = s.nameIndex[cameraID]
	if exists {
		return entry.ShallowSnapshot(), true
	}

	return EntrySnapshot{}, false
}

// UpdateRoadConditions updates the road conditions for a canyon
func (s *Store) UpdateRoadConditions(canyon string, conditions []RoadCondition) {
	s.roadConditionsMu.Lock()
	defer s.roadConditionsMu.Unlock()
	s.roadConditions[canyon] = conditions
}

// GetRoadConditions returns the current road conditions for a canyon
func (s *Store) GetRoadConditions(canyon string) []RoadCondition {
	s.roadConditionsMu.RLock()
	defer s.roadConditionsMu.RUnlock()
	conditions, exists := s.roadConditions[canyon]
	if !exists {
		return nil
	}
	// Return a copy to avoid external modification
	result := make([]RoadCondition, len(conditions))
	copy(result, conditions)
	return result
}

// StoreWeatherStationsById indexes UDOT weather stations by their Id for lookup by cameras.
// Each station is copied so the caller's slice can be discarded safely.
func (s *Store) StoreWeatherStationsById(stations []WeatherStation) {
	s.weatherStationsMu.Lock()
	defer s.weatherStationsMu.Unlock()

	m := make(map[int]*WeatherStation, len(stations))
	for i := range stations {
		cp := stations[i]
		m[cp.Id] = &cp
	}
	s.weatherStationsById = m
	logger.Muted("Indexed %d weather stations by Id", len(m))
}

// StoreWeatherStationsByStid indexes MesoWest/Synoptic/NWS stations by stid.
// Merges into the existing map so a partial poll does not wipe other stids.
func (s *Store) StoreWeatherStationsByStid(stations []WeatherStation) {
	s.weatherStationsMu.Lock()
	defer s.weatherStationsMu.Unlock()

	if s.weatherStationsByStid == nil {
		s.weatherStationsByStid = make(map[string]*WeatherStation)
	}
	for i := range stations {
		cp := stations[i]
		// StationName carries stid for map key when Source tags it; prefer explicit
		// Id=0 stations keyed via CameraSourceId or name prefix "STID:".
		stid := ""
		if cp.CameraSourceId != nil && *cp.CameraSourceId != "" {
			stid = *cp.CameraSourceId
		}
		if stid == "" {
			continue
		}
		s.weatherStationsByStid[stid] = &cp
	}
	logger.Muted("Indexed %d weather stations by stid (total %d)", len(stations), len(s.weatherStationsByStid))
}

// SynopticStids returns the unique non-empty synopticStid values from all cameras.
func (s *Store) SynopticStids() []string {
	s.imagesReady.Wait()
	seen := make(map[string]struct{})
	var out []string
	for _, e := range s.entries {
		e.Read(func(entry *Entry) {
			if entry.Camera == nil || entry.Camera.SynopticStid == nil {
				return
			}
			stid := *entry.Camera.SynopticStid
			if stid == "" {
				return
			}
			if _, ok := seen[stid]; ok {
				return
			}
			seen[stid] = struct{}{}
			out = append(out, stid)
		})
	}
	return out
}

// resolveStation prefers mountain MesoWest/Synoptic/NWS when available, else UDOT RWIS.
// Caller holds weatherStationsMu.
func (s *Store) resolveStation(udotID *int, stid *string) *WeatherStation {
	if stid != nil && *stid != "" {
		if station, ok := s.weatherStationsByStid[*stid]; ok && station != nil {
			cp := *station
			return &cp
		}
	}
	if udotID != nil {
		if station, ok := s.weatherStationsById[*udotID]; ok && station != nil {
			cp := *station
			return &cp
		}
	}
	return nil
}

// GetWeatherStation returns a copy of the weather station for a camera, or nil.
// Prefers MesoWest/Synoptic/NWS stid when available; falls back to UDOT RWIS.
// The returned pointer is not aliased into the store map.
func (s *Store) GetWeatherStation(cameraID string) *WeatherStation {
	s.imagesReady.Wait()

	entry, exists := s.index[cameraID]
	if !exists {
		entry, exists = s.nameIndex[cameraID]
	}
	if !exists {
		return nil
	}

	var udotID *int
	var stid *string
	entry.Read(func(e *Entry) {
		if e.Camera != nil {
			udotID = e.Camera.WeatherStationId
			stid = e.Camera.SynopticStid
		}
	})

	s.weatherStationsMu.RLock()
	defer s.weatherStationsMu.RUnlock()
	return s.resolveStation(udotID, stid)
}

// GetWeatherStationsForCanyon returns copies of weather stations for cameras in a canyon,
// acquiring the lock once instead of per-camera.
// Prefers mountain stid when available; otherwise UDOT RWIS.
func (s *Store) GetWeatherStationsForCanyon(canyon *Canyon) map[string]*WeatherStation {
	if canyon == nil {
		return nil
	}

	s.imagesReady.Wait()

	s.weatherStationsMu.RLock()
	defer s.weatherStationsMu.RUnlock()

	result := make(map[string]*WeatherStation)
	for _, cam := range canyon.Cameras {
		if st := s.resolveStation(cam.WeatherStationId, cam.SynopticStid); st != nil {
			result[cam.ID] = st
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// UpdateEvents updates the events for a canyon
func (s *Store) UpdateEvents(canyon string, events []Event) {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	s.events[canyon] = events
}

// GetEvents returns the current events for a canyon
func (s *Store) GetEvents(canyon string) []Event {
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()
	events, exists := s.events[canyon]
	if !exists {
		return nil
	}
	// Return a copy to avoid external modification
	result := make([]Event, len(events))
	copy(result, events)
	return result
}

// UpdateAvalancheDanger stores the global UAC Salt Lake danger rating.
func (s *Store) UpdateAvalancheDanger(d AvalancheDanger) {
	s.avalancheDangerMu.Lock()
	defer s.avalancheDangerMu.Unlock()
	cp := d
	s.avalancheDanger = &cp
}

// GetAvalancheDanger returns a copy of the current avalanche danger, or nil.
func (s *Store) GetAvalancheDanger() *AvalancheDanger {
	s.avalancheDangerMu.RLock()
	defer s.avalancheDangerMu.RUnlock()
	if s.avalancheDanger == nil {
		return nil
	}
	cp := *s.avalancheDanger
	return &cp
}

// UpdateAltaStatus stores Alta parking/road status.
func (s *Store) UpdateAltaStatus(st AltaStatus) {
	s.altaStatusMu.Lock()
	defer s.altaStatusMu.Unlock()
	cp := st
	s.altaStatus = &cp
}

// GetAltaStatus returns a copy of Alta status, or nil.
func (s *Store) GetAltaStatus() *AltaStatus {
	s.altaStatusMu.RLock()
	defer s.altaStatusMu.RUnlock()
	if s.altaStatus == nil {
		return nil
	}
	cp := *s.altaStatus
	return &cp
}
