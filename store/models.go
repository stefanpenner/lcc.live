package store

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"
)

type Image struct {
	Src   string
	Bytes []byte
	_     sync.Mutex
}

type HTTPHeaders struct {
	ContentType   string
	ETag          string
	ContentLength int64
	Status        int
	_             sync.Mutex
}

type Camera struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Src    string `json:"src"`
	Alt    string `json:"alt"`
	Canyon string `json:"canyon"`
	_      sync.Mutex
}

type Canyon struct {
	Name    string   `json:"name"`
	Status  Camera   `json:"status"`
	Cameras []Camera `json:"cameras"`
	_       sync.Mutex
}

type Canyons struct {
	LCC Canyon `json:"lcc"`
	BCC Canyon `json:"bcc"`
	_   sync.Mutex
}

func (c *Canyons) Load(f fs.FS, filepath string) error {
	data, err := f.(fs.ReadFileFS).ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filepath, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("file %s is empty", filepath)
	}

	// Try to validate JSON structurgge before unmarshaling
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON in file %s", filepath)
	}

	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %w", filepath, err)
	}

	// Validate required data was loaded
	if c.LCC.Status.Src == "" && c.BCC.Status.Src == "" {
		return fmt.Errorf("JSON from %s did not contain expected canyon data", filepath)
	}

	return nil
}

func (c *Canyons) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
