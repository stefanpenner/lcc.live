package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMinifyJS(t *testing.T) {
	in := "const  foo  =  1;\n// comment\nfunction bar ( x ) { return x + 1; }\n"
	var out bytes.Buffer
	require.NoError(t, minifyReader("js", strings.NewReader(in), &out))
	got := out.String()
	assert.NotContains(t, got, "// comment")
	assert.NotContains(t, got, "  ")
	assert.Less(t, len(got), len(in))
	assert.Contains(t, got, "foo")
	assert.Contains(t, got, "bar")
}

func TestMinifyCSS(t *testing.T) {
	in := "/* header */\n.foo  {\n  color :  red ;\n  margin : 0  ;\n}\n"
	var out bytes.Buffer
	require.NoError(t, minifyReader("css", strings.NewReader(in), &out))
	got := out.String()
	assert.NotContains(t, got, "/*")
	assert.NotContains(t, got, "  ")
	assert.Less(t, len(got), len(in))
	assert.Contains(t, got, ".foo")
	assert.Contains(t, got, "red")
}

func TestMinifyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.css")
	dst := filepath.Join(dir, "out", "out.css")
	require.NoError(t, os.WriteFile(src, []byte("body { color: blue; }\n"), 0o644))
	require.NoError(t, minifyFile("css", src, dst))
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Less(t, len(data), len("body { color: blue; }\n"))
}

func TestMediaTypeFor(t *testing.T) {
	js, err := mediaTypeFor("js")
	require.NoError(t, err)
	assert.Equal(t, "application/javascript", js)

	css, err := mediaTypeFor("style.css")
	require.NoError(t, err)
	assert.Equal(t, "text/css", css)

	_, err = mediaTypeFor("png")
	assert.Error(t, err)
}
