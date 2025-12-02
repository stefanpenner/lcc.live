package server

import (
	"encoding/json"
	"sort"
	"strconv"

	"github.com/cespare/xxhash/v2"
	"github.com/stefanpenner/lcc-live/store"
)

// StableJSONHash generates a stable hash from a JSON-marshalable value.
// It ensures deterministic hashing by sorting slices before marshaling.
func StableJSONHash(v interface{}) (string, error) {
	// Marshal to JSON
	jsonData, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	// Generate hash
	hash := xxhash.Sum64(jsonData)
	return "\"" + strconv.FormatUint(hash, 10) + "\"", nil
}

// SortRoadConditions sorts road conditions by Id for stable ordering
func SortRoadConditions(conditions []store.RoadCondition) []store.RoadCondition {
	sorted := make([]store.RoadCondition, len(conditions))
	copy(sorted, conditions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Id < sorted[j].Id
	})
	return sorted
}

// SortEvents sorts events by ID and their Restrictions slices for stable ordering
func SortEvents(events []store.Event) []store.Event {
	sorted := make([]store.Event, len(events))
	copy(sorted, events)
	// Sort Restrictions slice within each event
	for i := range sorted {
		if len(sorted[i].Restrictions) > 0 {
			sort.Strings(sorted[i].Restrictions)
		}
	}
	// Sort events by ID
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}

// FilterRoadConditions filters out unwanted road conditions
func FilterRoadConditions(conditions []store.RoadCondition) []store.RoadCondition {
	filtered := make([]store.RoadCondition, 0, len(conditions))
	for _, cond := range conditions {
		// Filter out "SR-210 Mouth of Little Cottonwood to SR-190"
		if cond.RoadwayName == "SR-210 Mouth of Little Cottonwood to SR-190" {
			continue
		}
		filtered = append(filtered, cond)
	}
	return filtered
}

