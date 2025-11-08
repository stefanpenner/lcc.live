package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/mitchellh/hashstructure"
)

type Image struct {
	Src   string
	ETag  string
	Bytes []byte
}

type HTTPHeaders struct {
	ContentType   string
	ETag          string
	ContentLength int64
	Status        int
}

type Camera struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Src      string `json:"src"`
	Alt      string `json:"alt"`
	Canyon   string `json:"canyon"`
	Position int    `json:"position,omitempty"`
}

type Canyon struct {
	Name    string   `json:"name"`
	ETag    string   `json:"etag"`
	Status  Camera   `json:"status"`
	Cameras []Camera `json:"cameras"`
}

type Canyons struct {
	LCC Canyon `json:"lcc"`
	BCC Canyon `json:"bcc"`
}

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

// NeonRepository defines the minimal interface needed from neon.Repository.
// This allows for easier testing with mocks.
type NeonRepository interface {
	ListCanyons(ctx context.Context) ([]NeonCanyon, error)
}

// NeonCanyon represents canyon data from the Neon database.
// These types mirror neon.Canyon but are defined here to avoid import cycles.
type NeonCanyon struct {
	ID      string
	Name    string
	Status  *NeonCanyonStatus
	Cameras []NeonCamera
}

// NeonCanyonStatus represents status camera data from Neon.
type NeonCanyonStatus struct {
	Src  string
	Alt  string
	Kind string
}

// NeonCamera represents camera data from Neon.
type NeonCamera struct {
	ID       string
	CanyonID string
	Src      string
	Alt      string
	Kind     string
	Position int
}

// NewStoreFromNeon creates a new Store by loading data from a Neon repository.
func NewStoreFromNeon(ctx context.Context, repo NeonRepository) (*Canyons, error) {
	neonCanyons, err := repo.ListCanyons(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list canyons from Neon: %w", err)
	}

	canyons := &Canyons{}

	// Convert Neon data to store format
	for _, neonCanyon := range neonCanyons {
		var canyon Canyon
		canyon.Name = neonCanyon.Name

		// Convert status camera if present
		if neonCanyon.Status != nil {
			canyon.Status = Camera{
				ID:   "", // Will be set by Store.createEntry
				Kind: neonCanyon.Status.Kind,
				Src:  neonCanyon.Status.Src,
				Alt:  neonCanyon.Status.Alt,
			}
		}

		// Convert regular cameras
		canyon.Cameras = make([]Camera, len(neonCanyon.Cameras))
		for i, neonCamera := range neonCanyon.Cameras {
			canyon.Cameras[i] = Camera{
				ID:       neonCamera.ID,
				Kind:     neonCamera.Kind,
				Src:      neonCamera.Src,
				Alt:      neonCamera.Alt,
				Position: neonCamera.Position,
			}
		}

		// Assign to appropriate canyon
		switch neonCanyon.ID {
		case "LCC":
			canyons.LCC = canyon
		case "BCC":
			canyons.BCC = canyon
		default:
			return nil, fmt.Errorf("unknown canyon ID: %s", neonCanyon.ID)
		}
	}

	// Compute ETags for both canyons
	if err := canyons.setETag(&canyons.LCC); err != nil {
		return nil, fmt.Errorf("failed to compute LCC ETag: %w", err)
	}
	if err := canyons.setETag(&canyons.BCC); err != nil {
		return nil, fmt.Errorf("failed to compute BCC ETag: %w", err)
	}

	return canyons, nil
}
