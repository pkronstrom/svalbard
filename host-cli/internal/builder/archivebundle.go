package builder

import (
	"archive/zip"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// archiveBundleAfter is how many attached files a project may have before
// they are packed into one zip: a long list of downloads is hard to use, and
// design files compress well.
const archiveBundleAfter = 3

// archiveStoredFile is one attachment saved beside a project page.
type archiveStoredFile struct {
	Name  string
	Local string
	Size  int
	// Repo names the repository a packed repository came from, and Note
	// says what of it was kept.
	Repo, Note string
}

// bundleArchiveFiles packs the files into one zip under their original names
// and removes the loose copies.
func bundleArchiveFiles(output, entry string, files []archiveStoredFile) (archiveStoredFile, error) {
	local := "_files/" + archiveBundleName(entry) + "-files.zip"
	destination := filepath.Join(output, filepath.FromSlash(local))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return archiveStoredFile{}, err
	}
	archive, err := os.Create(destination)
	if err != nil {
		return archiveStoredFile{}, err
	}
	writer := zip.NewWriter(archive)
	used := make(map[string]bool)
	for _, file := range files {
		name := archiveUniqueName(archiveFileName(file.Name), used)
		if err := archiveZipFile(writer, name, filepath.Join(output, filepath.FromSlash(file.Local))); err != nil {
			archive.Close()
			os.Remove(destination)
			return archiveStoredFile{}, err
		}
	}
	if err := writer.Close(); err != nil {
		archive.Close()
		return archiveStoredFile{}, err
	}
	if err := archive.Close(); err != nil {
		return archiveStoredFile{}, err
	}
	for _, file := range files {
		os.Remove(filepath.Join(output, filepath.FromSlash(file.Local)))
	}
	info, err := os.Stat(destination)
	if err != nil {
		return archiveStoredFile{}, err
	}
	return archiveStoredFile{Name: path.Base(local), Local: local, Size: int(info.Size())}, nil
}

func archiveZipFile(writer *zip.Writer, name, source string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, input)
	return err
}

// archiveBundleName names a project's zip after its page.
func archiveBundleName(entry string) string {
	name := strings.TrimSuffix(path.Base(entry), path.Ext(entry))
	if name == "index" && path.Dir(entry) != "." {
		name = path.Base(path.Dir(entry))
	}
	if name = archiveFileName(name); name == "" {
		name = "project"
	}
	return name
}

func archiveUniqueName(name string, used map[string]bool) string {
	if name == "" {
		name = "file"
	}
	candidate, extension := name, path.Ext(name)
	for number := 2; used[candidate]; number++ {
		candidate = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, extension), number, extension)
	}
	used[candidate] = true
	return candidate
}

// archiveFilesBox lists a project's attachments: one download for a bundle,
// with its contents named, or a link per file, then the packed
// repositories.
func archiveFilesBox(entry string, files []archiveStoredFile, bundle *archiveStoredFile, repos []archiveStoredFile) string {
	if len(files) == 0 && len(repos) == 0 {
		return ""
	}
	var box strings.Builder
	box.WriteString(`<div class="files"><h2>Files</h2>`)
	switch {
	case len(files) == 0:
	case bundle != nil:
		fmt.Fprintf(&box, `<p><a class="download" href="%s">Download all %d files</a> <span class="size">zip, %s</span></p><ul>`,
			template.HTMLEscapeString(archiveRelative(entry, bundle.Local)), len(files), archiveSize(bundle.Size))
		for _, file := range files {
			fmt.Fprintf(&box, `<li>%s <span class="size">%s</span></li>`, template.HTMLEscapeString(file.Name), archiveSize(file.Size))
		}
	default:
		box.WriteString("<ul>")
		for _, file := range files {
			fmt.Fprintf(&box, `<li><a href="%s">%s</a> <span class="size">%s</span></li>`,
				template.HTMLEscapeString(archiveRelative(entry, file.Local)), template.HTMLEscapeString(file.Name), archiveSize(file.Size))
		}
	}
	if len(files) > 0 {
		box.WriteString("</ul>")
	}
	if len(repos) > 0 {
		box.WriteString("<h3>Repositories</h3><ul>")
		for _, repo := range repos {
			fmt.Fprintf(&box, `<li><a href="%s">%s</a> <span class="size">zip, %s; %s</span></li>`,
				template.HTMLEscapeString(archiveRelative(entry, repo.Local)), template.HTMLEscapeString(repo.Repo), archiveSize(repo.Size), template.HTMLEscapeString(repo.Note))
		}
		box.WriteString("</ul>")
	}
	box.WriteString("</div>")
	return box.String()
}

func archiveSize(bytes int) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%d kB", bytes>>10)
	}
	return fmt.Sprintf("%d B", bytes)
}
