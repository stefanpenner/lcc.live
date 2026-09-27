package udot

import (
	"strings"

	"github.com/stefanpenner/lcc-live/web/store"
)

// RoadsByCanyon groups road conditions by canyon id.
func RoadsByCanyon(conditions []store.RoadCondition) map[string][]store.RoadCondition {
	out := map[string][]store.RoadCondition{}
	for _, cond := range conditions {
		for _, id := range matchCanyon(cond.RoadwayName, "", "") {
			out[id] = append(out[id], cond)
		}
	}
	return out
}

// EventsByCanyon groups events by canyon id.
// RoadwayName wins. Location and description also match the route number beside "sr" or "route".
func EventsByCanyon(events []store.Event) map[string][]store.Event {
	out := map[string][]store.Event{}
	for _, event := range events {
		seen := map[string]bool{}
		for _, id := range matchCanyon(event.RoadwayName, event.Location, event.Description) {
			if seen[id] {
				continue
			}
			seen[id] = true
			out[id] = append(out[id], event)
		}
	}
	return out
}

func matchCanyon(road, location, description string) []string {
	road = strings.ToLower(strings.TrimSpace(road))
	location = strings.ToLower(location)
	description = strings.ToLower(description)
	var ids []string
	if lccRoad(road) || lccText(location) || lccText(description) {
		ids = append(ids, "LCC")
	}
	if bccRoad(road) || bccText(location) || bccText(description) {
		ids = append(ids, "BCC")
	}
	if provoRoad(road) || provoText(location) || provoText(description) {
		ids = append(ids, "Provo")
	}
	if afcRoad(road) || afcText(location) || afcText(description) {
		ids = append(ids, "AFC")
	}
	if parleysText(road) || parleysText(location) || parleysText(description) {
		ids = append(ids, "Parleys")
	}
	return ids
}

func lccRoad(name string) bool {
	return strings.Contains(name, "sr-210") ||
		strings.Contains(name, "sr 210") ||
		strings.Contains(name, "state route 210") ||
		strings.Contains(name, "little cottonwood")
}

func bccRoad(name string) bool {
	return strings.Contains(name, "sr-190") ||
		strings.Contains(name, "sr 190") ||
		strings.Contains(name, "state route 190") ||
		strings.Contains(name, "big cottonwood")
}

func lccText(text string) bool {
	return lccRoad(text) || routeNumber(text, "210")
}

func bccText(text string) bool {
	return bccRoad(text) || routeNumber(text, "190")
}

func provoRoad(name string) bool {
	return strings.Contains(name, "provo canyon") ||
		strings.Contains(name, "us-189") ||
		strings.Contains(name, "us 189")
}

func provoText(text string) bool {
	return provoRoad(text)
}

func afcRoad(name string) bool {
	return strings.Contains(name, "american fork") ||
		strings.Contains(name, "alpine loop") ||
		strings.Contains(name, "timpanogos cave") ||
		strings.Contains(name, "sr-144") ||
		strings.Contains(name, "sr 144")
}

func afcText(text string) bool {
	return afcRoad(text) || routeNumber(text, "144")
}

func parleysText(text string) bool {
	return strings.Contains(text, "parley")
}

func routeNumber(text, number string) bool {
	return strings.Contains(text, number) && (strings.Contains(text, "sr") || strings.Contains(text, "route"))
}
