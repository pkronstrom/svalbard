package server

import (
	"context"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// StartKiwixForSearch uses a private catalog when Kiwix would otherwise reject
// even a single multilingual book on /search. Changing the catalog's language
// does not change the archive or the language settings in its Xapian index.
func StartKiwixForSearch(ctx context.Context, binary string, zims []string, port int, languages map[string]string) (*exec.Cmd, error) {
	for _, zim := range zims {
		if strings.Contains(languages[kiwixBookName(zim)], ",") {
			return startSearchLibrary(ctx, binary, zims, port, languages)
		}
	}
	return StartKiwix(ctx, binary, zims, port)
}

type searchBook struct {
	ID       string `xml:"id,attr"`
	Path     string `xml:"path,attr"`
	Title    string `xml:"title,attr"`
	Language string `xml:"language,attr"`
}

func startSearchLibrary(ctx context.Context, binary string, zims []string, port int, languages map[string]string) (*exec.Cmd, error) {
	library := struct {
		XMLName xml.Name     `xml:"library"`
		Version string       `xml:"version,attr"`
		Books   []searchBook `xml:"book"`
	}{Version: "20110515"}
	for _, zim := range zims {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		book, err := searchLibraryBook(zim, languages[kiwixBookName(zim)])
		if err != nil {
			return nil, err
		}
		library.Books = append(library.Books, book)
	}
	file, err := os.CreateTemp("", "svalbard-kiwix-search-*.xml")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	encodeErr := xml.NewEncoder(file).Encode(library)
	closeErr := file.Close()
	if encodeErr != nil {
		return nil, encodeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	// Kiwix loads the catalog at startup. The health check in startKiwix
	// lets us remove the temporary file before returning.
	return startKiwix(ctx, binary, []string{"--library", file.Name()}, port)
}

func searchLibraryBook(zim, languages string) (searchBook, error) {
	fullPath, err := filepath.Abs(zim)
	if err != nil {
		return searchBook{}, err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return searchBook{}, err
	}
	defer file.Close()
	// The sixteen UUID bytes follow the magic and version in the ZIM header.
	header := make([]byte, 24)
	if _, err := io.ReadFull(file, header); err != nil {
		return searchBook{}, fmt.Errorf("reading ZIM header for %s: %w", zim, err)
	}
	if binary.LittleEndian.Uint32(header[:4]) != 0x044d495a {
		return searchBook{}, fmt.Errorf("invalid ZIM header for %s", zim)
	}
	u := header[8:]
	id := fmt.Sprintf("%x-%x-%x-%x-%x", u[:4], u[4:6], u[6:8], u[8:10], u[10:])
	primary, _, _ := strings.Cut(languages, ",")
	return searchBook{ID: id, Path: fullPath, Title: kiwixBookName(zim), Language: strings.TrimSpace(primary)}, nil
}

func kiwixBookName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
