package fs

import (
	"fmt"
	"io/fs"
	"log"

	style "github.com/stefanpenner/lcc-live/style"
)

func Print(name string, f fs.FS) {
	entries, err := f.(fs.ReadDirFS).ReadDir(".")
	if err != nil {
		log.Printf("Error reading embedded FS: %v\n", err)
		return
	}

	fmt.Println(style.Section.Render(name + ":"))
	for _, entry := range entries {
		prefix := "  └─"
		if entry.IsDir() {
			fmt.Printf("%s %s\n", prefix, style.Dir.Render("📁 "+entry.Name()+"/"))
			PrintDir(f, entry.Name(), "     ")
		} else {
			fmt.Printf("%s %s\n", prefix, style.File.Render("📄 "+entry.Name()))
		}
	}
}

func PrintDir(f fs.FS, dir string, indent string) {
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
			fmt.Printf("%s %s\n", prefix, style.Dir.Render("📁 "+entry.Name()+"/"))
			newIndent := indent
			if isLast {
				newIndent += "   "
			} else {
				newIndent += "│  "
			}
			PrintDir(f, dir+"/"+entry.Name(), newIndent)
		} else {
			fmt.Printf("%s %s\n", prefix, style.File.Render("📄 "+entry.Name()))
		}
	}
}
