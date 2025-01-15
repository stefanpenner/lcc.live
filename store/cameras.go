package store

import (
	"encoding/json"
	"os"
)

type CamerasJSON struct {
	Cameras []Camera `json:"cameras"`
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
	Index       int
	Reload      bool
	IsIframe    bool
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

func CamerasFromFile(path string) ([]Camera, error) {
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
