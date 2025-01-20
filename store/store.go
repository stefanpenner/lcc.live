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

// Let's talk about the concurrency model:
//   - The Store is immutable post initialization, except for it's entries
//     values.
//   - Locking occurs at the entry level, through RWMutex
//
// To allow concurrent access to Entry Structs we abide by the following pattern:
//  1. the entry struct is mutable, and has a RWMutex, but is kept internal ot
//     the store
//  2. the entry struct points to only immutable values, when these values
//     change the old is left unchanged, a new one is created, and then assigned
//     to the stable entry struct
//  4. external access to entries are provided via snapshots of the entry object
//  5. a handshake agreement exists, where consumers of EntrySnapshot treat it
//     as "deep frozen"
//
// TODO: EntrySnapshot (and it's descendent structs) should consider having
// private members, and public getters.
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
	// doesn't need to be threadsafe, as the store is only accessed from a single thread during intializations
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

// TODO: this should return a summary of what changed, so that we can:
// 1. provide a /status that is updated via SSE
// 2. provide image updates via SSE
func (s *Store) FetchImages(ctx context.Context) {
	fmt.Println(infoStyle.Render("📸 Starting image fetch for all cameras..."))
	var wg sync.WaitGroup
	startTime := time.Now()
	successCount := 0
	errorCount := 0
	unchangedCount := 0
	var mu sync.Mutex // for thread-safe counter updates

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
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}

			headResp, err := s.client.Do(headReq)
			if err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Error making HEAD request for %s: %v",
					urlStyle.Render(src), err)))
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}
			headResp.Body.Close()

			newETag := headResp.Header.Get("ETag")

			if newETag != "" && newETag == headers.ETag {
				mu.Lock()
				unchangedCount++
				mu.Unlock()
				return
			}

			getReq, err := http.NewRequestWithContext(ctx, "GET", src, nil)
			if err != nil {
				log.Printf("Error creating GET request for %s: %v\n", src, err)
				return
			}

			// log.Printf("[CHANGED] Image %s (ETag: %s != %s)\n", camera.Src, newETag, camera.HTTPHeaders.ETag)
			resp, err := s.client.Do(getReq)
			if err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Error fetching image %s: %v",
					urlStyle.Render(src), err)))
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Bad status code from %s: %d",
					urlStyle.Render(src), resp.StatusCode)))
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}

			contentType := resp.Header.Get("Content-Type")
			contentLength := resp.ContentLength

			imageBytes, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			if err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Error reading image body from %s: %v",
					urlStyle.Render(src), err)))
				mu.Lock()
				errorCount++
				mu.Unlock()
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
			mu.Lock()
			successCount++
			mu.Unlock()
		}(entry, s.client)
	}
	wg.Wait()
	duration := time.Since(startTime).Round(time.Millisecond)

	summary := fmt.Sprintf("  ✨ Fetch complete in %v\n"+
		"  ✅ Success: %d\n"+
		"  💤 Unchanged: %d\n"+
		"  ❌ Errors: %d",
		duration, successCount, unchangedCount, errorCount)

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
