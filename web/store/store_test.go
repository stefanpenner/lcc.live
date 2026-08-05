package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_Canyon(t *testing.T) {
	canyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Src: "http://cam1", Canyon: "LCC"},
			},
		},
		BCC: Canyon{
			Name: "BCC",
			Cameras: []Camera{
				{Src: "http://cam2", Canyon: "BCC"},
			},
		},
	}

	store := NewStore(canyons)

	lcc := store.Canyon("LCC")
	assert.Equal(t, "LCC", lcc.Name)
	assert.Len(t, lcc.Cameras, 1)
	assert.NotEmpty(t, lcc.Cameras[0].ID)

	bcc := store.Canyon("BCC")
	assert.Equal(t, "BCC", bcc.Name)
	assert.Len(t, bcc.Cameras, 1)
	assert.NotEmpty(t, bcc.Cameras[0].ID)
}

func TestStore_Fetch_and_Get_Images(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		w.Header().Set("Content-Length", "15")

		if r.Method == "GET" {
			w.Write([]byte("mock image data"))
		}
	}))
	defer server.Close()

	canyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:   "webcam",
					Src:    server.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(canyons)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store.FetchImages(ctx)

	id := store.entries[0].Camera.ID
	entry, exists := store.Get(id)

	require.True(t, exists, "Camera entry should exist")
	assert.NotNil(t, entry.Image)
	assert.Equal(t, "mock image data", string(entry.Image.Bytes))
	assert.Equal(t, "image/jpeg", entry.HTTPHeaders.ContentType)
	assert.NotEmpty(t, entry.HTTPHeaders.ETag)

	entry, exists = store.Get("unknown")
	assert.False(t, exists)
}

func TestStore_ConcurrentReads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("mock image data"))
		}
	}))
	defer server.Close()

	canyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:   "webcam",
					Src:    server.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(canyons)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store.FetchImages(ctx)

	// Perform concurrent reads
	const numReaders = 100
	var wg sync.WaitGroup
	cameraID := store.entries[0].Camera.ID

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry, exists := store.Get(cameraID)
			assert.True(t, exists)
			assert.NotNil(t, entry.Image)
			assert.NotEmpty(t, entry.Image.Bytes)
		}()
	}

	wg.Wait()
}

func TestStore_ConcurrentFetchAndRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("mock image data"))
		}
	}))
	defer server.Close()

	canyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:   "webcam",
					Src:    server.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(canyons)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initial fetch
	store.FetchImages(ctx)

	cameraID := store.entries[0].Camera.ID
	var wg sync.WaitGroup

	// Start multiple readers
	const numReaders = 50
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				entry, exists := store.Get(cameraID)
				assert.True(t, exists)
				assert.NotNil(t, entry.Image)
			}
		}()
	}

	// Start concurrent fetchers
	const numFetchers = 5
	for i := 0; i < numFetchers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.FetchImages(ctx)
		}()
	}

	wg.Wait()
}

func TestStore_FetchImages_ETagCaching(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("ETag", "\"stable-etag\"")
			return
		}
		if r.Method == "GET" {
			requestCount++
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("ETag", "\"stable-etag\"")
			w.Write([]byte("mock image data"))
		}
	}))
	defer server.Close()

	canyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:   "webcam",
					Src:    server.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(canyons)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First fetch should download the image
	store.FetchImages(ctx)
	assert.Equal(t, 1, requestCount)

	// Second fetch should skip download due to matching ETag
	store.FetchImages(ctx)
	assert.Equal(t, 1, requestCount, "Second fetch should not download due to ETag match")
}

func TestStore_FetchImages_ErrorHandling(t *testing.T) {
	// Server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	canyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:   "webcam",
					Src:    server.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(canyons)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should not panic on errors
	store.FetchImages(ctx)

	// Get should still work, just with empty image
	cameraID := store.entries[0].Camera.ID
	entry, exists := store.Get(cameraID)
	require.True(t, exists)
	// Image should be empty or default
	assert.NotNil(t, entry.Image)
}

func TestStore_FetchImages_SkipsIframes(t *testing.T) {
	canyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:   "iframe",
					Src:    "http://example.com/iframe.html",
					Alt:    "Iframe Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(canyons)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should not try to fetch iframe sources
	store.FetchImages(ctx)

	cameraID := store.entries[0].Camera.ID
	entry, exists := store.Get(cameraID)
	require.True(t, exists)
	// Image should be empty since we skip iframes
	assert.Empty(t, entry.Image.Bytes)
}

// Overlapping FetchImages before first ready must not panic (WaitGroup double-Done).
func TestStore_FetchImages_ConcurrentReadyNoPanic(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"e\"")
		if r.Method == "GET" {
			_, _ = w.Write([]byte("img"))
		}
	}))
	defer server.Close()

	store := NewStore(&Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Kind: "webcam", Src: server.URL + "/c.jpg", Alt: "Cam", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	})

	assert.False(t, store.IsReady())
	assert.False(t, store.HasAnyLiveImage())

	ctx := context.Background()
	var wg sync.WaitGroup
	const n = 16
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			store.FetchImages(ctx)
		}()
	}

	// Let goroutines contend on ready gate / single-flight
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	assert.True(t, store.IsReady())
	assert.True(t, store.HasAnyLiveImage())
	assert.True(t, store.IsReady(), "IsReady sticky")
}

