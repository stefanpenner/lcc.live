// Package store provides data storage and camera management for canyon webcams
package store

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/mitchellh/hashstructure"
)

// Image represents a cached camera image with metadata
type Image struct {
	Src   string
	ETag  string
	Bytes []byte
}

// HTTPHeaders contains HTTP response metadata for cached images
type HTTPHeaders struct {
	ContentType   string
	ETag          string
	ContentLength int64
	Status        int
}

// Camera represents a webcam with its configuration
type Camera struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Src    string `json:"src"`
	Alt    string `json:"alt"`
	Canyon string `json:"canyon"`
}

// Canyon represents a canyon with its cameras and status
type Canyon struct {
	Name    string   `json:"name"`
	ETag    string   `json:"etag"`
	Status  Camera   `json:"status"`
	Cameras []Camera `json:"cameras"`
}

// Canyons represents the collection of all canyons
type Canyons struct {
	LCC Canyon `json:"lcc"`
	BCC Canyon `json:"bcc"`
}

// Load loads canyon data from a JSON file
func (c *Canyons) Load(f fs.FS, filepath string) error {
	data, err := f.(fs.ReadFileFS).ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filepath, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("file %s is empty", filepath)
	}

	// Try to validate JSON structure before unmarshaling
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON in file %s", filepath)
	}

	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse JSON from %s: %w", filepath, err)
	}

	// precompute etags
	if err := c.setETag(&c.LCC); err != nil {
		return fmt.Errorf("failed to compute LCC ETag: %w", err)
	}
	if err := c.setETag(&c.BCC); err != nil {
		return fmt.Errorf("failed to compute BCC ETag: %w", err)
	}

	return nil
}

func (c *Canyons) setETag(canyon *Canyon) error {
	hash, err := hashstructure.Hash(canyon, nil)
	if err != nil {
		return err
	}
	canyon.ETag = "\"" + strconv.FormatUint(hash, 10) + "\""
	return nil
}

func (c *Canyons) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
