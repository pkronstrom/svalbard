package zimext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/stazelabs/gozim/zim"
)

// Video describes an archived media entry and the timed text stored beside it.
type Video struct {
	Path     string
	Title    string
	Captions []string
	Chapters string
}

// Videos uses the MIME table as a cheap filter before scanning the directory.
func (a *Archive) Videos(ctx context.Context) ([]Video, error) {
	for _, mime := range a.zim.MIMETypes() {
		if strings.HasPrefix(mime, "video/") {
			videos, err := a.listVideos(ctx, a.zim.AllEntriesByNamespace(a.ns))
			if isLongDirectoryEntry(err) {
				return a.listVideos(ctx, readableContentEntries(ctx, a.zim))
			}
			return videos, err
		}
	}
	return nil, nil
}

func (a *Archive) listVideos(ctx context.Context, entries func(func(zim.Entry, error) bool)) ([]Video, error) {
	var videos []Video
	tracks := make(map[string][]string)
	titles := make(map[string]string)
	mediaPerDir := make(map[string]int)
	for entry, err := range entries {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			return nil, err
		}
		if entry.IsRedirect() {
			continue
		}
		name, mime := entry.Path(), entry.MIMEType()
		switch {
		case strings.HasPrefix(mime, "video/"):
			videos = append(videos, Video{Path: name, Title: entry.Title()})
			mediaPerDir[path.Dir(name)]++
		case isTimedText(name):
			dir := path.Dir(name)
			tracks[dir] = append(tracks[dir], name)
		case mime == "application/json" && strings.HasPrefix(name, "videos/") && path.Dir(name) == "videos":
			// youtube2zim names the viewer metadata by its human-readable slug.
			// The metadata maps it to the downloaded media asset.
			var metadata struct {
				Path  string `json:"videoPath"`
				Title string `json:"title"`
			}
			content, err := a.ReadEntry(name, 1<<20)
			if err == nil && json.Unmarshal(content, &metadata) == nil && metadata.Title != "" {
				titles[strings.TrimLeft(metadata.Path, "/")] = metadata.Title
			}
		}
	}
	for i := range videos {
		video := &videos[i]
		if title := titles[video.Path]; title != "" {
			video.Title = title
		}
		dir := path.Dir(video.Path)
		stem := strings.TrimSuffix(path.Base(video.Path), path.Ext(video.Path))
		perVideoDir := dir != "." && mediaPerDir[dir] == 1
		for _, track := range tracks[dir] {
			name := strings.ToLower(path.Base(track))
			switch {
			case name == "chapters.vtt" && perVideoDir:
				video.Chapters = track
			case perVideoDir, strings.HasPrefix(name, strings.ToLower(stem)+"."):
				video.Captions = append(video.Captions, track)
			}
		}
	}
	return videos, nil
}

func isTimedText(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".vtt", ".srt":
		return true
	}
	return false
}

// CopyVideo writes one media blob to a host file for the offline recognizer.
// It uses the archive's existing blob buffer without making a second copy.
func (a *Archive) CopyVideo(ctx context.Context, name string, out io.Writer) error {
	entry, err := a.zim.EntryByPath(string(a.ns) + "/" + name)
	if err != nil {
		return fmt.Errorf("find video: %w", err)
	}
	item, err := entry.Item()
	if err != nil {
		return err
	}
	reader, err := item.Reader()
	if err != nil {
		return err
	}
	_, err = io.Copy(out, videoReader{ctx, reader})
	return err
}

type videoReader struct {
	ctx context.Context
	io.Reader
}

func (r videoReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}
