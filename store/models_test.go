package store

import (
	"context"
	"fmt"
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
	assert.NotEmpty(t, canyons.LCC.ETag)
	assert.NotEmpty(t, canyons.BCC.ETag)
}

// mockNeonRepo is a mock implementation of NeonRepository for testing
type mockNeonRepo struct {
	canyons []NeonCanyon
	err     error
}

func (m *mockNeonRepo) ListCanyons(ctx context.Context) ([]NeonCanyon, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.canyons, nil
}

func TestCanyons_LoadFromNeon(t *testing.T) {
	tests := []struct {
		name       string
		mockData   []NeonCanyon
		mockErr    error
		wantErr    bool
		errContain string
		validate   func(*testing.T, *Canyons)
	}{
		{
			name: "successful load with status cameras",
			mockData: []NeonCanyon{
				{
					ID:   "LCC",
					Name: "Little Cottonwood Canyon",
					Status: &NeonCanyonStatus{
						Src:  "https://example.com/lcc-status.jpg",
						Alt:  "LCC Status",
						Kind: "status",
					},
					Cameras: []NeonCamera{
						{
							ID:       "lcc-cam1",
							CanyonID: "LCC",
							Src:      "https://example.com/lcc-cam1.jpg",
							Alt:      "LCC Camera 1",
							Kind:     "image",
						},
					},
				},
				{
					ID:   "BCC",
					Name: "Big Cottonwood Canyon",
					Status: &NeonCanyonStatus{
						Src:  "https://example.com/bcc-status.jpg",
						Alt:  "BCC Status",
						Kind: "status",
					},
					Cameras: []NeonCamera{
						{
							ID:       "bcc-cam1",
							CanyonID: "BCC",
							Src:      "https://example.com/bcc-cam1.jpg",
							Alt:      "BCC Camera 1",
							Kind:     "image",
						},
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, c *Canyons) {
				// Verify LCC
				assert.Equal(t, "Little Cottonwood Canyon", c.LCC.Name)
				assert.Equal(t, "https://example.com/lcc-status.jpg", c.LCC.Status.Src)
				assert.Len(t, c.LCC.Cameras, 1)
				assert.Equal(t, "lcc-cam1", c.LCC.Cameras[0].ID)
				assert.NotEmpty(t, c.LCC.ETag)

				// Verify BCC
				assert.Equal(t, "Big Cottonwood Canyon", c.BCC.Name)
				assert.Equal(t, "https://example.com/bcc-status.jpg", c.BCC.Status.Src)
				assert.Len(t, c.BCC.Cameras, 1)
				assert.Equal(t, "bcc-cam1", c.BCC.Cameras[0].ID)
				assert.NotEmpty(t, c.BCC.ETag)
			},
		},
		{
			name: "load without status cameras",
			mockData: []NeonCanyon{
				{
					ID:   "LCC",
					Name: "Little Cottonwood Canyon",
					Cameras: []NeonCamera{
						{
							ID:       "lcc-cam1",
							CanyonID: "LCC",
							Src:      "https://example.com/lcc-cam1.jpg",
							Alt:      "LCC Camera 1",
							Kind:     "image",
						},
					},
				},
				{
					ID:      "BCC",
					Name:    "Big Cottonwood Canyon",
					Cameras: []NeonCamera{},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, c *Canyons) {
				assert.Empty(t, c.LCC.Status.Src)
				assert.Len(t, c.LCC.Cameras, 1)
				assert.Empty(t, c.BCC.Status.Src)
				assert.Len(t, c.BCC.Cameras, 0)
			},
		},
		{
			name:       "repository error",
			mockErr:    fmt.Errorf("database connection failed"),
			wantErr:    true,
			errContain: "failed to list canyons from Neon",
		},
		{
			name: "unknown canyon ID",
			mockData: []NeonCanyon{
				{
					ID:      "UNKNOWN",
					Name:    "Unknown Canyon",
					Cameras: []NeonCamera{},
				},
			},
			wantErr:    true,
			errContain: "unknown canyon ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockNeonRepo{
				canyons: tt.mockData,
				err:     tt.mockErr,
			}

			canyons, err := NewStoreFromNeon(context.Background(), mock)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, canyons)

			if tt.validate != nil {
				tt.validate(t, canyons)
			}
		})
	}
}
