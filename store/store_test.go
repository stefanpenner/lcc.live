package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestStore_Reload_PreservesImageCache(t *testing.T) {
	// Create initial store with a camera
	initialCanyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Src: "http://example.com/cam1.jpg", Alt: "Camera 1", Canyon: "LCC"},
				{Src: "http://example.com/cam2.jpg", Alt: "Camera 2", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(initialCanyons)

	// Simulate populating the image cache
	testImageBytes := []byte("fake image data")
	testETag := "\"test-etag-123\""

	store.entries[0].Write(func(e *Entry) {
		e.Image = &Image{
			Bytes: testImageBytes,
			ETag:  testETag,
			Src:   e.Camera.Src,
		}
		e.HTTPHeaders = &HTTPHeaders{
			Status:        200,
			ContentType:   "image/jpeg",
			ContentLength: int64(len(testImageBytes)),
			ETag:          testETag,
		}
	})

	// Reload with the same cameras
	reloadedCanyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Src: "http://example.com/cam1.jpg", Alt: "Camera 1 Updated", Canyon: "LCC"},
				{Src: "http://example.com/cam2.jpg", Alt: "Camera 2", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	// Mark store as ready so Get() doesn't wait
	if store.isWaitingOnFirstImageReady.Load() {
		store.isWaitingOnFirstImageReady.Store(false)
		store.imagesReady.Done()
	}

	store.Reload(reloadedCanyons)

	// Verify image cache was preserved for cam1
	cam1ID := store.entries[0].ID
	entry, exists := store.Get(cam1ID)
	require.True(t, exists)
	assert.Equal(t, testImageBytes, entry.Image.Bytes)
	assert.Equal(t, testETag, entry.Image.ETag)
	assert.Equal(t, "Camera 1 Updated", entry.Camera.Alt) // Metadata was updated
}

func TestStore_Reload_HandlesChanges(t *testing.T) {
	// Start with 3 cameras: cam1, cam2, cam3
	initialCanyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Src: "http://example.com/cam1.jpg", Alt: "Camera 1", Canyon: "LCC"},
				{Src: "http://example.com/cam2.jpg", Alt: "Camera 2", Canyon: "LCC"},
				{Src: "http://example.com/cam3.jpg", Alt: "Camera 3", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(initialCanyons)
	assert.Len(t, store.entries, 3)

	// Mark store as ready
	if store.isWaitingOnFirstImageReady.Load() {
		store.isWaitingOnFirstImageReady.Store(false)
		store.imagesReady.Done()
	}

	// Populate image cache for cam1
	testImageBytes := []byte("cam1 image data")
	store.entries[0].Write(func(e *Entry) {
		e.Image = &Image{Bytes: testImageBytes, ETag: "\"etag1\""}
	})

	// Reload with changes:
	// - cam1 stays (should preserve cache)
	// - cam2 modified src (treated as removed + added)
	// - cam3 removed
	// - cam4, cam5 added
	reloadedCanyons := &Canyons{
		LCC: Canyon{
			Name: "LCC Updated",
			Cameras: []Camera{
				{Src: "http://example.com/cam1.jpg", Alt: "Camera 1", Canyon: "LCC"},
				{Src: "http://example.com/cam4.jpg", Alt: "Camera 4", Canyon: "LCC"},
				{Src: "http://example.com/cam5.jpg", Alt: "Camera 5", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store.Reload(reloadedCanyons)

	// Verify counts
	assert.Len(t, store.entries, 3)
	assert.Equal(t, "LCC Updated", store.canyons.LCC.Name)

	// Verify cam1 cache was preserved
	cam1Entry := store.entries[0]
	cam1Entry.Read(func(e *Entry) {
		assert.Equal(t, testImageBytes, e.Image.Bytes)
	})

	// Verify new cameras exist
	foundCam4 := false
	foundCam5 := false
	for _, entry := range store.entries {
		if entry.Camera.Alt == "Camera 4" {
			foundCam4 = true
		}
		if entry.Camera.Alt == "Camera 5" {
			foundCam5 = true
		}
	}
	assert.True(t, foundCam4, "Camera 4 should exist")
	assert.True(t, foundCam5, "Camera 5 should exist")
}

func TestStore_Reload_ThreadSafety(t *testing.T) {
	initialCanyons := &Canyons{
		LCC: Canyon{
			Name: "LCC",
			Cameras: []Camera{
				{Src: "http://example.com/cam1.jpg", Alt: "Camera 1", Canyon: "LCC"},
			},
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(initialCanyons)

	// Populate image cache
	store.entries[0].Write(func(e *Entry) {
		e.Image = &Image{Bytes: []byte("test data"), ETag: "\"etag\""}
	})

	// Mark as ready
	if store.isWaitingOnFirstImageReady.Load() {
		store.isWaitingOnFirstImageReady.Store(false)
		store.imagesReady.Done()
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start concurrent Get calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Continuously call Get
					cam1ID := store.entries[0].ID
					_, exists := store.Get(cam1ID)
					assert.True(t, exists)
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	// Perform reloads while Gets are happening
	for i := 0; i < 5; i++ {
		reloadedCanyons := &Canyons{
			LCC: Canyon{
				Name: "LCC",
				Cameras: []Camera{
					{Src: "http://example.com/cam1.jpg", Alt: "Camera 1", Canyon: "LCC"},
					{Src: "http://example.com/cam2.jpg", Alt: "Camera 2", Canyon: "LCC"},
				},
			},
			BCC: Canyon{Name: "BCC"},
		}
		store.Reload(reloadedCanyons)
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	wg.Wait()

	// Verify final state is consistent
	assert.Len(t, store.entries, 2)
}
