package push

import (
	"testing"

	"github.com/stefanpenner/lcc-live/web/store"
	"github.com/stretchr/testify/assert"
)

func TestClosed(t *testing.T) {
	open := []store.RoadCondition{{RoadwayName: "SR-210", RoadCondition: "Dry", Restriction: "none"}}
	shut, label := Closed(open, nil)
	assert.False(t, shut)
	assert.Empty(t, label)

	shut, label = Closed([]store.RoadCondition{{
		RoadwayName: "SR-210 Upper", RoadCondition: "Dry", Restriction: "Closed",
	}}, nil)
	assert.True(t, shut)
	assert.Equal(t, "SR-210 Upper Closed", label)

	shut, label = Closed([]store.RoadCondition{{
		RoadwayName: "SR-190", RoadCondition: "Snow", Restriction: "Traction law",
	}}, nil)
	assert.True(t, shut)
	assert.Contains(t, label, "Traction law")

	shut, label = Closed(open, []store.Event{{IsFullClosure: true, Name: "LCC closed for avalanche"}})
	assert.True(t, shut)
	assert.Equal(t, "LCC closed for avalanche", label)

	shut, _ = Closed(open, []store.Event{{IsFullClosure: false, Description: "flagging Tuesday"}})
	assert.False(t, shut)
}

func TestNotice(t *testing.T) {
	assert.Equal(t, "", Notice(false, false, true))
	assert.Equal(t, "", Notice(true, false, false))
	assert.Equal(t, "", Notice(true, true, true))
	assert.Equal(t, "closed", Notice(true, false, true))
	assert.Equal(t, "open", Notice(true, true, false))
}
