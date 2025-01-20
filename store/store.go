package store

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF0000"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ADD8"))

	urlStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#FFA500"))
)

type Store struct {
	client  *http.Client
	canyons *Canyons
	path    string
	index   map[string]*Entry
	entries []*Entry
	mu      sync.RWMutex
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
func (e *Entry) ShallowSnapshot() *EntrySnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &EntrySnapshot{
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
	e.mu.RLock()
	defer e.mu.RUnlock()

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

	// Process status cameras
	createEntry(&canyons.LCC.Status)
	createEntry(&canyons.BCC.Status)

	// Process regular cameras
	for i := range canyons.LCC.Cameras {
		createEntry(&canyons.LCC.Cameras[i])
	}
	for i := range canyons.BCC.Cameras {
		createEntry(&canyons.BCC.Cameras[i])
	}

	return &Store{
		entries: entries,
		index:   index,
		canyons: canyons,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
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
	fmt.Println(infoStyle.Render("📸 Starting image fetch for all cameras..."))
	var wg sync.WaitGroup
	startTime := time.Now()
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

			// lock while reading
			// let's simply copy the structs we need for the long-lived function,
			// then unlock immediately after copying when we update, we will relock
			var src string
			var headers HTTPHeaders

			entry.Read(func(entry *Entry) {
				src = entry.Camera.Src // Copy
				// TODO: explore option of an explicit copy via Copy() or Snapshot(), vs the current implicit approach
				headers = *entry.HTTPHeaders // Copy
			})

			headReq, err := http.NewRequestWithContext(ctx, "HEAD", src, nil)
			if err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Error creating HEAD request for %s: %v",
					urlStyle.Render(src), err)))
				atomic.AddInt32(&errorCount, 1)
				return
			}

			headResp, err := s.client.Do(headReq)
			if err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Error making HEAD request for %s: %v",
					urlStyle.Render(src), err)))
				atomic.AddInt32(&errorCount, 1)
				return
			}
			headResp.Body.Close()

			newETag := headResp.Header.Get("ETag")

			if newETag != "" && newETag == headers.ETag {
				atomic.AddInt32(&unchangedCount, 1)
				return
			}

			getReq, err := http.NewRequestWithContext(ctx, "GET", src, nil)
			if err != nil {
				log.Printf("Error creating GET request for %s: %v\n", src, err)
				return
			}

			resp, err := s.client.Do(getReq)
			if err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Error fetching image %s: %v",
					urlStyle.Render(src), err)))
				atomic.AddInt32(&errorCount, 1)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Bad status code from %s: %d",
					urlStyle.Render(src), resp.StatusCode)))
				atomic.AddInt32(&errorCount, 1)
				return
			}

			contentType := resp.Header.Get("Content-Type")
			contentLength := resp.ContentLength

			imageBytes, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			if err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Error reading image body from %s: %v",
					urlStyle.Render(src), err)))
				atomic.AddInt32(&errorCount, 1)
				return
			}

			entry.Write(func(entry *Entry) {
				entry.HTTPHeaders = &HTTPHeaders{
					Status:        http.StatusOK,
					ContentType:   contentType,
					ContentLength: contentLength,
					ETag:          newETag,
				}
				entry.Image.Bytes = imageBytes
			})
			atomic.AddInt32(&changedCount, 1)
		}(entry, s.client)
	}
	wg.Wait()
	duration := time.Since(startTime).Round(time.Millisecond)

	summary := fmt.Sprintf("  ✨ Fetch complete in %v\n"+
		"  ✅ Changed: %d\n"+
		"  💤 Unchanged: %d\n"+
		"  ❌ Errors: %d",
		duration, changedCount, unchangedCount, errorCount)

	if errorCount > 0 {
		fmt.Println(errorStyle.Render(summary))
	} else {
		fmt.Println(successStyle.Render(summary))
	}
}

func (s *Store) Get(cameraID string) (*EntrySnapshot, bool) {
	entry, exists := s.index[cameraID]
	return entry.ShallowSnapshot(), exists
}
