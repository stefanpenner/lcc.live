// Command minify minifies JS or CSS files for the Bazel static asset pipeline.
//
// Usage:
//
//	minify <js|css> <input> <output>
//
// Hermetic go_binary tool; invoked from //web/static genrules.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "usage: %s <js|css> <input> <output>\n", os.Args[0])
		os.Exit(2)
	}
	kind, src, dst := os.Args[1], os.Args[2], os.Args[3]
	if err := minifyFile(kind, src, dst); err != nil {
		fmt.Fprintf(os.Stderr, "minify: %v\n", err)
		os.Exit(1)
	}
}
