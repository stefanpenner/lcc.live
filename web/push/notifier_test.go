package push

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stefanpenner/lcc-live/web/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifierSendsOnlyAVBH(t *testing.T) {
	dir := t.TempDir()
	book, err := Open(filepath.Join(dir, "push.json"))
	require.NoError(t, err)
	var got []string
	n := New(book, "pub", "priv", "https://lcc.live", map[string]Canyon{
		"LCC":  {Title: "Little Cottonwood Canyon", Path: "/"},
		"AVBH": {Title: "Apple Valley to Brian Head", Path: "/av-bh"},
	})
	n.send = func(ctx context.Context, sub Subscription, payload []byte) error {
		got = append(got, sub.Endpoint+":"+string(payload))
		return nil
	}
	require.NoError(t, n.Save(Subscription{Canyon: "LCC", Endpoint: "https://push.example/lcc", P256dh: "k", Auth: "a"}))
	require.NoError(t, n.Save(Subscription{Canyon: "AVBH", Endpoint: "https://push.example/avbh", P256dh: "k", Auth: "a"}))

	s := store.NewStore(&store.Canyons{})
	s.UpdateRoadConditions("LCC", []store.RoadCondition{{RoadwayName: "SR-210", Restriction: "Closed"}})
	s.UpdateRoadConditions("AVBH", []store.RoadCondition{{RoadwayName: "SR-143", Restriction: "Closed"}})
	n.Check(context.Background(), s)
	assert.Empty(t, got, "first reading is the baseline")

	n.Check(context.Background(), s)
	assert.Empty(t, got, "still closed")

	s.UpdateRoadConditions("LCC", []store.RoadCondition{{RoadwayName: "SR-210", Restriction: "none", RoadCondition: "Dry"}})
	n.Check(context.Background(), s)
	assert.Empty(t, got, "LCC is not an alert drive")

	s.UpdateRoadConditions("AVBH", []store.RoadCondition{{RoadwayName: "SR-143", Restriction: "none", RoadCondition: "Dry"}})
	n.Check(context.Background(), s)
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "https://push.example/avbh:")
	assert.NotContains(t, got[0], "push.example/lcc")
	assert.Contains(t, got[0], "is open")
	assert.Contains(t, got[0], `"url":"/av-bh"`)

	got = nil
	assert.Equal(t, 0, n.SendTest(context.Background(), "LCC"))
	assert.Equal(t, 1, n.SendTest(context.Background(), "AVBH"))
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "Apple Valley to Brian Head is closed")
	assert.Contains(t, got[0], "The road is closed.")
	assert.Contains(t, got[0], `"tag":"lcc-road-AVBH"`)
	assert.NotContains(t, got[0], "test")
}

func TestSendTestDropsExpiredSubscription(t *testing.T) {
	dir := t.TempDir()
	book, err := Open(filepath.Join(dir, "push.json"))
	require.NoError(t, err)
	n := New(book, "pub", "priv", "https://lcc.live", map[string]Canyon{
		"AVBH": {Title: "Apple Valley to Brian Head", Path: "/av-bh"},
	})
	n.send = func(ctx context.Context, sub Subscription, payload []byte) error {
		return &statusError{code: http.StatusGone}
	}
	require.NoError(t, n.Save(Subscription{
		Canyon: "AVBH", Endpoint: "https://push.example/1", P256dh: "k", Auth: "a",
	}))

	assert.Equal(t, 0, n.SendTest(context.Background(), "AVBH"))
	assert.Empty(t, book.For("AVBH"))
}

func TestPushErrHidesEndpoint(t *testing.T) {
	err := &url.Error{
		Op:  "Post",
		URL: "https://fcm.googleapis.com/fcm/send/secret-token",
		Err: context.DeadlineExceeded,
	}
	got := pushErr(err).Error()
	assert.NotContains(t, got, "secret-token")
	assert.Equal(t, "push endpoint timed out", got)

	rejected := &statusError{code: http.StatusForbidden, body: "BadJwtToken"}
	assert.Equal(t, "push endpoint returned 403: BadJwtToken", pushErr(rejected).Error())
	assert.False(t, gone(rejected))
	assert.True(t, gone(&statusError{code: http.StatusNotFound, body: "no such subscription"}))
}
