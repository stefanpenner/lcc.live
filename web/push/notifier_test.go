package push

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stefanpenner/lcc-live/web/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifierSendsOnClose(t *testing.T) {
	dir := t.TempDir()
	book, err := Open(filepath.Join(dir, "push.json"))
	require.NoError(t, err)
	var got []string
	n := New(book, "pub", "priv", "https://lcc.live", map[string]Canyon{
		"LCC": {Title: "Little Cottonwood Canyon", Path: "/"},
	})
	n.send = func(ctx context.Context, sub Subscription, payload []byte) error {
		got = append(got, sub.Endpoint+":"+string(payload))
		return nil
	}
	require.NoError(t, n.Save(Subscription{Canyon: "LCC", Endpoint: "https://push.example/1", P256dh: "k", Auth: "a"}))

	s := store.NewStore(&store.Canyons{})
	s.UpdateRoadConditions("LCC", []store.RoadCondition{{RoadwayName: "SR-210", Restriction: "Closed"}})
	n.Check(context.Background(), s)
	assert.Empty(t, got, "first reading is the baseline")

	n.Check(context.Background(), s)
	assert.Empty(t, got, "still closed")

	s.UpdateRoadConditions("LCC", []store.RoadCondition{{RoadwayName: "SR-210", Restriction: "none", RoadCondition: "Dry"}})
	n.Check(context.Background(), s)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "is open")
	assert.Contains(t, got[0], `"url":"/"`)

	got = nil
	assert.Equal(t, 1, n.SendTest(context.Background(), "LCC"))
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "is closed")
	assert.Contains(t, got[0], "The road is closed.")
	assert.Equal(t, 0, n.SendTest(context.Background(), "BCC"))
}
