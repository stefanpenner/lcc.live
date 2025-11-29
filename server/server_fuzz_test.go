package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stefanpenner/lcc-live/store"
)

// FuzzImageRoute tests the image route with various IDs to ensure no crashes
func FuzzImageRoute(f *testing.F) {
	// Seed corpus with various interesting inputs
	f.Add("valid-id")
	f.Add("")
	f.Add("../../../etc/passwd")
	f.Add("image/../../secret")
	f.Add(string([]byte{0x00, 0x01, 0x02}))
	f.Add("a" + string(make([]byte, 10000)))
	f.Add("../../")
	f.Add("\\..\\..\\")
	f.Add("%2e%2e%2f")
	f.Add("<script>alert('xss')</script>")
	f.Add("'; DROP TABLE images;--")

	// Create test server setup
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("test image"))
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
	if err != nil {
		f.Fatal(err)
	}
	srv := &http.Server{Handler: app}

	f.Fuzz(func(t *testing.T, imageID string) {
		// Test should never panic, regardless of input
		defer func() {
			if r := recover(); r != nil {
				// Check if panic is from invalid URL creation
				panicStr := fmt.Sprintf("%v", r)
				if bytes.Contains([]byte(panicStr), []byte("invalid control character in URL")) ||
					bytes.Contains([]byte(panicStr), []byte("invalid NewRequest")) {
					return
				}
				t.Errorf("ImageRoute panicked with input %q: %v", imageID, r)
			}
		}()

		req := httptest.NewRequest("GET", "/image/"+imageID, nil)
		rec := httptest.NewRecorder()

		srv.Handler.ServeHTTP(rec, req)

		// Validate response is always valid HTTP
		if rec.Code == 0 {
			t.Error("Response code is 0, expected valid HTTP status")
		}

		// Status should be in valid range
		if rec.Code < 100 || rec.Code >= 600 {
			t.Errorf("Invalid HTTP status code: %d", rec.Code)
		}

		// Headers should be valid
		if rec.Header().Get("Content-Type") != "" {
			contentType := rec.Header().Get("Content-Type")
			if len(contentType) > 1000 {
				t.Error("Content-Type header suspiciously long")
			}
		}
	})
}

// FuzzCanyonRoute tests the canyon route with various canyon IDs
func FuzzCanyonRoute(f *testing.F) {
	// Seed corpus
	f.Add("LCC")
	f.Add("BCC")
	f.Add("")
	f.Add("INVALID")
	f.Add("../../../etc/passwd")
	f.Add(string([]byte{0x00, 0x01, 0x02}))
	f.Add("<script>alert('xss')</script>")

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("test image"))
		}
	}))
	defer imageServer.Close()

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
	testStore.FetchImages(context.Background())

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

	app, err := Start(ServerConfig{
		Store:         testStore,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       false,
		SentryEnabled: false,
	})
	if err != nil {
		f.Fatal(err)
	}
	srv := &http.Server{Handler: app}

	f.Fuzz(func(t *testing.T, canyonPath string) {
		defer func() {
			if r := recover(); r != nil {
				// Check if panic is from invalid URL creation (control characters, etc)
				// This is expected - HTTP server would reject these before reaching handler
				panicStr := fmt.Sprintf("%v", r)
				if bytes.Contains([]byte(panicStr), []byte("invalid control character in URL")) ||
					bytes.Contains([]byte(panicStr), []byte("invalid NewRequest")) {
					// This is expected behavior - skip
					return
				}
				t.Errorf("CanyonRoute panicked with input %q: %v", canyonPath, r)
			}
		}()

		req := httptest.NewRequest("GET", "/"+canyonPath, nil)
		rec := httptest.NewRecorder()

		srv.Handler.ServeHTTP(rec, req)

		// Validate response
		if rec.Code == 0 {
			t.Error("Response code is 0, expected valid HTTP status")
		}

		if rec.Code < 100 || rec.Code >= 600 {
			t.Errorf("Invalid HTTP status code: %d", rec.Code)
		}

		// For successful responses, check for HTML validity markers
		if rec.Code == http.StatusOK {
			body := rec.Body.String()
			// Should not contain null bytes
			if bytes.Contains([]byte(body), []byte{0x00}) {
				t.Error("Response contains null bytes")
			}
		}
	})
}

