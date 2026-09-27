package udot

import (
	"strconv"
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
	if avbh(road, location, description) {
		ids = append(ids, "AVBH")
	}
	return ids
}

// avbh is the Apple Valley to Brian Head drive.
// The drive is SR-59, I-15 from Anderson Junction through Parowan, then SR-143.
func avbh(road, location, description string) bool {
	text := road + " " + location + " " + description
	if hasRoute(text, "143") || hasRoute(text, "59") ||
		strings.Contains(text, "brian head") ||
		strings.Contains(text, "parowan canyon") ||
		strings.Contains(text, "apple valley") {
		return true
	}
	return i15OnAVBH(text)
}

func hasRoute(text, num string) bool {
	for _, prefix := range []string{"sr-" + num, "sr " + num, "state route " + num} {
		from := 0
		for {
			i := strings.Index(text[from:], prefix)
			if i < 0 {
				break
			}
			i += from
			end := i + len(prefix)
			if end >= len(text) || text[end] < '0' || text[end] > '9' {
				return true
			}
			from = end
		}
	}
	return false
}

func i15OnAVBH(text string) bool {
	if !hasI15(text) {
		return false
	}
	if strings.Contains(text, "st george") || strings.Contains(text, "st. george") {
		return false
	}
	for _, place := range []string{
		"cedar city",
		"parowan",
		"iron/washington",
		"black ridge",
		"new harmony",
		"ash creek",
		"hamilton",
		"kanarraville",
		"anderson",
	} {
		if strings.Contains(text, place) {
			return true
		}
	}
	return i15MileOnDrive(text)
}

func hasI15(text string) bool {
	for _, prefix := range []string{"i-15", "i 15"} {
		from := 0
		for {
			i := strings.Index(text[from:], prefix)
			if i < 0 {
				break
			}
			i += from
			end := i + len(prefix)
			if end >= len(text) || text[end] < '0' || text[end] > '9' {
				return true
			}
			from = end
		}
	}
	return false
}

func i15MileOnDrive(text string) bool {
	for _, key := range []string{"milepost:", "milepost ", "mp:", "mp "} {
		rest := text
		for {
			i := strings.Index(rest, key)
			if i < 0 {
				break
			}
			n, ok := leadingNumber(rest[i+len(key):])
			if ok && n >= 15 && n <= 78 {
				return true
			}
			rest = rest[i+len(key):]
		}
	}
	return false
}

func leadingNumber(s string) (float64, bool) {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	start := i
	dot := false
	for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || (s[i] == '.' && !dot)) {
		if s[i] == '.' {
			dot = true
		}
		i++
	}
	if i == start || (dot && i == start+1) {
		return 0, false
	}
	n, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil {
		return 0, false
	}
	return n, true
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
