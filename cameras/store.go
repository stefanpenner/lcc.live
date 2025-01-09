package cameras

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type CamerasJSON struct {
	Cameras []Camera `json:"cameras"`
}

// TODO: use generics
type Entry struct {
	entry *Camera
	mu    sync.Mutex
}

func (e *Entry) Lock() {
	// fmt.Printf("locking %s\n", e.entry.ID)
	e.mu.Lock()
	// fmt.Printf("locked %s\n", e.entry.ID)
}

func (e *Entry) Unlock() {
	// fmt.Printf("unlocking %s\n", e.entry.ID)
	e.mu.Unlock()
	// fmt.Printf("unlocked %s\n", e.entry.ID)
}

func (e *Entry) Atomic(fn func(*Camera)) {
	e.Lock()
	defer e.Unlock()

	fn(e.entry)
}

type Camera struct {
	// _          sync.Mutex
	ID          string
	Kind        string
	Src         string
	Alt         string
	Canyon      string
	Image       Image
	HTTPHeaders HTTPHeaders
}

type Image struct {
	// _          sync.Mutex
	Src   string
	Bytes []byte
}

type HTTPHeaders struct {
	// _          sync.Mutex
	ContentType   string
	ETag          string
	ContentLength int64
	Status        int
}

type Store struct {
	path    string
	index   map[string]*Entry
	entries []*Entry
}

func FromFile(path string) ([]Camera, error) {
	unparsedJSON, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data CamerasJSON

	err = json.Unmarshal(unparsedJSON, &data)
	if err != nil {
		return nil, err
	}

	return data.Cameras, nil
}

func NewStoreFromFile(path string) (*Store, error) {
	cameras, err := FromFile(path)
	if err != nil {
		return nil, err
	}

	return NewStore(cameras...), err
}

func NewStore(cameras ...Camera) *Store {
	// doesn't need to be threadsafe, as we initialize the camera structure completely prior to any parallelism
	index := make(map[string]*Entry)
	entries := []*Entry{}

	// build an index from ID to Camera
	for i := range cameras {
		camera := &cameras[i] // Get pointer to camera
		id := url.QueryEscape(camera.Src)
		camera.ID = id
		entry := &Entry{
			mu:    sync.Mutex{},
			entry: camera,
		}

		index[id] = entry

		entries = append(entries, entry)
	}

	return &Store{
		entries: entries,
		index:   index,
	}
}

func (s *Store) FetchImages() {
	var wg sync.WaitGroup

	for i := range s.entries {
		entry := s.entries[i]

		if entry.entry.Kind == "iframe" {
			continue
		}
		wg.Add(1)

		go func(entry *Entry) {
			defer wg.Done()

			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			// lock while reading
			// let's simply copy the structs we need for the long-lived function, then unlock immediately after copying
			// when we update, we will relock
			var src string
			var headers HTTPHeaders

			entry.Atomic(func(camera *Camera) {
				src = entry.entry.Src
				headers = entry.entry.HTTPHeaders
			})

			headReq, err := http.NewRequest("HEAD", src, nil)
			if err != nil {
				log.Printf("Error creating HEAD request for %s: %v\n", src, err)
				return
			}

			headResp, err := client.Do(headReq)
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

			// log.Printf("[CHANGED] Image %s (ETag: %s != %s)\n", camera.Src, newETag, camera.HTTPHeaders.ETag)
			resp, err := client.Get(src)
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

			entry.Atomic(func(camera *Camera) {
				camera.HTTPHeaders = HTTPHeaders{
					Status:        http.StatusOK,
					ContentType:   contentType,
					ContentLength: contentLength,
					ETag:          newETag,
				}
				camera.Image.Bytes = imageBytes
			})
		}(entry)
	}
	log.Printf("done")
	wg.Wait()
}

func (s *Store) Get(cameraID string) (Camera, bool) {
	entry, exists := s.index[cameraID]
	entry.Lock()
	defer entry.Unlock()
	return *entry.entry, exists
}
