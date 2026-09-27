// Package push notices browsers when a canyon road closes or opens.
package push

import (
	"strings"

	"github.com/stefanpenner/lcc-live/web/store"
)

// Closed reports whether the canyon road is shut or under a traction law.
// The label is the roadway plus the restriction that tripped it.
func Closed(roads []store.RoadCondition, events []store.Event) (bool, string) {
	for _, ev := range events {
		if !ev.IsFullClosure {
			continue
		}
		label := ev.Name
		if label == "" {
			label = ev.Description
		}
		if label == "" {
			label = ev.RoadwayName
		}
		return true, strings.TrimSpace(label)
	}
	for _, rd := range roads {
		text := strings.ToLower(rd.Restriction + " " + rd.RoadCondition)
		if !strings.Contains(text, "closed") && !strings.Contains(text, "traction") {
			continue
		}
		bit := rd.Restriction
		if bit == "" || strings.EqualFold(bit, "none") {
			bit = rd.RoadCondition
		}
		return true, strings.TrimSpace(rd.RoadwayName + " " + bit)
	}
	return false, ""
}

// Notice is the edge between two closure readings.
// The first reading is a baseline and sends nothing.
func Notice(seen, wasClosed, closed bool) string {
	if !seen {
		return ""
	}
	if !wasClosed && closed {
		return "closed"
	}
	if wasClosed && !closed {
		return "open"
	}
	return ""
}
