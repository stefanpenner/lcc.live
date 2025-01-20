package main

import (
	"fmt"
	"io/fs"
	"log"
)

func printFS(name string, f fs.FS) {
	entries, err := f.(fs.ReadDirFS).ReadDir(".")
	if err != nil {
		log.Printf("Error reading embedded FS: %v\n", err)
		return
	}

	fmt.Println(sectionStyle.Render(name + ":"))
	for _, entry := range entries {
		prefix := "  └─"
		if entry.IsDir() {
			fmt.Printf("%s %s\n", prefix, dirStyle.Render("📁 "+entry.Name()+"/"))
			printDir(f, entry.Name(), "     ")
		} else {
			fmt.Printf("%s %s\n", prefix, fileStyle.Render("📄 "+entry.Name()))
		}
	}
}

func printDir(f fs.FS, dir string, indent string) {
	entries, err := f.(fs.ReadDirFS).ReadDir(dir)
	if err != nil {
		return
	}

	for i, entry := range entries {
		isLast := i == len(entries)-1
		prefix := indent + "└─"
		if !isLast {
			prefix = indent + "├─"
		}

		if entry.IsDir() {
			fmt.Printf("%s %s\n", prefix, dirStyle.Render("📁 "+entry.Name()+"/"))
			newIndent := indent
			if isLast {
				newIndent += "   "
			} else {
				newIndent += "│  "
			}
			printDir(f, dir+"/"+entry.Name(), newIndent)
		} else {
			fmt.Printf("%s %s\n", prefix, fileStyle.Render("📄 "+entry.Name()))
		}
	}
}
