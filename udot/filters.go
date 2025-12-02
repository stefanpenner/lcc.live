package udot

import (
	"strings"

	"github.com/stefanpenner/lcc-live/store"
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

// FilterEventsByCanyon filters events by canyon - SR-210 for LCC, SR-190 for BCC
// Prioritizes RoadwayName field as it's the most authoritative identifier
func FilterEventsByCanyon(events []store.Event) (lccEvents []store.Event, bccEvents []store.Event) {
	for _, event := range events {
		roadwayName := strings.ToLower(strings.TrimSpace(event.RoadwayName))
		location := strings.ToLower(event.Location)
		description := strings.ToLower(event.Description)

		// Helper function to check if RoadwayName matches SR-210 patterns
		isLCCRoadway := func(name string) bool {
			if name == "" {
				return false
			}
			return name == "sr-210" ||
				name == "sr 210" ||
				name == "state route 210" ||
				strings.HasPrefix(name, "sr-210") ||
				strings.HasPrefix(name, "sr 210") ||
				strings.HasPrefix(name, "state route 210") ||
				strings.Contains(name, "sr-210") ||
				strings.Contains(name, "sr 210") ||
				strings.Contains(name, "state route 210") ||
				strings.Contains(name, "little cottonwood")
		}

		// Helper function to check if RoadwayName matches SR-190 patterns
		isBCCRoadway := func(name string) bool {
			if name == "" {
				return false
			}
			return name == "sr-190" ||
				name == "sr 190" ||
				name == "state route 190" ||
				strings.HasPrefix(name, "sr-190") ||
				strings.HasPrefix(name, "sr 190") ||
				strings.HasPrefix(name, "state route 190") ||
				strings.Contains(name, "sr-190") ||
				strings.Contains(name, "sr 190") ||
				strings.Contains(name, "state route 190") ||
				strings.Contains(name, "big cottonwood")
		}

		// Helper function for fallback matching in Location/Description
		isLCCFallback := func(text string) bool {
			return strings.Contains(text, "sr-210") ||
				strings.Contains(text, "sr 210") ||
				strings.Contains(text, "state route 210") ||
				strings.Contains(text, "little cottonwood") ||
				(strings.Contains(text, "210") && (strings.Contains(text, "sr") || strings.Contains(text, "route")))
		}

		isBCCFallback := func(text string) bool {
			return strings.Contains(text, "sr-190") ||
				strings.Contains(text, "sr 190") ||
				strings.Contains(text, "state route 190") ||
				strings.Contains(text, "big cottonwood") ||
				(strings.Contains(text, "190") && (strings.Contains(text, "sr") || strings.Contains(text, "route")))
		}

		// Prioritize RoadwayName - it's the most authoritative field
		isLCC := isLCCRoadway(roadwayName)
		if !isLCC {
			isLCC = isLCCFallback(location) || isLCCFallback(description)
		}

		isBCC := isBCCRoadway(roadwayName)
		if !isBCC {
			isBCC = isBCCFallback(location) || isBCCFallback(description)
		}

		if isLCC {
			lccEvents = append(lccEvents, event)
		}
		if isBCC {
			bccEvents = append(bccEvents, event)
		}
	}
	return lccEvents, bccEvents
}
