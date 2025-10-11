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

	fileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			fileCount++
		} else {
			// Count files in subdirectories
			fileCount += countFiles(f, entry.Name())
		}
	}

	icon := "📄"
	if name == "Public" {
		icon = "🌐"
	} else if name == "Templates" {
		icon = "📑"
	}

	fmt.Printf("%s %s: %d %s\n",
		style.File.Render(icon),
		style.Section.Render(name),
		fileCount,
		pluralize(fileCount, "file", "files"))
}

func countFiles(f fs.FS, dir string) int {
	entries, err := f.(fs.ReadDirFS).ReadDir(dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count += countFiles(f, dir+"/"+entry.Name())
		} else {
			count++
		}
	}
	return count
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
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
