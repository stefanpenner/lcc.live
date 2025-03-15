package store

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanyons_Load(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid canyons data",
			jsonData: `{
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
					"cameras": [
						{
							"id": "bcc-cam1",
							"kind": "camera",
							"src": "https://example.com/bcc-cam1.jpg",
							"alt": "BCC Camera 1",
							"canyon": "bcc"
						}
					]
				}
			}`,
			wantErr: false,
		},
		{
			name:     "empty file",
			jsonData: "",
			wantErr:  true,
			errMsg:   "file test.json is empty",
		},
		{
			name:     "invalid JSON",
			jsonData: "{invalid json",
			wantErr:  true,
			errMsg:   "invalid JSON in file test.json",
		},
		{
			name: "missing status cameras",
			jsonData: `{
				"lcc": {
					"name": "Little Cottonwood Canyon",
					"status": {
						"id": "lcc-status",
						"kind": "status",
						"alt": "LCC Status",
						"canyon": "lcc"
					},
					"cameras": []
				},
				"bcc": {
					"name": "Big Cottonwood Canyon",
					"status": {
						"id": "bcc-status",
						"kind": "status",
						"alt": "BCC Status",
						"canyon": "bcc"
					},
					"cameras": []
				}
			}`,
			wantErr: true,
			errMsg:  "JSON from test.json did not contain expected canyon data",
		},
		{
			name: "one empty status camera is ok",
			jsonData: `{
				"lcc": {
					"name": "Little Cottonwood Canyon",
					"status": {
						"id": "lcc-status",
						"kind": "status",
						"src": "https://example.com/lcc-status.jpg",
						"alt": "LCC Status",
						"canyon": "lcc"
					},
					"cameras": []
				},
				"bcc": {
					"name": "Big Cottonwood Canyon",
					"status": {
						"id": "bcc-status",
						"kind": "status",
						"alt": "BCC Status",
						"canyon": "bcc"
					},
					"cameras": []
				}
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a virtual filesystem with our test data
			fs := fstest.MapFS{
				"test.json": &fstest.MapFile{
					Data: []byte(tt.jsonData),
				},
			}

			var canyons Canyons
			err := canyons.Load(fs, "test.json")

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				// Verify ETags were computed
				assert.NotEmpty(t, canyons.LCC.ETag)
				assert.NotEmpty(t, canyons.BCC.ETag)
			}
		})
	}
}

func TestCanyons_SetETag(t *testing.T) {
	canyon := Canyon{
		Name: "Test Canyon",
		Status: Camera{
			ID:     "test-status",
			Kind:   "status",
			Src:    "https://example.com/test.jpg",
			Alt:    "Test Status",
			Canyon: "test",
		},
		Cameras: []Camera{
			{
				ID:     "test-cam1",
				Kind:   "camera",
				Src:    "https://example.com/test-cam1.jpg",
				Alt:    "Test Camera 1",
				Canyon: "test",
			},
		},
	}

	var canyons Canyons
	err := canyons.setETag(&canyon)
	require.NoError(t, err)

	// Verify ETag format
	assert.NotEmpty(t, canyon.ETag)
}

func TestCanyons_String(t *testing.T) {
	canyons := Canyons{
		LCC: Canyon{
			Name: "Little Cottonwood Canyon",
			Status: Camera{
				ID:     "lcc-status",
				Kind:   "status",
				Src:    "https://example.com/lcc-status.jpg",
				Alt:    "LCC Status",
				Canyon: "lcc",
			},
		},
	}

	// Set ETag
	err := canyons.setETag(&canyons.LCC)
	require.NoError(t, err)

	// Test String method
	str := canyons.String()
	assert.Contains(t, str, "Little Cottonwood Canyon")
	assert.Contains(t, str, "lcc-status")
}

// Integration test with real file system
func TestCanyons_LoadFromRealFile(t *testing.T) {
	// Create a temporary file with valid JSON
	tempDir, err := os.MkdirTemp("", "canyons-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	jsonData := `{
		"lcc": {
			"name": "Little Cottonwood Canyon",
			"status": {
				"id": "lcc-status",
				"kind": "status",
				"src": "https://example.com/lcc-status.jpg",
				"alt": "LCC Status",
				"canyon": "lcc"
			},
			"cameras": []
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
	}`

	filePath := filepath.Join(tempDir, "canyons.json")
	err = os.WriteFile(filePath, []byte(jsonData), 0644)
	require.NoError(t, err)

	var canyons Canyons
	err = canyons.Load(os.DirFS(tempDir), "canyons.json")
	require.NoError(t, err)

	// Verify data was loaded correctly
	assert.Equal(t, "Little Cottonwood Canyon", canyons.LCC.Name)
	assert.Equal(t, "Big Cottonwood Canyon", canyons.BCC.Name)
	assert.Equal(t, "https://example.com/lcc-status.jpg", canyons.LCC.Status.Src)
	assert.Equal(t, "https://example.com/bcc-status.jpg", canyons.BCC.Status.Src)
	assert.Equal(t, "\"4173245339693007937\"", canyons.LCC.ETag)
	assert.Equal(t, "\"16678285087420380908\"", canyons.BCC.ETag)
}
