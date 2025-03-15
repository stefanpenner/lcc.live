package store

import (
	"context"
	"net/http"
	"net/http/httptest"
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