// While a fetch is in flight, additional FetchImages calls skip (single-flight).
func TestStore_FetchImages_SingleFlightSkips(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var getCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			atomic.AddInt32(&getCount, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("img"))
			return
		}
		w.Header().Set("ETag", "\"x\"")
	}))
	defer server.Close()

	store := NewStore(&Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Kind: "webcam", Src: server.URL + "/c.jpg", Alt: "Cam", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	})

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		store.FetchImages(ctx)
		close(done)
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first fetch did not reach GET")
	}

	// Contending calls while first is blocked must not start more GETs
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.FetchImages(ctx)
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(0), atomic.LoadInt32(&getCount), "skippers must not complete GET while first holds flight")

	close(release)
	<-done
	assert.Equal(t, int32(1), atomic.LoadInt32(&getCount))
}

func TestStore_ContentLengthMatchesBytes(t *testing.T) {
	body := []byte("mock image data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"e\"")
		if r.Method == "GET" {
			// No Content-Length header → client sees ContentLength -1; store must use len(bytes)
			_, _ = w.Write(body)
		}
	}))
	defer server.Close()

	store := NewStore(&Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Kind: "webcam", Src: server.URL + "/c.jpg", Alt: "Cam", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	})
	store.FetchImages(context.Background())

	entry, ok := store.Get(store.entries[0].Camera.ID)
	require.True(t, ok)
	require.NotNil(t, entry.Image)
	assert.Equal(t, body, entry.Image.Bytes)
	assert.Equal(t, int64(len(body)), entry.HTTPHeaders.ContentLength,
		"ContentLength must be len(bytes), not upstream Content-Length (-1 when omitted)")
}

func TestStore_WeatherStationReturnsCopy(t *testing.T) {
	store := NewStore(&Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:             "webcam",
					Src:              "http://example.com/c.jpg",
					Alt:              "Cam",
					Canyon:           "LCC",
					WeatherStationId: intPtr(42),
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	})
	// Unblock GetWeatherStation (waits on first image ready)
	store.FetchImages(context.Background())

	name := "Station A"
	store.StoreWeatherStationsById([]WeatherStation{
		{Id: 42, StationName: name},
	})

	got := store.GetWeatherStation(store.entries[0].Camera.ID)
	require.NotNil(t, got)
	assert.Equal(t, "Station A", got.StationName)
	got.StationName = "MUTATED"

	got2 := store.GetWeatherStation(store.entries[0].Camera.ID)
	require.NotNil(t, got2)
	assert.Equal(t, "Station A", got2.StationName, "store must not expose mutable alias")

	byCanyon := store.GetWeatherStationsForCanyon(store.Canyon("LCC"))
	require.Len(t, byCanyon, 1)
	for _, st := range byCanyon {
		st.StationName = "MUTATED2"
	}
	got3 := store.GetWeatherStation(store.entries[0].Camera.ID)
	require.NotNil(t, got3)
	assert.Equal(t, "Station A", got3.StationName)
}

func intPtr(i int) *int { return &i }

func strPtr(s string) *string { return &s }

func TestStore_WeatherPrefersSynopticStid(t *testing.T) {
	store := NewStore(&Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{
					Kind:             "webcam",
					Src:              "http://example.com/c.jpg",
					Alt:              "Peak",
					Canyon:           "LCC",
					WeatherStationId: intPtr(42),
					SynopticStid:     strPtr("CLN"),
				},
				{
					Kind:             "webcam",
					Src:              "http://example.com/r.jpg",
					Alt:              "Road",
					Canyon:           "LCC",
					WeatherStationId: intPtr(42),
				},
			},
		},
		BCC: Canyon{Name: "BCC"},
	})
	store.FetchImages(context.Background())

	store.StoreWeatherStationsById([]WeatherStation{
		{Id: 42, StationName: "UDOT Road", AirTemperature: strPtr("70")},
	})
	store.StoreWeatherStationsByStid([]WeatherStation{
		{StationName: "ALTA - COLLINS", CameraSourceId: strPtr("CLN"), AirTemperature: strPtr("48"), Source: "nws"},
	})

	// Peak cam has both → mountain stid wins
	peak := store.GetWeatherStation(store.entries[0].Camera.ID)
	require.NotNil(t, peak)
	assert.Equal(t, "ALTA - COLLINS", peak.StationName)

	// Road-only → UDOT
	road := store.GetWeatherStation(store.entries[1].Camera.ID)
	require.NotNil(t, road)
	assert.Equal(t, "UDOT Road", road.StationName)

	stids := store.SynopticStids()
	assert.Equal(t, []string{"CLN"}, stids)
}
