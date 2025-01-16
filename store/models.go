package store

import (
	"encoding/json"
	"fmt"
	"os"
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

func (c *Canyons) Load(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("file %s is empty", filename)
	}

	// Try to validate JSON structure before unmarshaling
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON in file %s", filename)
	}

	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %w", filename, err)
	}

	// Validate required data was loaded
	if c.LCC.Status.Src == "" && c.BCC.Status.Src == "" {
		return fmt.Errorf("JSON from %s did not contain expected canyon data", filename)
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
