package store

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// FuzzStoreCameraID tests the store with various camera IDs
func FuzzStoreCameraID(f *testing.F) {
	// Seed corpus
	f.Add("valid-camera-id")
	f.Add("")
	f.Add("../../../secret")
	f.Add(string([]byte{0x00, 0x01, 0x02}))
	f.Add("<script>alert('xss')</script>")
	f.Add("'; DROP TABLE cameras;--")
	f.Add("a" + string(make([]byte, 10000)))

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

	f.Fuzz(func(t *testing.T, cameraID string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Store.Get panicked with ID %q: %v", cameraID, r)
			}
		}()

		// Should never panic, even with invalid IDs
		entry, exists := store.Get(cameraID)

		// Validate the returned data
		if exists {
			if entry.Image == nil {
				t.Error("Entry exists but Image is nil")
			}
			if entry.HTTPHeaders == nil {
				t.Error("Entry exists but HTTPHeaders is nil")
			}
			if entry.Camera == nil {
				t.Error("Entry exists but Camera is nil")
			}

			// Check for null bytes in returned data
			if entry.Image != nil && bytes.Contains(entry.Image.Bytes, []byte{0x00, 0x00, 0x00, 0x00, 0x00}) {
				// Only flag if there are many null bytes (might be valid in binary data)
				nullCount := 0
				for _, b := range entry.Image.Bytes {
					if b == 0x00 {
						nullCount++
					}
				}
				if nullCount > len(entry.Image.Bytes)/2 {
					t.Error("Image contains suspicious number of null bytes")
				}
			}
		}
	})
}

// FuzzImageData tests the store with various image data
func FuzzImageData(f *testing.F) {
	// Seed corpus with various types of data
	f.Add([]byte("valid image data"))
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0}) // JPEG header
	f.Add([]byte{0x89, 0x50, 0x4E, 0x47}) // PNG header
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	f.Add(make([]byte, 10000)) // Large empty buffer

	f.Fuzz(func(t *testing.T, imageData []byte) {
		// Limit size to avoid OOM
		if len(imageData) > 10*1024*1024 {
			t.Skip("Image data too large")
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("ETag", "\"test-etag\"")
			if r.Method == "GET" {
				w.Write(imageData)
			}
		}))
		defer server.Close()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Store panicked with image data of length %d: %v", len(imageData), r)
			}
		}()

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

		// Should handle any image data without crashing
		store.FetchImages(ctx)

		// Verify we can read the data
		cameraID := store.entries[0].Camera.ID
		entry, exists := store.Get(cameraID)

		if !exists {
			t.Error("Camera entry should exist")
		}

		if entry.Image == nil {
			t.Error("Image should not be nil")
		}

		// Verify the stored data matches what we sent
		if exists && entry.Image != nil && !bytes.Equal(entry.Image.Bytes, imageData) {
			t.Errorf("Stored image data doesn't match. Expected %d bytes, got %d bytes",
				len(imageData), len(entry.Image.Bytes))
		}
	})
}

// FuzzConcurrentAccess tests concurrent operations with fuzzing
func FuzzConcurrentAccess(f *testing.F) {
	// Seed with number of goroutines
	f.Add(int32(10), int32(5))
	f.Add(int32(100), int32(50))
	f.Add(int32(1), int32(1))
	f.Add(int32(1000), int32(500))

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

	f.Fuzz(func(t *testing.T, numReaders int32, numWriters int32) {
		// Limit concurrency to avoid resource exhaustion
		if numReaders < 0 || numReaders > 500 {
			t.Skip("Invalid number of readers")
		}
		if numWriters < 0 || numWriters > 100 {
			t.Skip("Invalid number of writers")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Concurrent access panicked with readers=%d writers=%d: %v",
					numReaders, numWriters, r)
			}
		}()

		store := NewStore(canyons)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Initial fetch
		store.FetchImages(ctx)

		cameraID := store.entries[0].Camera.ID
		var wg sync.WaitGroup

		// Start readers
		for i := int32(0); i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				entry, exists := store.Get(cameraID)
				if !exists {
					t.Error("Camera entry should exist")
				}
				if entry.Image == nil {
					t.Error("Image should not be nil")
				}
			}()
		}

		// Start writers (fetchers)
		for i := int32(0); i < numWriters; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				store.FetchImages(ctx)
			}()
		}

		wg.Wait()
	})
}

