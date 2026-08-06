// Package minify provides CSS/JS minification helpers (used by the CLI and tests).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
)

// mediaTypeFor returns the minify media type for a language or file path.
func mediaTypeFor(kind string) (string, error) {
	switch strings.ToLower(kind) {
	case "js", "mjs", "javascript", "application/javascript", "text/javascript":
		return "application/javascript", nil
	case "css", "text/css":
		return "text/css", nil
	default:
		ext := strings.ToLower(filepath.Ext(kind))
		switch ext {
		case ".js", ".mjs":
			return "application/javascript", nil
		case ".css":
			return "text/css", nil
		}
		return "", fmt.Errorf("unsupported minify kind %q (use js or css)", kind)
	}
}

func newMinifier() *minify.M {
	m := minify.New()
	m.AddFunc("application/javascript", js.Minify)
	m.AddFunc("text/javascript", js.Minify)
	m.AddFunc("text/css", css.Minify)
	return m
}

// minifyReader minifies in → out for the given kind (js|css or a filename).
func minifyReader(kind string, in io.Reader, out io.Writer) error {
	mediaType, err := mediaTypeFor(kind)
	if err != nil {
		return err
	}
	return newMinifier().Minify(mediaType, out, in)
}

// minifyFile minifies src into dst.
func minifyFile(kind, src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", dst, err)
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer func() { _ = out.Close() }()

	if err := minifyReader(kind, in, out); err != nil {
		return fmt.Errorf("minify %s: %w", src, err)
	}
	return out.Close()
}
