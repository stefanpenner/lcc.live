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

type Camera struct {
	ID     string
	Kind   string
	Src    string
	Alt    string
	Canyon string
	Image  Image
}

type Image struct {
	Status        int
	Src           string
	ContentType   string
	Bytes         []byte
	ContentLength int64
	ETag          string
}

type Store struct {
	path       string
	IDToCamera map[string]*Camera
	Cameras    []Camera
}

func NewStoreFromFile(path string) (*Store, error) {
	cameras, err := FromFile(path)
	if err != nil {
		return nil, err
	}

	return NewStore(cameras...), err
}

func NewStore(cameras ...Camera) *Store {
	IDToCamera := make(map[string]*Camera)

	// build an index from ID to Camera
	for i := range cameras {
		camera := &cameras[i] // Get pointer to camera
		id := url.QueryEscape(camera.Src)
		camera.ID = id
		IDToCamera[id] = camera
	}

	return &Store{
		Cameras:    cameras,
		IDToCamera: IDToCamera,
	}
}

func (s *Store) FetchImages() {
	cameras := s.Cameras
	// TODO: lock cameras
	var wg sync.WaitGroup
	wg.Add(len(cameras))

	for i := range cameras {
		// i := i
		go func(i int) {
			camera := s.Cameras[i]
			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			log.Printf("HEAD")
			// First make a HEAD request to check headers
			headReq, err := http.NewRequest("HEAD", camera.Src, nil)
			if err != nil {
				log.Printf("Error creating HEAD request for %s: %v\n", camera.Src, err)
				return
			}

			headResp, err := client.Do(headReq)
			if err != nil {
				log.Printf("Error making HEAD request for %s: %v\n", camera.Src, err)
				return
			}
			headResp.Body.Close()

			newETag := headResp.Header.Get("ETag")

			// If content hasn't changed (same ETag) and we have data, skip download
			if newETag != "" newETag == camera.Image.ETag {
				log.Printf("Image %s unchanged (ETag: %s)\n", camera.Src, newETag)
				return
			}

			log.Printf("Image %s changed (ETag: %s != %s)\n", camera.Src, newETag, camera.Image.ETag)
			// Fetch the actual image
			resp, err := client.Get(camera.Src)
			if err != nil {
				log.Printf("Error fetching image %s: %v\n", camera.Src, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("Bad status code from image source %s: %d\n", camera.Src, resp.StatusCode)
				return
			}

			contentType := resp.Header.Get("Content-Type")
			contentLength := resp.ContentLength

			// Read the full response body into memory
			imageBytes, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			if err != nil {
				log.Fatalf("Error reading image body: %v\n", err)
			}

			camera.Image = Image{
				Status:        http.StatusOK,
				Bytes:         imageBytes,
				ContentType:   contentType,
				ContentLength: contentLength,
				ETag:          newETag,
			}
		}(i)
	}
	wg.Wait()

	// TODO: unlock cameras
}

func (s *Store) Get(cameraID string) (*Camera, bool) {
	camera, exists := s.IDToCamera[cameraID]
	return camera, exists
}
