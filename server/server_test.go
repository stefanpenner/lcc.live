package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stefanpenner/lcc-live/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) *http.Server {
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
		"camera.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><head><title>{{.Camera.Alt}}</title></head><body><h1>{{.Camera.Alt}}</h1></body></html>`),
		},
	}

	staticFS := fstest.MapFS{
		"test.css": &fstest.MapFile{
			Data: []byte(`body { margin: 0; }`),
		},
	}

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
	require.NoError(t, err)

	return &http.Server{Handler: app}
}

func TestHealthCheckRoute(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/healthcheck", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestHealthCheckStates(t *testing.T) {
	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
		"camera.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Camera.Alt}}</body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	tests := []struct {
		name           string
		setupStore     func() *store.Store
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "not ready - images not fetched",
			setupStore: func() *store.Store {
				imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.Header().Set("ETag", "\"test-etag\"")
					if r.Method == "GET" {
						w.Write([]byte("test image"))
					}
				}))
				t.Cleanup(imageServer.Close)

				canyons := &store.Canyons{
					LCC: store.Canyon{
						Name: "Little Cottonwood Canyon",
						Cameras: []store.Camera{
							{Kind: "webcam", Src: imageServer.URL + "/test.jpg", Alt: "Test Camera", Canyon: "LCC"},
						},
					},
					BCC: store.Canyon{Name: "Big Cottonwood Canyon"},
				}
				// Don't fetch images - store should not be ready
				return store.NewStore(canyons)
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "not ready",
		},
		{
			name: "ready - images fetched",
			setupStore: func() *store.Store {
				imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "image/jpeg")
					w.Header().Set("ETag", "\"test-etag\"")
					if r.Method == "GET" {
						w.Write([]byte("test image"))
					}
				}))
				t.Cleanup(imageServer.Close)

				canyons := &store.Canyons{
					LCC: store.Canyon{
						Name: "Little Cottonwood Canyon",
						Cameras: []store.Camera{
							{Kind: "webcam", Src: imageServer.URL + "/test.jpg", Alt: "Test Camera", Canyon: "LCC"},
						},
					},
					BCC: store.Canyon{Name: "Big Cottonwood Canyon"},
				}
				testStore := store.NewStore(canyons)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				testStore.FetchImages(ctx)
				return testStore
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "no cameras configured",
			setupStore: func() *store.Store {
				canyons := &store.Canyons{
					LCC: store.Canyon{Name: "Little Cottonwood Canyon"},
					BCC: store.Canyon{Name: "Big Cottonwood Canyon"},
				}
				testStore := store.NewStore(canyons)
				testStore.FetchImages(context.Background())
				return testStore
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "No cameras configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testStore := tt.setupStore()
			app, err := Start(ServerConfig{
				Store:         testStore,
				StaticFS:      staticFS,
				TemplateFS:    tmplFS,
				DevMode:       false,
				SentryEnabled: false,
			})
			require.NoError(t, err)

			req := httptest.NewRequest("GET", "/healthcheck", nil)
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.expectedBody)
		})
	}
}

func TestCanyonRoute_GET_LCC(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Little Cottonwood Canyon")
	// ETag should contain base ETag, version, and format suffix
	etag := rec.Header().Get("ETag")
	assert.Contains(t, etag, "\"test-lcc-etag\"")
	assert.Contains(t, etag, "-html")
	assert.Equal(t, "public, max-age=30, stale-while-revalidate=60, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestCanyonRoute_HEAD_LCC(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("HEAD", "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	// ETag should contain base ETag, version, and format suffix
	etag := rec.Header().Get("ETag")
	assert.Contains(t, etag, "\"test-lcc-etag\"")
	assert.Contains(t, etag, "-html")
	assert.Equal(t, "public, max-age=30, stale-while-revalidate=60, must-revalidate", rec.Header().Get("Cache-Control"))
}

func TestCanyonRoute_GET_BCC(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/bcc", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Big Cottonwood Canyon")
	// ETag should contain base ETag, version, and format suffix
	etag := rec.Header().Get("ETag")
	assert.Contains(t, etag, "\"test-bcc-etag\"")
	assert.Contains(t, etag, "-html")
}

