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

	"github.com/stefanpenner/lcc-live/logger"
	"github.com/stefanpenner/lcc-live/metrics"
)

const (
	// HTTP client timeout for fetching images
	httpClientTimeout = 5 * time.Second
	// Timeout for HEAD requests to check image changes
	headRequestTimeout = 2 * time.Second
	// Timeout for GET requests to fetch images
	getRequestTimeout = 2 * time.Second
	// User agent to mimic Chrome browser (helps with servers that block non-browser requests)
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// Store manages camera images and provides concurrent access
type Store struct {
	client                     *http.Client
	canyons                    *Canyons
	index                      map[string]*Entry // Maps camera ID -> Entry
	nameIndex                  map[string]*Entry // Maps camera slug -> Entry
	entries                    []*Entry
	mu                         sync.RWMutex
	imagesReady                sync.WaitGroup
	isWaitingOnFirstImageReady atomic.Bool
	syncCallback               func(duration time.Duration, changed, unchanged, errors int)
	syncCallbackMu             sync.Mutex
	roadConditions             map[string][]RoadCondition // Maps canyon -> road conditions
	roadConditionsMu           sync.RWMutex
	weatherStations            map[string]*WeatherStation // Maps camera SourceId -> weather station
	weatherStationsMu          sync.RWMutex
	allWeatherStations         []WeatherStation // Store all weather stations for re-matching
	allWeatherStationsMu       sync.RWMutex
	events                     map[string][]Event // Maps canyon -> events
	eventsMu                   sync.RWMutex
}

// Entry represents a single camera's cached data
type Entry struct {
	Camera      *Camera
	Image       *Image
	HTTPHeaders *HTTPHeaders
	ID          string
	mu          sync.RWMutex
}

// EntrySnapshot is an immutable snapshot of an Entry's state
type EntrySnapshot struct {
	Camera      *Camera
	Image       *Image
	HTTPHeaders *HTTPHeaders
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

func (s *Store) Read(fn func(*Store)) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fn(s)
}

func (s *Store) Write(fn func(*Store)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn(s)
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
		// Extract SourceId from UDOT camera URLs (e.g., https://udottraffic.utah.gov/map/Cctv/86277)
		if camera.SourceId == "" && strings.Contains(camera.Src, "udottraffic.utah.gov/map/Cctv/") {
			parts := strings.Split(camera.Src, "/Cctv/")
			if len(parts) > 1 {
				// Extract the number before the query string or end of URL
				sourceIdPart := strings.Split(parts[1], "?")[0]
				sourceIdPart = strings.Split(sourceIdPart, "#")[0]
				camera.SourceId = strings.TrimSpace(sourceIdPart)
			}
		}
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
		entries:            entries,
		index:              index,
		nameIndex:          nameIndex,
		canyons:            canyons,
		roadConditions:     make(map[string][]RoadCondition),
		weatherStations:    make(map[string]*WeatherStation),
		allWeatherStations: make([]WeatherStation, 0),
		events:             make(map[string][]Event),
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

// FetchImages fetches images for all cameras concurrently
// TODO: this should return a more detailed summary of what changed, so that we can:
// 1. provide a /status endpoint
// 2. provide "camera down" or "camera live" UI
// 3. provide image updates via push of some sort
func (s *Store) FetchImages(ctx context.Context) {
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
			contentLength := resp.ContentLength

			imageBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "read_body").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}
			etag := "\"" + strconv.FormatUint(xxhash.Sum64(imageBytes), 10) + "\""
			entry.Write(func(entry *Entry) {
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
	if s.isWaitingOnFirstImageReady.Load() {
		s.isWaitingOnFirstImageReady.Store(false)
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

// IsReady returns true if the store has completed its initial image fetch
// and is ready to serve requests. This is used by the healthcheck endpoint
// to ensure the application is fully initialized before accepting traffic.
func (s *Store) IsReady() bool {
	return !s.isWaitingOnFirstImageReady.Load()
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

// StoreAllWeatherStations stores all weather stations for later re-matching
func (s *Store) StoreAllWeatherStations(stations []WeatherStation) {
	s.allWeatherStationsMu.Lock()
	defer s.allWeatherStationsMu.Unlock()
	s.allWeatherStations = stations
}

// UpdateWeatherStation updates the weather station data for a camera SourceId
func (s *Store) UpdateWeatherStation(cameraSourceId string, station *WeatherStation) {
	if cameraSourceId == "" || station == nil {
		return
	}
	s.weatherStationsMu.Lock()
	defer s.weatherStationsMu.Unlock()
	s.weatherStations[cameraSourceId] = station
}

// UpdateCameraCoordinates updates camera coordinates from UDOT cameras data
// This is called with a map of camera ID (from UDOT) -> (lat, lon) pairs
// The camera ID is matched against the SourceId extracted from camera URLs
func (s *Store) UpdateCameraCoordinates(cameraIdToCoords map[string]struct {
	Lat float64
	Lon float64
}) {
	s.mu.RLock()
	entries := make([]*Entry, len(s.entries))
	copy(entries, s.entries)
	s.mu.RUnlock()

	updated := 0
	for _, entry := range entries {
		entry.Write(func(e *Entry) {
			if e.Camera == nil || e.Camera.SourceId == "" {
				return
			}

			// Match by SourceId (which is the camera ID from UDOT URL)
			coords, exists := cameraIdToCoords[e.Camera.SourceId]
			if !exists {
				return
			}

			// Update coordinates
			lat := coords.Lat
			lon := coords.Lon
			e.Camera.Latitude = &lat
			e.Camera.Longitude = &lon
			updated++
		})
	}
	logger.Muted("Updated coordinates for %d cameras", updated)

	// Re-match weather stations now that coordinates are available
	s.allWeatherStationsMu.RLock()
	stations := make([]WeatherStation, len(s.allWeatherStations))
	copy(stations, s.allWeatherStations)
	s.allWeatherStationsMu.RUnlock()

	if len(stations) > 0 {
		logger.Muted("Re-matching %d weather stations after coordinate update", len(stations))
		s.MatchWeatherStationsByCoordinates(stations)
	} else {
		logger.Muted("No weather stations stored yet for re-matching")
	}
}

// MatchWeatherStationsByCoordinates matches weather stations to cameras by lat/long coordinates
// Uses a threshold of 0.001 degrees (~111 meters) for matching
func (s *Store) MatchWeatherStationsByCoordinates(stations []WeatherStation) {
	const coordThreshold = 0.001 // ~111 meters

	s.mu.RLock()
	entries := make([]*Entry, len(s.entries))
	copy(entries, s.entries)
	s.mu.RUnlock()

	matched := 0
	for i := range stations {
		station := &stations[i]
		if station.Latitude == nil || station.Longitude == nil {
			continue
		}

		// Find matching camera by coordinates
		for _, entry := range entries {
			entry.Read(func(e *Entry) {
				if e.Camera == nil {
					return
				}
				if e.Camera.Latitude == nil || e.Camera.Longitude == nil {
					return
				}

				latDiff := *e.Camera.Latitude - *station.Latitude
				if latDiff < 0 {
					latDiff = -latDiff
				}
				lonDiff := *e.Camera.Longitude - *station.Longitude
				if lonDiff < 0 {
					lonDiff = -lonDiff
				}

				if latDiff < coordThreshold && lonDiff < coordThreshold {
					// Match found - use camera ID as key
					s.UpdateWeatherStation(e.Camera.ID, station)
					matched++
					logger.Muted("Matched weather station %s (%s) to camera %s (%s) by coordinates",
						station.StationName, e.Camera.Alt, e.Camera.ID, e.Camera.Alt)
				}
			})
		}
	}
	logger.Muted("Matched %d weather stations to cameras by coordinates", matched)
}

// GetWeatherStation returns the weather station data for a camera by its ID
// It first tries to match by SourceId, then by camera ID (for coordinate-based matching)
func (s *Store) GetWeatherStation(cameraID string) *WeatherStation {
	s.imagesReady.Wait()

	// Get the camera entry
	entry, exists := s.index[cameraID]
	if !exists {
		entry, exists = s.nameIndex[cameraID]
	}
	if !exists {
		return nil
	}

	var sourceId string
	var cameraIDForMatch string
	entry.Read(func(e *Entry) {
		if e.Camera != nil {
			sourceId = e.Camera.SourceId
			cameraIDForMatch = e.Camera.ID
		}
	})

	s.weatherStationsMu.RLock()
	defer s.weatherStationsMu.RUnlock()

	// First try by SourceId
	if sourceId != "" {
		if station, exists := s.weatherStations[sourceId]; exists {
			return station
		}
	}

	// Then try by camera ID (for coordinate-based matching)
	if station, exists := s.weatherStations[cameraIDForMatch]; exists {
		return station
	}

	return nil
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
