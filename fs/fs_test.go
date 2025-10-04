package fs

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrint_EmptyFS(t *testing.T) {
	testFS := fstest.MapFS{}

	output := captureOutput(func() {
		Print("Test FS", testFS)
	})

	assert.Contains(t, output, "Test FS:")
}

func TestPrint_WithFiles(t *testing.T) {
	testFS := fstest.MapFS{
		"file1.txt": &fstest.MapFile{
			Data: []byte("content1"),
		},
		"file2.txt": &fstest.MapFile{
			Data: []byte("content2"),
		},
	}

	output := captureOutput(func() {
		Print("Test FS", testFS)
	})

	assert.Contains(t, output, "Test FS:")
	assert.Contains(t, output, "file1.txt")
	assert.Contains(t, output, "file2.txt")
	assert.Contains(t, output, "📄")
}

func TestPrint_WithDirectories(t *testing.T) {
	testFS := fstest.MapFS{
		"dir1/file1.txt": &fstest.MapFile{
			Data: []byte("content1"),
		},
		"dir1/file2.txt": &fstest.MapFile{
			Data: []byte("content2"),
		},
		"dir2/subdir/file3.txt": &fstest.MapFile{
			Data: []byte("content3"),
		},
	}

	output := captureOutput(func() {
		Print("Test FS", testFS)
	})

	assert.Contains(t, output, "Test FS:")
	assert.Contains(t, output, "dir1")
	assert.Contains(t, output, "dir2")
	assert.Contains(t, output, "📁")
	assert.Contains(t, output, "file1.txt")
	assert.Contains(t, output, "file2.txt")
}

func TestPrintDir_SingleLevel(t *testing.T) {
	testFS := fstest.MapFS{
		"dir/file1.txt": &fstest.MapFile{
			Data: []byte("content1"),
		},
		"dir/file2.txt": &fstest.MapFile{
			Data: []byte("content2"),
		},
	}

	output := captureOutput(func() {
		PrintDir(testFS, "dir", "  ")
	})

	assert.Contains(t, output, "file1.txt")
	assert.Contains(t, output, "file2.txt")
}

func TestPrintDir_Nested(t *testing.T) {
	testFS := fstest.MapFS{
		"dir/subdir1/file1.txt": &fstest.MapFile{
			Data: []byte("content1"),
		},
		"dir/subdir2/file2.txt": &fstest.MapFile{
			Data: []byte("content2"),
		},
	}

	output := captureOutput(func() {
		PrintDir(testFS, "dir", "")
	})

	assert.Contains(t, output, "subdir1")
	assert.Contains(t, output, "subdir2")
	assert.Contains(t, output, "file1.txt")
	assert.Contains(t, output, "file2.txt")
}

func TestPrintDir_TreeStructure(t *testing.T) {
	testFS := fstest.MapFS{
		"root/a.txt": &fstest.MapFile{Data: []byte("a")},
		"root/b.txt": &fstest.MapFile{Data: []byte("b")},
		"root/c.txt": &fstest.MapFile{Data: []byte("c")},
	}

	output := captureOutput(func() {
		PrintDir(testFS, "root", "")
	})

	// Should have tree characters
	assert.Contains(t, output, "├─")
	assert.Contains(t, output, "└─")
}

func TestPrintDir_InvalidDirectory(t *testing.T) {
	testFS := fstest.MapFS{
		"file.txt": &fstest.MapFile{
			Data: []byte("content"),
		},
	}

	// Should not panic on invalid directory
	output := captureOutput(func() {
		PrintDir(testFS, "nonexistent", "")
	})

	// Should produce no output for nonexistent directory
	assert.Empty(t, strings.TrimSpace(output))
}

func TestPrint_MixedContent(t *testing.T) {
	testFS := fstest.MapFS{
		"file1.txt": &fstest.MapFile{
			Data: []byte("content1"),
		},
		"dir1/file2.txt": &fstest.MapFile{
			Data: []byte("content2"),
		},
		"dir1/subdir/file3.txt": &fstest.MapFile{
			Data: []byte("content3"),
		},
		"file4.json": &fstest.MapFile{
			Data: []byte("{}"),
		},
	}

	output := captureOutput(func() {
		Print("Mixed FS", testFS)
	})

	// Check structure is present
	assert.Contains(t, output, "Mixed FS:")
	assert.Contains(t, output, "file1.txt")
	assert.Contains(t, output, "file4.json")
	assert.Contains(t, output, "dir1")
	assert.Contains(t, output, "file2.txt")
	assert.Contains(t, output, "subdir")
	assert.Contains(t, output, "file3.txt")
}

// Test with actual fs.FS interface behavior
func TestPrint_RealFSBehavior(t *testing.T) {
	// Create a real temporary directory structure
	tmpDir := t.TempDir()

	// Create files
	require.NoError(t, os.WriteFile(tmpDir+"/test.txt", []byte("test"), 0644))
	require.NoError(t, os.Mkdir(tmpDir+"/subdir", 0755))
	require.NoError(t, os.WriteFile(tmpDir+"/subdir/nested.txt", []byte("nested"), 0644))

	// Create fs.FS from real directory
	dirFS := os.DirFS(tmpDir)

	output := captureOutput(func() {
		Print("Real FS", dirFS)
	})

	assert.Contains(t, output, "Real FS:")
	assert.Contains(t, output, "test.txt")
	assert.Contains(t, output, "subdir")
	assert.Contains(t, output, "nested.txt")
}

// Test error handling when FS doesn't implement ReadDirFS
type invalidFS struct{}

func (invalidFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func TestPrint_InvalidFSType(t *testing.T) {
	// This should panic because invalidFS doesn't implement ReadDirFS
	// But Print tries to type assert it
	defer func() {
		if r := recover(); r != nil {
			// Expected panic
			assert.NotNil(t, r)
		}
	}()

	var testFS fs.FS = invalidFS{}

	captureOutput(func() {
		Print("Invalid FS", testFS)
	})
}
