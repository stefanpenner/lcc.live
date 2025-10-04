package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stefanpenner/lcc-live/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*http.Server, *store.Store) {
	// Create a test HTTP server that serves mock images
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("test image"))
		}
	}))
	t.Cleanup(func() { imageServer.Close() })

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "Little Cottonwood Canyon",
			ETag: "\"test-lcc-etag\"",
			Status: store.Camera{
				Kind:   "webcam",
				Src:    imageServer.URL + "/lcc-status.jpg",
				Alt:    "LCC Status",
				Canyon: "LCC",
			},
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/lcc-cam1.jpg",
					Alt:    "LCC Camera 1",
					Canyon: "LCC",
				},
			},
		},
		BCC: store.Canyon{
			Name: "Big Cottonwood Canyon",
			ETag: "\"test-bcc-etag\"",
			Status: store.Camera{
				Kind:   "webcam",
				Src:    imageServer.URL + "/bcc-status.jpg",
				Alt:    "BCC Status",
				Canyon: "BCC",
			},
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/bcc-cam1.jpg",
					Alt:    "BCC Camera 1",
					Canyon: "BCC",
				},
			},
		},
	}

	testStore := store.NewStore(canyons)

	// Fetch images immediately so store.Get() doesn't block
	testStore.FetchImages(context.Background())

	// Create minimal filesystem for templates
	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><head><title>{{.Name}}</title></head><body><h1>{{.Name}}</h1></body></html>`),
		},
	}

	staticFS := fstest.MapFS{
		"test.css": &fstest.MapFile{
			Data: []byte(`body { margin: 0; }`),
		},
	}

	app, err := Start(testStore, staticFS, tmplFS)
	require.NoError(t, err)

	return &http.Server{Handler: app}, testStore
}

func TestHealthCheckRoute(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/healthcheck", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestCanyonRoute_GET_LCC(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Little Cottonwood Canyon")
	assert.Equal(t, "\"test-lcc-etag\"", rec.Header().Get("ETAG"))
	assert.Equal(t, "public, no-cache, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestCanyonRoute_HEAD_LCC(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("HEAD", "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "\"test-lcc-etag\"", rec.Header().Get("ETAG"))
	assert.Equal(t, "public, no-cache, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestCanyonRoute_GET_BCC(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/bcc", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Big Cottonwood Canyon")
	assert.Equal(t, "\"test-bcc-etag\"", rec.Header().Get("ETAG"))
}

func TestCanyonRoute_HEAD_BCC(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("HEAD", "/bcc", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "\"test-bcc-etag\"", rec.Header().Get("ETAG"))
}

func TestImageRoute_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/image/nonexistent", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "image not found")
}

func TestImageRoute_GET_Success(t *testing.T) {
	// Create a test HTTP server that serves mock images
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"mock-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("fake image data"))
		}
	}))
	defer imageServer.Close()

	// Create store with camera pointing to test server
	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "Little Cottonwood Canyon",
			ETag: "\"test-lcc-etag\"",
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: store.Canyon{Name: "BCC"},
	}

	testStore := store.NewStore(canyons)

	// Fetch images to populate the store
	testStore.FetchImages(httptest.NewRequest("GET", "/", nil).Context())

	// Create server
	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	app, err := Start(testStore, staticFS, tmplFS)
	require.NoError(t, err)
	srv := &http.Server{Handler: app}

	// Get the camera ID
	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	req := httptest.NewRequest("GET", "/image/"+cameraID, nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "fake image data", rec.Body.String())
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	assert.Equal(t, "public, max-age=5", rec.Header().Get("Cache-Control"))
}

func TestImageRoute_HEAD_Success(t *testing.T) {
	// Create a test HTTP server that serves mock images
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"mock-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("fake image data"))
		}
	}))
	defer imageServer.Close()

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "Little Cottonwood Canyon",
			ETag: "\"test-lcc-etag\"",
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: store.Canyon{Name: "BCC"},
	}

	testStore := store.NewStore(canyons)
	testStore.FetchImages(httptest.NewRequest("GET", "/", nil).Context())

	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	app, err := Start(testStore, staticFS, tmplFS)
	require.NoError(t, err)
	srv := &http.Server{Handler: app}

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	req := httptest.NewRequest("HEAD", "/image/"+cameraID, nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get("ETag"))
}

func TestImageRoute_NotModified(t *testing.T) {
	// Create a test HTTP server that serves mock images
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"mock-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("fake image data"))
		}
	}))
	defer imageServer.Close()

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "Little Cottonwood Canyon",
			ETag: "\"test-lcc-etag\"",
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/test.jpg",
					Alt:    "Test Camera",
					Canyon: "LCC",
				},
			},
		},
		BCC: store.Canyon{Name: "BCC"},
	}

	testStore := store.NewStore(canyons)
	testStore.FetchImages(httptest.NewRequest("GET", "/", nil).Context())

	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	app, err := Start(testStore, staticFS, tmplFS)
	require.NoError(t, err)
	srv := &http.Server{Handler: app}

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	// Get the ETag from the first request
	snapshot, _ := testStore.Get(cameraID)
	imageETag := snapshot.Image.ETag

	req := httptest.NewRequest("GET", "/image/"+cameraID, nil)
	req.Header.Set("If-None-Match", imageETag)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestStaticFiles(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/s/test.css", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "body")
}

// HTTP Caching Tests

func TestImageRoute_ETagCaching(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"stable-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("test image content"))
		}
	}))
	defer imageServer.Close()

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "LCC",
			ETag: "\"test-lcc-etag\"",
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/test.jpg",
					Alt:    "Test",
					Canyon: "LCC",
				},
			},
		},
		BCC: store.Canyon{Name: "BCC"},
	}

	testStore := store.NewStore(canyons)
	testStore.FetchImages(context.Background())

	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	app, err := Start(testStore, staticFS, tmplFS)
	require.NoError(t, err)
	srv := &http.Server{Handler: app}

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	// First request - should return full image
	req1 := httptest.NewRequest("GET", "/image/"+cameraID, nil)
	rec1 := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)
	etag := rec1.Header().Get("ETag")
	assert.NotEmpty(t, etag)

	// Second request with If-None-Match - should return 304
	req2 := httptest.NewRequest("GET", "/image/"+cameraID, nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotModified, rec2.Code)
	assert.Empty(t, rec2.Body.String())
}

func TestImageRoute_CacheHeaders(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("image data"))
		}
	}))
	defer imageServer.Close()

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "LCC",
			ETag: "\"test-lcc-etag\"",
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/test.jpg",
					Alt:    "Test",
					Canyon: "LCC",
				},
			},
		},
		BCC: store.Canyon{Name: "BCC"},
	}

	testStore := store.NewStore(canyons)
	testStore.FetchImages(context.Background())

	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	app, err := Start(testStore, staticFS, tmplFS)
	require.NoError(t, err)
	srv := &http.Server{Handler: app}

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	req := httptest.NewRequest("GET", "/image/"+cameraID, nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=5", rec.Header().Get("Cache-Control"))
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	assert.NotEmpty(t, rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get("Content-Length"))
}

func TestCanyonRoute_CacheHeaders(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, no-cache, must-revalidate", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "\"test-lcc-etag\"", rec.Header().Get("ETAG"))
}

func TestCanyonRoute_ETag_NotModified(t *testing.T) {
	srv, _ := setupTestServer(t)

	// First request to get the ETag
	req1 := httptest.NewRequest("GET", "/", nil)
	rec1 := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec1, req1)

	etag := rec1.Header().Get("ETAG")
	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.NotEmpty(t, etag)

	// Second request with matching ETag should return 304
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotModified, rec2.Code)
	assert.Empty(t, rec2.Body.String())
}

func TestCanyonRoute_ETag_Modified(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Request with wrong ETag should return full content
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("If-None-Match", "\"wrong-etag\"")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Little Cottonwood Canyon")
}

func TestImageRoute_WrongETag(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"correct-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("image data"))
		}
	}))
	defer imageServer.Close()

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "LCC",
			ETag: "\"test-lcc-etag\"",
			Cameras: []store.Camera{
				{
					Kind:   "webcam",
					Src:    imageServer.URL + "/test.jpg",
					Alt:    "Test",
					Canyon: "LCC",
				},
			},
		},
		BCC: store.Canyon{Name: "BCC"},
	}

	testStore := store.NewStore(canyons)
	testStore.FetchImages(context.Background())

	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	app, err := Start(testStore, staticFS, tmplFS)
	require.NoError(t, err)
	srv := &http.Server{Handler: app}

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	// Request with wrong ETag should return full content
	req := httptest.NewRequest("GET", "/image/"+cameraID, nil)
	req.Header.Set("If-None-Match", "\"wrong-etag\"")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.String())
}

func TestMetricsEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/_/metrics", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Verify Prometheus metrics are present
	assert.Contains(t, body, "lcc_image_fetch_total")
	assert.Contains(t, body, "lcc_store_entries_total")
	assert.Contains(t, body, "lcc_cameras_total")
	assert.Contains(t, body, "lcc_images_ready")
	assert.Contains(t, body, "# HELP")
	assert.Contains(t, body, "# TYPE")
}
