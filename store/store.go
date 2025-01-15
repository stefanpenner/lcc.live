package store

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Store struct {
	client  *http.Client
	path    string
	index   map[string]*Entry
	entries []*Entry
	mu      sync.RWMutex
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
	cameras, err := CamerasFromFile(path)
	if err != nil {
		return nil, err
	}

	return NewStore(cameras...), err
}

func NewStore(cameras ...Camera) *Store {
	// doesn't need to be threadsafe, as we don't add/remove from the store once
	// it's initialized, only it's entries must be
	index := make(map[string]*Entry)
	entries := []*Entry{}

	// build an index from ID to Camera
	for i := range cameras {
		camera := &cameras[i] // Get pointer to camera
		id := url.QueryEscape(camera.Src)
		camera.ID = id
		entry := &Entry{
			mu:    sync.RWMutex{},
			entry: camera,
		}

		index[id] = entry

		entries = append(entries, entry)
	}
	return &Store{
		entries: entries,
		index:   index,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type Canyon struct {
	Cameras []Camera
	Status  Camera
}

func (s *Store) Canyon(canyon string) Canyon {
	cameras := []Camera{}
	var status Camera

	for _, entry := range s.entries {
		fmt.Printf("canyon: %s\n", canyon)
		fmt.Printf("entry: %s\n", entry.entry.ID)
		if entry.entry.Canyon == canyon {
			if entry.entry.Kind == "status" {
				status = *entry.entry
			} else {
				cameras = append(cameras, *entry.entry)
			}
		}
	}

	return Canyon{
		Status:  status,
		Cameras: cameras,
	}
}

func (s *Store) FetchImages(ctx context.Context) {
	var wg sync.WaitGroup

	for i := range s.entries {
		entry := s.entries[i]

		if entry.entry.Kind == "iframe" {
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

			entry.Read(func(camera *Camera) {
				src = entry.entry.Src
				headers = entry.entry.HTTPHeaders
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

			entry.Write(func(camera *Camera) {
				camera.HTTPHeaders = HTTPHeaders{
					Status:        http.StatusOK,
					ContentType:   contentType,
					ContentLength: contentLength,
					ETag:          newETag,
				}
				camera.Image.Bytes = imageBytes
			})
		}(entry, s.client)
	}
	wg.Wait()
	log.Printf("done")
}

func (s *Store) GetCamera(cameraID string) (Camera, bool) {
	entry, exists := s.index[cameraID]
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return *entry.entry, exists
}
