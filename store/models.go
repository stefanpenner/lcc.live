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
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Src       string   `json:"src"`
	Alt       string   `json:"alt"`
	Canyon    string   `json:"canyon"`
	SourceId  string   `json:"sourceId,omitempty"`  // Optional: for matching with weather stations
	Latitude  *float64 `json:"latitude,omitempty"`  // Optional: camera latitude for weather station matching
	Longitude *float64 `json:"longitude,omitempty"` // Optional: camera longitude for weather station matching
}

// RoadCondition represents road condition data from UDOT API
type RoadCondition struct {
	Id               int    `json:"Id"`
	SourceId         string `json:"SourceId"`
	RoadCondition    string `json:"RoadCondition"`
	WeatherCondition string `json:"WeatherCondition"`
	Restriction      string `json:"Restriction"`
	RoadwayName      string `json:"RoadwayName"`
	EncodedPolyline  string `json:"EncodedPolyline"`
	LastUpdated      int64  `json:"LastUpdated"`
}

// WeatherStation represents weather station data from UDOT API
type WeatherStation struct {
	Id               int      `json:"Id"`
	Latitude         *float64 `json:"Latitude"`
	Longitude        *float64 `json:"Longitude"`
	StationName      string   `json:"StationName"`
	CameraSource     *string  `json:"CameraSource"`
	CameraSourceId   *string  `json:"CameraSourceId"`
	AirTemperature   *string  `json:"AirTemperature"`
	SurfaceTemp      *string  `json:"SurfaceTemp"`
	SubSurfaceTemp   *string  `json:"SubSurfaceTemp"`
	SurfaceStatus    *string  `json:"SurfaceStatus"`
	RelativeHumidity *string  `json:"RelativeHumidity"`
	DewpointTemp     *string  `json:"DewpointTemp"`
	Precipitation    *string  `json:"Precipitation"`
	WindSpeedAvg     *string  `json:"WindSpeedAvg"`
	WindSpeedGust    *string  `json:"WindSpeedGust"`
	WindDirection    *string  `json:"WindDirection"`
	Source           string   `json:"Source"`
	LastUpdated      int64    `json:"LastUpdated"`
}

// Event represents traffic event data from UDOT API
type Event struct {
	ID                  string   `json:"ID"`
	SourceId            string   `json:"SourceId"`
	Organization        string   `json:"Organization"`
	RoadwayName         string   `json:"RoadwayName"`
	DirectionOfTravel   string   `json:"DirectionOfTravel"`
	Description         string   `json:"Description"`
	Reported            int64    `json:"Reported"`
	LastUpdated         int64    `json:"LastUpdated"`
	StartDate           int64    `json:"StartDate"`
	PlannedEndDate      int64    `json:"PlannedEndDate"`
	LanesAffected       string   `json:"LanesAffected"`
	Latitude            float64  `json:"Latitude"`
	Longitude           float64  `json:"Longitude"`
	LatitudeSecondary   float64  `json:"LatitudeSecondary"`
	LongitudeSecondary  float64  `json:"LongitudeSecondary"`
	EventType           string   `json:"EventType"`
	EventSubType        string   `json:"EventSubType"`
	IsFullClosure       bool     `json:"IsFullClosure"`
	Severity            string   `json:"Severity"`
	Comment             string   `json:"Comment"`
	EncodedPolyline     string   `json:"EncodedPolyline"`
	Restrictions        []string `json:"Restrictions"`
	DetourPolyline      string   `json:"DetourPolyline"`
	DetourInstructions  string   `json:"DetourInstructions"`
	Recurrence          string   `json:"Recurrence"`
	RecurrenceSchedules string   `json:"RecurrenceSchedules"`
	Name                string   `json:"Name"`
	EventCategory       string   `json:"EventCategory"`
	Location            string   `json:"Location"`
	County              string   `json:"County"`
	MPStart             string   `json:"MPStart"`
	MPEnd               string   `json:"MPEnd"`
}

// UnmarshalJSON implements custom JSON unmarshaling for Event to handle ID as either string or number
// and Restrictions as either array of strings or object/null
func (e *Event) UnmarshalJSON(data []byte) error {
	// Define an alias type to avoid infinite recursion
	type Alias Event

	// Create a temporary struct that uses json.RawMessage for ID and Restrictions
	aux := &struct {
		ID           json.RawMessage `json:"ID"`
		Restrictions json.RawMessage `json:"Restrictions"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle ID field - can be string or number
	if len(aux.ID) > 0 {
		// Try to unmarshal as string first
		var idStr string
		if err := json.Unmarshal(aux.ID, &idStr); err == nil {
			e.ID = idStr
		} else {
			// If not a string, try as number
			var idNum float64
			if err := json.Unmarshal(aux.ID, &idNum); err == nil {
				e.ID = strconv.FormatFloat(idNum, 'f', -1, 64)
			} else {
				return fmt.Errorf("cannot unmarshal ID field: %v", err)
			}
		}
	}

	// Handle Restrictions field - can be array of strings, object, null, or missing
	if len(aux.Restrictions) > 0 {
		// Try to unmarshal as array of strings first
		var restrictions []string
		if err := json.Unmarshal(aux.Restrictions, &restrictions); err == nil {
			e.Restrictions = restrictions
		} else {
			// If not an array, check if it's null
			var nullValue *string
			if err := json.Unmarshal(aux.Restrictions, &nullValue); err == nil {
				// It's null, set to empty array
				e.Restrictions = []string{}
			} else {
				// It's an object or other type, set to empty array
				e.Restrictions = []string{}
			}
		}
	} else {
		// Missing field, set to empty array
		e.Restrictions = []string{}
	}

	return nil
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