// FuzzHTTPHeaders tests various HTTP headers with fuzzing
func FuzzHTTPHeaders(f *testing.F) {
	// Seed corpus with various header values
	f.Add("image/jpeg", "\"etag123\"")
	f.Add("", "")
	f.Add("application/octet-stream", "W/\"weak-etag\"")
	f.Add(string([]byte{0x00, 0x01}), string([]byte{0xFF, 0xFE}))
	f.Add("image/jpeg; charset=utf-8", "\"etag\"")
	f.Add("<script>alert('xss')</script>", "'; DROP TABLE;--")

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("test image"))
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
	if err != nil {
		f.Fatal(err)
	}
	srv := &http.Server{Handler: app}

	cameraID := testStore.Canyon("LCC").Cameras[0].ID

	f.Fuzz(func(t *testing.T, userAgent string, ifNoneMatch string) {
		defer func() {
			if r := recover(); r != nil {
				panicStr := fmt.Sprintf("%v", r)
				if bytes.Contains([]byte(panicStr), []byte("invalid control character")) ||
					bytes.Contains([]byte(panicStr), []byte("invalid NewRequest")) {
					return
				}
				t.Errorf("Handler panicked with headers User-Agent=%q If-None-Match=%q: %v",
					userAgent, ifNoneMatch, r)
			}
		}()

		req := httptest.NewRequest("GET", "/image/"+cameraID, nil)
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("If-None-Match", ifNoneMatch)
		rec := httptest.NewRecorder()

		srv.Handler.ServeHTTP(rec, req)

		// Validate response
		if rec.Code == 0 {
			t.Error("Response code is 0")
		}

		if rec.Code < 100 || rec.Code >= 600 {
			t.Errorf("Invalid HTTP status code: %d", rec.Code)
		}

		// Check headers don't contain dangerous characters
		for key, values := range rec.Header() {
			for _, value := range values {
				if bytes.Contains([]byte(value), []byte{0x00}) {
					t.Errorf("Response header %q contains null bytes", key)
				}
			}
		}
	})
}

// FuzzStaticFiles tests static file serving with various paths
func FuzzStaticFiles(f *testing.F) {
	// Seed corpus
	f.Add("test.css")
	f.Add("../../../etc/passwd")
	f.Add("test.css/../../secret")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("test.css%00.txt")
	f.Add("\\..\\..\\windows\\system32")

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", "\"test-etag\"")
		if r.Method == "GET" {
			w.Write([]byte("test image"))
		}
	}))
	defer imageServer.Close()

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "LCC",
			ETag: "\"test-lcc-etag\"",
		},
		BCC: store.Canyon{Name: "BCC"},
	}

	testStore := store.NewStore(canyons)

	tmplFS := fstest.MapFS{
		"canyon.html.tmpl": &fstest.MapFile{
			Data: []byte(`<!DOCTYPE html><html><body>test</body></html>`),
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
	if err != nil {
		f.Fatal(err)
	}
	srv := &http.Server{Handler: app}

	f.Fuzz(func(t *testing.T, filePath string) {
		defer func() {
			if r := recover(); r != nil {
				panicStr := fmt.Sprintf("%v", r)
				if bytes.Contains([]byte(panicStr), []byte("invalid control character in URL")) ||
					bytes.Contains([]byte(panicStr), []byte("invalid NewRequest")) {
					return
				}
				t.Errorf("Static file handler panicked with path %q: %v", filePath, r)
			}
		}()

		req := httptest.NewRequest("GET", "/s/"+filePath, nil)
		rec := httptest.NewRecorder()

		srv.Handler.ServeHTTP(rec, req)

		// Should always return valid status
		if rec.Code == 0 {
			t.Error("Response code is 0")
		}

		if rec.Code < 100 || rec.Code >= 600 {
			t.Errorf("Invalid HTTP status code: %d", rec.Code)
		}

		// If successful, body should not contain null bytes
		if rec.Code == http.StatusOK {
			if bytes.Contains(rec.Body.Bytes(), []byte{0x00}) {
				t.Error("Response contains null bytes")
			}
		}
	})
}
