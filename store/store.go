package store

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
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

func (e *Entry) Snapshot() *EntrySnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// TODO: does this actually copy?
	// Copy fields to a new snapshot
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

func NewStoreFromFile(path string) (*Store, error) {
	canyons := &Canyons{}
	err := canyons.Load(path)
	if err != nil {
		return nil, err
	}

	return NewStore(canyons), err
}

func NewStore(canyons *Canyons) *Store {
	// doesn't need to be threadsafe, as the store is only accessed from a single thread during intializations
	index := make(map[string]*Entry)
	entries := []*Entry{}
	cameras := append(canyons.LCC.Cameras, canyons.BCC.Cameras...)
	cameras = append(cameras, canyons.LCC.Status)
	cameras = append(cameras, canyons.BCC.Status)

	// build an index from ID to Camera
	for i := range cameras {
		camera := &cameras[i] // Get pointer to camera
		id := url.QueryEscape(camera.Src)
		entry := &Entry{
			ID:          id,
			Camera:      camera,
			Image:       &Image{},
			HTTPHeaders: &HTTPHeaders{},
			mu:          sync.RWMutex{},
		}

		index[id] = entry
		entries = append(entries, entry)
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

func (s *Store) Canyon(canyon string) Canyon {
	switch canyon {
	case "LCC":
		return s.canyons.LCC
	case "BCC":
		return s.canyons.BCC
	default:
		panic("invalid canyon: must be either 'LCC' or 'BCC'")
	}
}

func (s *Store) FetchImages(ctx context.Context) {
	var wg sync.WaitGroup

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
				src = entry.Camera.Src
				headers = *entry.HTTPHeaders
			})

			headReq, err := http.NewRequestWithContext(ctx, "HEAD", src, nil)
			if err != nil {
				log.Printf("Error creating HEAD request for %s: %v\n", src, err)
				return
			}

			headResp, err := s.client.Do(headReq)
			if err != nil {
				log.Printf("Error making HEAD request for %s: %v\n", src, err)
				return
			}
			headResp.Body.Close()

			newETag := headResp.Header.Get("ETag")

			if newETag != "" && newETag == headers.ETag {
				// log.Printf("[UNCHANGED]: Image %s (ETag: %s)\n", camera.Src, newETag)
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
				log.Printf("Error fetching image %s: %v\n", src, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("Bad status code from image source %s: %d\n", src, resp.StatusCode)
				return
			}

			contentType := resp.Header.Get("Content-Type")
			contentLength := resp.ContentLength

			imageBytes, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			if err != nil {
				log.Fatalf("Error reading image body: %v\n", err)
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
		}(entry, s.client)
	}
	wg.Wait()
	log.Printf("done")
}

func (s *Store) Get(cameraID string) (*EntrySnapshot, bool) {
	entry, exists := s.index[cameraID]
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.Snapshot(), exists
}