// FuzzHTTPResponseHeaders tests various HTTP response headers
func FuzzHTTPResponseHeaders(f *testing.F) {
	// Seed corpus with various header combinations
	f.Add("image/jpeg", "\"etag123\"", int64(1024))
	f.Add("", "", int64(0))
	f.Add("text/html", "W/\"weak\"", int64(-1))
	f.Add("<script>", "'; DROP", int64(999999999))

	f.Fuzz(func(t *testing.T, contentType string, etag string, contentLength int64) {
		// Limit sizes to avoid resource issues
		if contentLength > 100*1024*1024 || contentLength < -1 {
			t.Skip("Invalid content length")
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("ETag", etag)
			if r.Method == "GET" {
				w.Write([]byte("test data"))
			}
		}))
		defer server.Close()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Store panicked with headers ContentType=%q ETag=%q ContentLength=%d: %v",
					contentType, etag, contentLength, r)
			}
		}()

		canyons := &Canyons{
			LCC: Canyon{
				Name: "LCC",
				Cameras: []Camera{
					{
						Kind:   "webcam",
						Src:    server.URL + "/test.jpg",
						Alt:    "Test",
						Canyon: "LCC",
					},
				},
			},
			BCC: Canyon{Name: "BCC"},
		}

		store := NewStore(canyons)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Should handle any headers without crashing
		store.FetchImages(ctx)

		// Verify we can read the entry
		cameraID := store.entries[0].Camera.ID
		entry, exists := store.Get(cameraID)

		if !exists {
			t.Error("Camera entry should exist")
		}

		// Validate stored headers don't contain dangerous characters
		if entry.HTTPHeaders != nil {
			if bytes.Contains([]byte(entry.HTTPHeaders.ContentType), []byte{0x00}) {
				t.Error("ContentType contains null bytes")
			}
			if bytes.Contains([]byte(entry.HTTPHeaders.ETag), []byte{0x00}) {
				t.Error("ETag contains null bytes")
			}
		}
	})
}

// FuzzCameraURL tests various camera source URLs
func FuzzCameraURL(f *testing.F) {
	// Seed corpus
	f.Add("http://example.com/camera.jpg")
	f.Add("https://example.com/cam")
	f.Add("")
	f.Add("javascript:alert('xss')")
	f.Add("file:///etc/passwd")
	f.Add("http://localhost:1234/../../../secret")
	f.Add("http://[::1]/test")
	f.Add("http://user:pass@host/path")
	f.Add("http://" + string(make([]byte, 1000)) + ".com/test")

	f.Fuzz(func(t *testing.T, cameraURL string) {
		// Skip extremely long URLs
		if len(cameraURL) > 2000 {
			t.Skip("URL too long")
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Store panicked with camera URL %q: %v", cameraURL, r)
			}
		}()

		canyons := &Canyons{
			LCC: Canyon{
				Name: "LCC",
				Cameras: []Camera{
					{
						Kind:   "webcam",
						Src:    cameraURL,
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

		// Should not crash with any URL
		store.FetchImages(ctx)

		// Verify store is still functional
		if len(store.entries) != 1 {
			t.Errorf("Expected 1 entry, got %d", len(store.entries))
		}

		// Note: Camera ID is generated from the src URL, so empty src = empty ID
		// This is expected behavior, not an error

		// Should be able to attempt to get the entry without crashing
		if store.entries[0].Camera.ID != "" {
			cameraID := store.entries[0].Camera.ID
			_, _ = store.Get(cameraID)
		}
	})
}
