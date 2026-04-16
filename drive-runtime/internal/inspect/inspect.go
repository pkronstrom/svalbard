package inspect

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func Run(w io.Writer, driveRoot string) error {
	manifestMeta, _ := ReadManifestMetadata(filepath.Join(driveRoot, "manifest.yaml"))

	fmt.Fprintln(w, "Drive contents")
	fmt.Fprintln(w, "==============")
	if manifestMeta["preset"] != "" {
		fmt.Fprintf(w, "  Preset:  %s\n", manifestMeta["preset"])
	}
	if manifestMeta["region"] != "" {
		fmt.Fprintf(w, "  Region:  %s\n", manifestMeta["region"])
	}
	if manifestMeta["created"] != "" {
		fmt.Fprintf(w, "  Created: %s\n", manifestMeta["created"])
	}
	fmt.Fprintln(w)

	for _, dir := range []string{"zim", "maps", "models", "data", "apps", "books", "bin"} {
		full := filepath.Join(driveRoot, dir)
		count, size, err := SummarizeDirectory(full)
		if err != nil || count == 0 {
			continue
		}
		fmt.Fprintf(w, "  %-10s %4d files  %8s\n", dir+"/", count, HumanSize(size))
	}
	fmt.Fprintln(w)

	if err := printFilesSection(w, driveRoot, "ZIM files", "zim", ".zim"); err != nil {
		return err
	}
	if err := printFilesSection(w, driveRoot, "Models", "models", ".gguf"); err != nil {
		return err
	}
	if err := printFilesSection(w, driveRoot, "Databases", "data", ".sqlite"); err != nil {
		return err
	}
	if err := printFilesSection(w, driveRoot, "Map tiles", "maps", ".pmtiles"); err != nil {
		return err
	}
	return nil
}

func ReadManifestMetadata(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return map[string]string{}, err
	}
	defer file.Close()

	meta := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		for _, key := range []string{"preset", "region", "created"} {
			prefix := key + ":"
			if strings.HasPrefix(line, prefix) {
				meta[key] = strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return meta, scanner.Err()
}

func SummarizeDirectory(root string) (int, int64, error) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return 0, 0, err
	}

	var count int
	var size int64
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), "._") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		count++
		size += info.Size()
		return nil
	})
	return count, size, err
}

func printFilesSection(w io.Writer, driveRoot, title, dirName, ext string) error {
	files, err := ListFilesWithExtension(filepath.Join(driveRoot, dirName), ext)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(files) == 0 {
		return nil
	}

	fmt.Fprintln(w, title)
	fmt.Fprintln(w, strings.Repeat("=", len(title)))
	for _, file := range files {
		rel, err := filepath.Rel(driveRoot, file)
		if err != nil {
			rel = file
		}
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "  %-8s %s\n", HumanSize(info.Size()), filepath.ToSlash(rel))
	}
	fmt.Fprintln(w)
	return nil
}

func ListFilesWithExtension(root, ext string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		if strings.HasPrefix(name, "._") || !strings.HasSuffix(strings.ToLower(name), ext) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func HumanSize(size int64) string {
	const (
		kb = 1000
		mb = 1000 * kb
		gb = 1000 * mb
	)
	switch {
	case size >= gb:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.0f MB", float64(size)/float64(mb))
	default:
		return fmt.Sprintf("%.0f KB", float64(size)/float64(kb))
	}
}