func TestCanyonRoute_HEAD_BCC(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("HEAD", "/bcc", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	// ETag should contain base ETag, version, and format suffix
	etag := rec.Header().Get("ETag")
	assert.Contains(t, etag, "\"test-bcc-etag\"")
	assert.Contains(t, etag, "-html")
}

func TestImageRoute_NotFound(t *testing.T) {
	srv := setupTestServer(t)

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

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
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
	assert.Equal(t, "public, max-age=10, stale-while-revalidate=20", rec.Header().Get("Cache-Control"))
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

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
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

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
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
	srv := setupTestServer(t)

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

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
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

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
	require.NoError(t, err)
	srv := &http.Server{Handler: app}

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	req := httptest.NewRequest("GET", "/image/"+cameraID, nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=10, stale-while-revalidate=20", rec.Header().Get("Cache-Control"))
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	assert.NotEmpty(t, rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get("Content-Length"))
}

func TestCanyonRoute_CacheHeaders(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public, max-age=30, stale-while-revalidate=60, must-revalidate", rec.Header().Get("Cache-Control"))
	// ETag should contain version
	etag := rec.Header().Get("ETag")
	assert.Contains(t, etag, "\"test-lcc-etag\"")
	assert.Contains(t, etag, "-html")
}

func TestCanyonRoute_ETag_NotModified(t *testing.T) {
	srv := setupTestServer(t)

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
	srv := setupTestServer(t)

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

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
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
	srv := setupTestServer(t)

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

func TestInternalEndpointsCacheHeaders(t *testing.T) {
	srv := setupTestServer(t)

	tests := []struct {
		name string
		path string
	}{
		{"version", "/_/version"},
		{"metrics", "/_/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			srv.Handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			// Verify that internal endpoints are uncachable
			assert.Equal(t, "no-store, no-cache, must-revalidate, private, max-age=0", rec.Header().Get("Cache-Control"))
			assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
			assert.Equal(t, "0", rec.Header().Get("Expires"))
		})
	}
}

// JSON Response Tests

func TestCanyonRoute_GET_JSON_LCC(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/.json", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	// ETag should contain base ETag, version, and format suffix
	etag := rec.Header().Get("ETag")
	assert.Contains(t, etag, "\"test-lcc-etag\"")
	assert.Contains(t, etag, "-json")
	assert.Equal(t, "public, max-age=30, stale-while-revalidate=60, must-revalidate", rec.Header().Get("Cache-Control"))

	// Verify JSON structure
	body := rec.Body.String()
	assert.Contains(t, body, `"name":"Little Cottonwood Canyon"`)
	assert.Contains(t, body, `"etag":"\"test-lcc-etag\""`)
}

func TestCanyonRoute_GET_JSON_BCC(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/bcc.json", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	// ETag should contain base ETag, version, and format suffix
	etag := rec.Header().Get("ETag")
	assert.Contains(t, etag, "\"test-bcc-etag\"")
	assert.Contains(t, etag, "-json")

	body := rec.Body.String()
	assert.Contains(t, body, `"name":"Big Cottonwood Canyon"`)
	assert.Contains(t, body, `"etag":"\"test-bcc-etag\""`)
}

func TestCanyonRoute_JSON_ETag_NotModified(t *testing.T) {
	srv := setupTestServer(t)

	// First request to get the ETag
	req1 := httptest.NewRequest("GET", "/.json", nil)
	rec1 := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec1, req1)

	etag := rec1.Header().Get("ETag")
	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.NotEmpty(t, etag)

	// Second request with matching ETag should return 304
	req2 := httptest.NewRequest("GET", "/.json", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotModified, rec2.Code)
	assert.Empty(t, rec2.Body.String())
}

func TestCanyonRoute_JSON_Extension(t *testing.T) {
	srv := setupTestServer(t)

	testCases := []struct {
		name       string
		path       string
		expectJSON bool
	}{
		{"root with .json", "/.json", true},
		{"bcc with .json", "/bcc.json", true},
		{"root without .json", "/", false},
		{"bcc without .json", "/bcc", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			rec := httptest.NewRecorder()

			srv.Handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			if tc.expectJSON {
				assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
			} else {
				assert.NotContains(t, rec.Header().Get("Content-Type"), "application/json")
				assert.Contains(t, rec.Body.String(), "<!DOCTYPE html>")
			}
		})
	}
}

func TestCameraRoute(t *testing.T) {
	// Create shared test server and store
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("test image data"))
		}
	}))
	t.Cleanup(imageServer.Close)

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "Little Cottonwood Canyon",
			Cameras: []store.Camera{
				{Kind: "webcam", Src: imageServer.URL + "/test.jpg", Alt: "Test Camera", Canyon: "LCC"},
			},
		},
		BCC: store.Canyon{Name: "Big Cottonwood Canyon"},
	}

	testStore := store.NewStore(canyons)
	testStore.FetchImages(context.Background())

	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>{{.Name}}</body></html>`),
		},
		"camera.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><head><title>{{.Camera.Alt}}</title></head><body><h1>{{.Camera.Alt}}</h1><img src="{{.ImageURL}}" /></body></html>`),
		},
	}
	staticFS := fstest.MapFS{}

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
	require.NoError(t, err)

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	tests := []struct {
		name           string
		method         string
		path           string
		headers        map[string]string
		expectedStatus int
		checkResponse  func(t *testing.T, rec *httptest.ResponseRecorder)
	}{
		{
			name:           "GET HTML success",
			method:         "GET",
			path:           "/camera/" + cameraID,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "Test Camera")
				assert.Contains(t, rec.Body.String(), "/image/"+cameraID)
			},
		},
		{
			name:           "GET JSON success",
			method:         "GET",
			path:           "/camera/" + cameraID + ".json",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
				assert.Contains(t, rec.Body.String(), "Test Camera")
				assert.Contains(t, rec.Body.String(), cameraID)
			},
		},
		{
			name:           "HEAD request",
			method:         "HEAD",
			path:           "/camera/" + cameraID,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Empty(t, rec.Body.String())
				assert.NotEmpty(t, rec.Header().Get("ETag"))
			},
		},
		{
			name:           "not found",
			method:         "GET",
			path:           "/camera/nonexistent",
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "Camera not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			app.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}

	// Test ETag Not Modified separately (requires two requests)
	t.Run("ETag not modified", func(t *testing.T) {
		// First request to get ETag
		req1 := httptest.NewRequest("GET", "/camera/"+cameraID, nil)
		rec1 := httptest.NewRecorder()
		app.ServeHTTP(rec1, req1)
		etag := rec1.Header().Get("ETag")

		// Second request with If-None-Match
		req2 := httptest.NewRequest("GET", "/camera/"+cameraID, nil)
		req2.Header.Set("If-None-Match", etag)
		rec2 := httptest.NewRecorder()
		app.ServeHTTP(rec2, req2)

		assert.Equal(t, http.StatusNotModified, rec2.Code)
		assert.Empty(t, rec2.Body.String())
	})
}
