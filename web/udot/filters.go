package udot

import (
	"strings"

	"github.com/stefanpenner/lcc-live/web/store"
)

// FilterRoadConditionsByCanyon filters road conditions by canyon
func FilterRoadConditionsByCanyon(conditions []store.RoadCondition) (lccConditions []store.RoadCondition, bccConditions []store.RoadCondition) {
	for _, cond := range conditions {
		name := strings.ToLower(cond.RoadwayName)

		// LCC: match "Little Cottonwood", "LCC", "SR-210", "210"
		if strings.Contains(name, "little cottonwood") ||
			strings.Contains(name, "lcc") ||
			strings.Contains(name, "sr-210") ||
			strings.Contains(name, " 210") ||
			strings.Contains(name, "-210") {
			lccConditions = append(lccConditions, cond)
		}

		// BCC: match "Big Cottonwood", "BCC", "SR-190", "190"
		if strings.Contains(name, "big cottonwood") ||
			strings.Contains(name, "bcc") ||
			strings.Contains(name, "sr-190") ||
			strings.Contains(name, " 190") ||
			strings.Contains(name, "-190") {
			bccConditions = append(bccConditions, cond)
		}
	}
	return lccConditions, bccConditions
}

// FilterEventsByCanyon keeps SR-210 on LCC and SR-190 on BCC.
// RoadwayName wins. Location and description also match the route number beside "sr" or "route".
func FilterEventsByCanyon(events []store.Event) (lccEvents []store.Event, bccEvents []store.Event) {
	for _, event := range events {
		road := strings.ToLower(strings.TrimSpace(event.RoadwayName))
		location := strings.ToLower(event.Location)
		description := strings.ToLower(event.Description)

		if lccRoad(road) || lccText(location) || lccText(description) {
			lccEvents = append(lccEvents, event)
		}
		if bccRoad(road) || bccText(location) || bccText(description) {
			bccEvents = append(bccEvents, event)
		}
	}
	return lccEvents, bccEvents
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

func routeNumber(text, number string) bool {
	return strings.Contains(text, number) && (strings.Contains(text, "sr") || strings.Contains(text, "route"))
}
