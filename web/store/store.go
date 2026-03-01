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
	weatherStationsById        map[int]*WeatherStation // Maps station Id -> weather station
	weatherStationsMu          sync.RWMutex
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
		roadConditions:      make(map[string][]RoadCondition),
		weatherStationsById: make(map[int]*WeatherStation),
		events:              make(map[string][]Event),
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

			imageBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
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

// StoreWeatherStationsById indexes weather stations by their Id for lookup by cameras
func (s *Store) StoreWeatherStationsById(stations []WeatherStation) {
	s.weatherStationsMu.Lock()
	defer s.weatherStationsMu.Unlock()

	m := make(map[int]*WeatherStation, len(stations))
	for i := range stations {
		m[stations[i].Id] = &stations[i]
	}
	s.weatherStationsById = m
	logger.Muted("Indexed %d weather stations by Id", len(m))
}

// GetWeatherStation returns the weather station data for a camera by its ID
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

	var stationId *int
	entry.Read(func(e *Entry) {
		if e.Camera != nil {
			stationId = e.Camera.WeatherStationId
		}
	})

	if stationId == nil {
		return nil
	}

	s.weatherStationsMu.RLock()
	defer s.weatherStationsMu.RUnlock()
	return s.weatherStationsById[*stationId]
}

// GetWeatherStationsForCanyon returns weather stations for all cameras in a canyon,
// acquiring the lock once instead of per-camera.
func (s *Store) GetWeatherStationsForCanyon(canyon *Canyon) map[string]*WeatherStation {
	if canyon == nil {
		return nil
	}

	s.imagesReady.Wait()

	// Collect weatherStationIds from cameras
	type lookup struct {
		cameraID  string
		stationId int
	}
	var lookups []lookup
	for _, cam := range canyon.Cameras {
		if cam.WeatherStationId == nil {
			continue
		}
		lookups = append(lookups, lookup{cameraID: cam.ID, stationId: *cam.WeatherStationId})
	}

	if len(lookups) == 0 {
		return nil
	}

	s.weatherStationsMu.RLock()
	defer s.weatherStationsMu.RUnlock()

	result := make(map[string]*WeatherStation)
	for _, l := range lookups {
		if station, exists := s.weatherStationsById[l.stationId]; exists {
			result[l.cameraID] = station
		}
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
