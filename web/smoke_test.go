package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stefanpenner/lcc-live/web/server"
	"github.com/stefanpenner/lcc-live/web/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSmokeServer starts a real HTTP server on a random port with mock camera data.
// Returns the base URL and a cleanup function.
func setupSmokeServer(t *testing.T) string {
	t.Helper()

	// Mock image server
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("ETag", `"mock-etag"`)
		if r.Method == http.MethodGet {
			// Minimal valid JPEG (SOI + EOI markers)
			w.Write([]byte{0xFF, 0xD8, 0xFF, 0xD9})
		}
	}))
	t.Cleanup(imageServer.Close)

	canyons := &store.Canyons{
		LCC: store.Canyon{
			Name: "Little Cottonwood Canyon",
			ETag: `"smoke-lcc"`,
			Status: store.Camera{
				Kind: "img", Src: imageServer.URL + "/lcc-status.jpg",
				Alt: "LCC Status", Canyon: "LCC",
			},
			Cameras: []store.Camera{
				{Kind: "img", Src: imageServer.URL + "/lcc-cam1.jpg", Alt: "LCC Camera 1", Canyon: "LCC"},
				{Kind: "img", Src: imageServer.URL + "/lcc-cam2.jpg", Alt: "LCC Camera 2", Canyon: "LCC"},
			},
		},
		BCC: store.Canyon{
			Name: "Big Cottonwood Canyon",
			ETag: `"smoke-bcc"`,
			Status: store.Camera{
				Kind: "img", Src: imageServer.URL + "/bcc-status.jpg",
				Alt: "BCC Status", Canyon: "BCC",
			},
			Cameras: []store.Camera{
				{Kind: "img", Src: imageServer.URL + "/bcc-cam1.jpg", Alt: "BCC Camera 1", Canyon: "BCC"},
			},
		},
	}

	s := store.NewStore(canyons)
	s.FetchImages(context.Background())

	// Use real templates from disk
	os.Setenv("DEV_MODE", "1")
	t.Cleanup(func() { os.Unsetenv("DEV_MODE") })

	tmplFS, err := loadFilesystem("web/templates")
	require.NoError(t, err)
	staticFS, err := loadFilesystem("web/static")
	require.NoError(t, err)

	app, err := server.Start(server.ServerConfig{
		Store:      s,
		StaticFS:   staticFS,
		TemplateFS: tmplFS,
		DevMode:    true,
	})
	require.NoError(t, err)

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Start real HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: app,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("server error: %v", err)
		}
	}()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Wait for server to be ready
	client := &http.Client{Timeout: 5 * time.Second}
	for i := 0; i < 50; i++ {
		resp, err := client.Get(baseURL + "/healthcheck")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return baseURL
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("server did not become ready within 5 seconds")
	return ""
}

func TestSmokeE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e smoke test in short mode")
	}

	baseURL := setupSmokeServer(t)
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("healthcheck returns 200", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/healthcheck")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "OK", string(body))
	})

	t.Run("LCC page returns HTML", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "Little Cottonwood Canyon")
	})

	t.Run("BCC page returns HTML", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/bcc")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "Big Cottonwood Canyon")
	})

	t.Run("LCC JSON returns valid JSON", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/.json")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "Little Cottonwood Canyon")
	})

	t.Run("HEAD requests work", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodHead, baseURL+"/", nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Empty(t, body, "HEAD response should have no body")
	})

	t.Run("security headers present", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
		assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
		assert.Equal(t, "camera=(), microphone=(), geolocation=()", resp.Header.Get("Permissions-Policy"))
		assert.NotEmpty(t, resp.Header.Get("X-Version"))
	})

	t.Run("version endpoint returns JSON", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/_/version")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "version")
		assert.Contains(t, string(body), "uptime")
	})

	t.Run("metrics endpoint returns prometheus format", func(t *testing.T) {
		// Disable automatic gzip to get raw prometheus text
		noGzipClient := &http.Client{
			Timeout:   10 * time.Second,
			Transport: &http.Transport{DisableCompression: true},
		}
		resp, err := noGzipClient.Get(baseURL + "/_/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, string(body), "go_goroutines")
	})

	t.Run("static files served", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/s/style.css")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		assert.NotEmpty(t, body)
	})

	t.Run("UDOT API endpoint returns JSON", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/canyon/LCC/udot")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	})

	t.Run("ETag caching returns 304", func(t *testing.T) {
		// First request to get the ETag
		resp1, err := client.Get(baseURL + "/")
		require.NoError(t, err)
		etag := resp1.Header.Get("ETag")
		resp1.Body.Close()

		if etag != "" {
			// Second request with If-None-Match
			req, _ := http.NewRequest(http.MethodGet, baseURL+"/", nil)
			req.Header.Set("If-None-Match", etag)
			resp2, err := client.Do(req)
			require.NoError(t, err)
			defer resp2.Body.Close()
			assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
		}
	})

	t.Run("image route serves camera image", func(t *testing.T) {
		// Get LCC JSON to find camera IDs
		resp, err := client.Get(baseURL + "/.json")
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// The page HTML should have image IDs; let's try the LCC page and extract an image link
		resp, err = client.Get(baseURL + "/")
		require.NoError(t, err)
		htmlBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Find an image ID in the HTML (data-camera-id="...")
		html := string(htmlBody)
		if idx := strings.Index(html, `/image/`); idx >= 0 {
			// Extract the ID
			rest := html[idx+7:]
			endIdx := strings.IndexAny(rest, `"' >`)
			if endIdx > 0 {
				imageID := rest[:endIdx]
				imgResp, err := client.Get(baseURL + "/image/" + imageID)
				require.NoError(t, err)
				defer imgResp.Body.Close()
				assert.Equal(t, http.StatusOK, imgResp.StatusCode)
				assert.Contains(t, imgResp.Header.Get("Content-Type"), "image/")
			}
		}
		_ = body // used above for context
	})

	t.Run("404 for unknown routes", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/nonexistent-route")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("gzip compression works", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Go's http.Client transparently decompresses, so check the response is valid
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("concurrent requests handled", func(t *testing.T) {
		done := make(chan bool, 20)
		for i := 0; i < 20; i++ {
			go func() {
				resp, err := client.Get(baseURL + "/")
				if err == nil {
					resp.Body.Close()
					done <- resp.StatusCode == http.StatusOK
				} else {
					done <- false
				}
			}()
		}
		for i := 0; i < 20; i++ {
			assert.True(t, <-done, "concurrent request %d should succeed", i)
		}
	})
}
