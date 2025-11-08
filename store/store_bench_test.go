package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func BenchmarkStore_Get(b *testing.B) {
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
	store.FetchImages(context.Background())

	cameraID := store.entries[0].Camera.ID

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, exists := store.Get(cameraID)
			if !exists {
				b.Fatal("camera not found")
			}
		}
	})
}

func BenchmarkStore_ShallowSnapshot(b *testing.B) {
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
	store.FetchImages(context.Background())

	entry := store.entries[0]

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = entry.ShallowSnapshot()
		}
	})
}

func BenchmarkStore_FetchImages_SingleCamera(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write(make([]byte, 1024*100)) // 100KB image
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
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.FetchImages(ctx)
	}
}

func BenchmarkStore_FetchImages_MultipleCamera(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write(make([]byte, 1024*50)) // 50KB image
		}
	}))
	defer server.Close()

	cameras := make([]Camera, 10)
	for i := 0; i < 10; i++ {
		cameras[i] = Camera{
			Kind:   "webcam",
			Src:    server.URL + "/test.jpg",
			Alt:    "Test Camera",
			Canyon: "LCC",
		}
	}

	canyons := &Canyons{
		LCC: Canyon{
			Name:    "LCC",
			Cameras: cameras,
		},
		BCC: Canyon{Name: "BCC"},
	}

	store := NewStore(canyons)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.FetchImages(ctx)
	}
}

func BenchmarkStore_ConcurrentGetAndFetch(b *testing.B) {
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
	store.FetchImages(context.Background())

	cameraID := store.entries[0].Camera.ID
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 90% reads, 10% writes
			if b.N%10 == 0 {
				store.FetchImages(ctx)
			} else {
				_, exists := store.Get(cameraID)
				if !exists {
					b.Fatal("camera not found")
				}
			}
		}
	})
}

func BenchmarkCanyons_Load(b *testing.B) {
	jsonData := []byte(`{
		"lcc": {
			"name": "Little Cottonwood Canyon",
			"status": {
				"id": "lcc-status",
				"kind": "status",
				"src": "https://example.com/lcc-status.jpg",
				"alt": "LCC Status",
				"canyon": "lcc"
			},
			"cameras": [
				{
					"id": "lcc-cam1",
					"kind": "camera",
					"src": "https://example.com/lcc-cam1.jpg",
					"alt": "LCC Camera 1",
					"canyon": "lcc"
				}
			]
		},
		"bcc": {
			"name": "Big Cottonwood Canyon",
			"status": {
				"id": "bcc-status",
				"kind": "status",
				"src": "https://example.com/bcc-status.jpg",
				"alt": "BCC Status",
				"canyon": "bcc"
			},
			"cameras": []
		}
	}`)

	testFS := fstest.MapFS{
		"seed.json": &fstest.MapFile{
			Data: jsonData,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var canyons Canyons
		if err := canyons.Load(testFS, "seed.json"); err != nil {
			b.Fatal(err)
		}
	}
}
