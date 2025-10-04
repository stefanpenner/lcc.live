package store

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/stefanpenner/lcc-live/metrics"
	"github.com/stefanpenner/lcc-live/style"
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

type Store struct {
	client                     *http.Client
	canyons                    *Canyons
	path                       string
	index                      map[string]*Entry
	entries                    []*Entry
	mu                         sync.RWMutex
	imagesReady                sync.WaitGroup
	isWaitingOnFirstImageReady atomic.Bool
}

type Entry struct {
	Camera      *Camera
	Image       *Image
	HTTPHeaders *HTTPHeaders
	ID          string
	mu          sync.RWMutex
}

type EntrySnapshot struct {
	Camera      *Camera
	Image       *Image
	HTTPHeaders *HTTPHeaders
	ID          string
	ETag        string
}

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

func NewStoreFromFile(f fs.FS, filepath string) (*Store, error) {
	canyons := &Canyons{}
	err := canyons.Load(f, filepath)
	if err != nil {
		return nil, err
	}

	return NewStore(canyons), err
}

func NewStore(canyons *Canyons) *Store {
	// store initialization doesn't need to be threadsafe, as the store is only
	// accessed from a single thread during intializations.
	//
	// Only subsequent access must be
	//
	index := make(map[string]*Entry)
	entries := []*Entry{}

	createEntry := func(camera *Camera) *Entry {
		camera.ID = base64.StdEncoding.EncodeToString([]byte(camera.Src))
		entry := &Entry{
			Camera:      camera,
			Image:       &Image{},
			HTTPHeaders: &HTTPHeaders{},
			mu:          sync.RWMutex{},
		}
		index[camera.ID] = entry
		entries = append(entries, entry)
		return entry
	}

	// Process status cameras if present
	if canyons.LCC.Status.Src != "" {
		createEntry(&canyons.LCC.Status)
	}
	if canyons.BCC.Status.Src != "" {
		createEntry(&canyons.BCC.Status)
	}

	// Process regular cameras
	for i := range canyons.LCC.Cameras {
		createEntry(&canyons.LCC.Cameras[i])
	}
	for i := range canyons.BCC.Cameras {
		createEntry(&canyons.BCC.Cameras[i])
	}

	// Create HTTP client with custom TLS config to handle camera servers
	// with self-signed or non-standard certificates
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Skip certificate verification for camera servers
		},
	}

	store := &Store{
		entries: entries,
		index:   index,
		canyons: canyons,
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

// TODO: this should return a more defailt summary of what changed, so that we can:
// 1. provide a /status endpoint
// 2. provide "camera down" or "camera live" UI
// 2. provide image updates via push of some sort
func (s *Store) FetchImages(ctx context.Context) {
	fmt.Println(style.Info.Render("📸 Starting image fetch for all cameras..."))

	// Start timing for metrics
	timer := metrics.ImageFetchDuration
	startTime := time.Now()

	var wg sync.WaitGroup
	var (
		changedCount   int32 = 0
		errorCount     int32 = 0
		unchangedCount int32 = 0
	)

	for i := range s.entries {
		entry := s.entries[i]

		if entry.Camera.Kind == "iframe" {
			continue
		}
		wg.Add(1)

		go func(entry *Entry, client *http.Client) {
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
				fmt.Println(style.Error.Render(fmt.Sprintf("❌ Error creating HEAD request for %s: %v",
					style.URL.Render(src), err)))
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
				fmt.Println(style.Error.Render(fmt.Sprintf("❌ Error making HEAD request for %s (camera: %s, origin: %s): %v",
					style.URL.Render(src), cameraName, origin, err)))
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "connection").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}

			headResp.Body.Close()

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
				fmt.Println(style.Error.Render(fmt.Sprintf("❌ Error creating GET request for %s: %v",
					style.URL.Render(src), err)))
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
				fmt.Println(style.Error.Render(fmt.Sprintf("❌ Error fetching image %s (camera: %s, origin: %s): %v",
					style.URL.Render(src), cameraName, origin, err)))
				atomic.AddInt32(&errorCount, 1)
				metrics.CameraFetchTotal.WithLabelValues(cameraName, canyon, "error").Inc()
				metrics.OriginFetchTotal.WithLabelValues(origin, "error").Inc()
				metrics.OriginErrorsByType.WithLabelValues(origin, "connection").Inc()
				metrics.CameraAvailability.WithLabelValues(cameraName, canyon).Set(0)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				// Read response body for diagnostic info (limit to 500 bytes)
				bodySnippet := ""
				if body, err := io.ReadAll(io.LimitReader(resp.Body, 500)); err == nil && len(body) > 0 {
					bodySnippet = fmt.Sprintf(", body: %s", string(body))
				}

				fmt.Println(style.Error.Render(fmt.Sprintf("❌ Bad status code from %s: %d %s (Content-Type: %s, Server: %s%s)",
					style.URL.Render(src),
					resp.StatusCode,
					http.StatusText(resp.StatusCode),
					resp.Header.Get("Content-Type"),
					resp.Header.Get("Server"),
					bodySnippet)))
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
				fmt.Println(style.Error.Render(fmt.Sprintf("❌ Error reading image body from %s (camera: %s, origin: %s, content-length: %d): %v",
					style.URL.Render(src), cameraName, origin, resp.ContentLength, err)))
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
		}(entry, s.client)
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

	summary := fmt.Sprintf("  ✨ Fetch complete in %v\n"+
		"  ✅ Changed: %d\n"+
		"  💤 Unchanged: %d\n"+
		"  ❌ Errors: %d",
		duration.Round(time.Millisecond), changedCount, unchangedCount, errorCount)

	if errorCount > 0 {
		fmt.Println(style.Error.Render(summary))
	} else {
		fmt.Println(style.Success.Render(summary))
	}
}

func (s *Store) Get(cameraID string) (EntrySnapshot, bool) {
	s.imagesReady.Wait()
	entry, exists := s.index[cameraID]

	if exists {
		return entry.ShallowSnapshot(), true
	}
	return EntrySnapshot{}, false
}
