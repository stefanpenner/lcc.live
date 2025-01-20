package main

import (
	"fmt"
	"io/fs"
	"log"
)

// print dir structures
func printFS(name string, f fs.FS) {
	entries, err := f.(fs.ReadDirFS).ReadDir(".")
	if err != nil {
		log.Printf("Error reading embedded FS: %v\n", err)
		return
	}

	fmt.Printf("%s: \n", name)
	for _, entry := range entries {
		if entry.IsDir() {
			printDir(f, entry.Name(), "  ")
		} else {
			fmt.Printf("  %s\n", entry.Name())
		}
	}
	fmt.Println()
}

func printDir(f fs.FS, dir string, indent string) {
	entries, err := f.(fs.ReadDirFS).ReadDir(dir)
	if err != nil {
		log.Printf("Error reading dir %s: %v\n", dir, err)
		return
	}

	fmt.Printf("%s%s/\n", indent, dir)
	for _, entry := range entries {
		if entry.IsDir() {
			printDir(f, dir+"/"+entry.Name(), indent+"  ")
		} else {
			fmt.Printf("%s%s\n", indent+"  ", entry.Name())
		}
	}
}
