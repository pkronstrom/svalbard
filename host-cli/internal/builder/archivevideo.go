package builder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// archiveVideoMarker tags the paragraph that stands in for an embedded video
// until the video is stored, or for good when it is not.
const archiveVideoMarker = "data-archive-video"

var (
	youtubeIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	archiveEmbedSelector = cascadia.MustCompile(`iframe, embed, oembed`)
)

type archiveVideo struct {
	ID          string
	Title       string
	Uploader    string
	Description string
	File        string
	Poster      string
	Captions    string
}

// youtubeVideoID returns the ID of a YouTube watch, short, embed or youtu.be
// address, or "" for anything else.
func youtubeVideoID(target *url.URL) string {
	if target == nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(target.Host), "www.")
	host = strings.TrimPrefix(host, "m.")
	id := ""
	switch host {
	case "youtu.be":
		id = strings.Trim(target.Path, "/")
	case "youtube.com", "youtube-nocookie.com":
		if target.Path == "/watch" {
			id = target.Query().Get("v")
		}
		for _, prefix := range []string{"/embed/", "/shorts/", "/v/", "/live/"} {
			if rest, found := strings.CutPrefix(target.Path, prefix); found {
				id, _, _ = strings.Cut(rest, "/")
			}
		}
	}
	if youtubeIDPattern.MatchString(id) {
		return id
	}
	return ""
}

// youtubeArticle archives a YouTube link as the video itself.
func youtubeArticle(fetcher *archiveFetcher, source *url.URL) (*archiveArticle, error) {
	id := youtubeVideoID(source)
	if id == "" {
		return archiveHTMLArticle(fetcher, source, nil)
	}
	return &archiveArticle{Base: source, Content: archiveContainer(), Videos: []string{id}}, nil
}

// archiveMarkVideoEmbeds swaps embedded YouTube players for a marked
// paragraph naming the video, and queues the videos when embeds are archived.
func archiveMarkVideoEmbeds(fetcher *archiveFetcher, article *archiveArticle, root *html.Node) {
	for _, node := range cascadia.QueryAll(root, archiveEmbedSelector) {
		reference := ""
		for _, key := range []string{"src", "data-src", "data-lazy-src", "url"} {
			if reference = strings.TrimSpace(archiveAttribute(node, key)); reference != "" {
				break
			}
		}
		target, err := article.Base.Parse(reference)
		id := ""
		if err == nil {
			id = youtubeVideoID(target)
		}
		if id == "" || node.Parent == nil {
			continue
		}
		placeholder := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P, Attr: []html.Attribute{{Key: archiveVideoMarker, Val: id}}}
		placeholder.AppendChild(&html.Node{Type: html.TextNode, Data: "▶ Video, not archived: https://www.youtube.com/watch?v=" + id})
		node.Parent.InsertBefore(placeholder, node)
		node.Parent.RemoveChild(node)
		if fetcher != nil && fetcher.embedVideos && !containsString(article.Videos, id) {
			article.Videos = append(article.Videos, id)
		}
	}
}

// archiveVideos stores the article's videos and places a player where each
// was embedded, or at the top for a link to the video itself. It fails only
// when the article is nothing but videos and none could be stored.
func archiveVideos(fetcher *archiveFetcher, article *archiveArticle, output, entry string) ([]archivedPage, error) {
	pages := make([]archivedPage, 0)
	videoOnly := len(article.Videos) > 0 && strings.TrimSpace(archiveNodeText(article.Content)) == ""
	var failure error
	for index, id := range article.Videos {
		if index >= maxArchiveVideos {
			break
		}
		video, err := archiveStoreVideo(fetcher, id, output)
		if err != nil {
			slog.Warn("video not archived", "video", id, "err", err)
			failure = err
			continue
		}
		for _, local := range []string{video.Poster, video.File, video.Captions} {
			if local != "" {
				pages = append(pages, archivedPage{URL: "https://www.youtube.com/watch?v=" + id, Path: local})
			}
		}
		player := video.player(entry)
		placed := false
		for _, placeholder := range cascadia.QueryAll(article.Content, cascadia.MustCompile("p["+archiveVideoMarker+"]")) {
			if archiveAttribute(placeholder, archiveVideoMarker) == id && placeholder.Parent != nil {
				placeholder.Parent.InsertBefore(player, placeholder)
				placeholder.Parent.RemoveChild(placeholder)
				placed = true
				break
			}
		}
		if !placed {
			article.Content.InsertBefore(player, article.Content.FirstChild)
		}
		if article.Title == "" {
			article.Title = video.Title
		}
		if article.Author == "" {
			article.Author = video.Uploader
		}
		if !placed && len(strings.TrimSpace(archiveNodeText(article.Content))) == 0 {
			appendArchiveText(article.Content, video.Description)
		}
	}
	if videoOnly && len(pages) == 0 && failure != nil {
		return nil, failure
	}
	return pages, nil
}

// archiveStoreVideo downloads one YouTube video as WebM (VP9 and Opus where
// YouTube has them), with its poster and English captions.
func archiveStoreVideo(fetcher *archiveFetcher, id, output string) (archiveVideo, error) {
	fetcher.videos <- struct{}{}
	defer func() { <-fetcher.videos }()
	relative := filepath.ToSlash(filepath.Join("_video", id))
	directory := filepath.Join(output, filepath.FromSlash(relative))
	if err := os.RemoveAll(directory); err != nil {
		return archiveVideo{}, err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return archiveVideo{}, err
	}
	height := fetcher.videoHeight
	format := fmt.Sprintf("bv*[height<=%[1]d][vcodec^=vp9]+ba[acodec=opus]/bv*[height<=%[1]d]+ba/b[height<=%[1]d]/b", height)
	args := []string{
		"--no-playlist", "--no-progress", "--quiet", "--no-warnings",
		"-f", format, "--merge-output-format", "webm", "--recode-video", "webm",
		"--match-filter", fmt.Sprintf("duration <= %d", fetcher.videoMinutes*60),
		"--write-info-json", "--write-thumbnail", "--convert-thumbnails", "jpg",
		"-o", filepath.Join(directory, "video.%(ext)s"),
		"https://www.youtube.com/watch?v=" + id,
	}
	captions := []string{"--write-subs", "--write-auto-subs", "--sub-langs", "en", "--sub-format", "vtt", "--convert-subs", "vtt"}
	// YouTube rate-limits caption requests on their own, and yt-dlp then
	// drops the whole video; a second try goes without captions.
	if _, err := runToolCommand(fetcher.ctx, fetcher.root, fetcher.workdir, BaseToolsImage, "yt-dlp", append(captions, args...)); err != nil {
		slog.Warn("video download failed, retrying without captions", "video", id, "err", err)
		if _, err := runToolCommand(fetcher.ctx, fetcher.root, fetcher.workdir, BaseToolsImage, "yt-dlp", args); err != nil {
			return archiveVideo{}, err
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "video.webm")); err != nil {
		return archiveVideo{}, fmt.Errorf("yt-dlp stored no video for %s (longer than %d minutes or unavailable)", id, fetcher.videoMinutes)
	}
	video := archiveVideo{ID: id, File: relative + "/video.webm"}
	if data, err := os.ReadFile(filepath.Join(directory, "video.info.json")); err == nil {
		var info struct {
			Title       string `json:"title"`
			Uploader    string `json:"uploader"`
			Channel     string `json:"channel"`
			Description string `json:"description"`
		}
		if json.Unmarshal(data, &info) == nil {
			video.Title, video.Uploader, video.Description = info.Title, info.Uploader, info.Description
			if video.Uploader == "" {
				video.Uploader = info.Channel
			}
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "video.jpg")); err == nil {
		video.Poster = relative + "/video.jpg"
	}
	for _, language := range []string{"en", "en-US", "en-GB", "en-orig"} {
		if _, err := os.Stat(filepath.Join(directory, "video."+language+".vtt")); err == nil {
			video.Captions = relative + "/video." + language + ".vtt"
			break
		}
	}
	entries, _ := os.ReadDir(directory)
	for _, entry := range entries {
		if local := relative + "/" + entry.Name(); local != video.File && local != video.Poster && local != video.Captions {
			_ = os.RemoveAll(filepath.Join(directory, entry.Name()))
		}
	}
	return video, nil
}

func (video archiveVideo) player(entry string) *html.Node {
	player := &html.Node{Type: html.ElementNode, Data: "video", DataAtom: atom.Video, Attr: []html.Attribute{{Key: "controls"}, {Key: "preload", Val: "metadata"}}}
	if video.Poster != "" {
		player.Attr = append(player.Attr, html.Attribute{Key: "poster", Val: archiveRelative(entry, video.Poster)})
	}
	player.AppendChild(&html.Node{Type: html.ElementNode, Data: "source", DataAtom: atom.Source, Attr: []html.Attribute{
		{Key: "src", Val: archiveRelative(entry, video.File)}, {Key: "type", Val: "video/webm"},
	}})
	if video.Captions != "" {
		player.AppendChild(&html.Node{Type: html.ElementNode, Data: "track", DataAtom: atom.Track, Attr: []html.Attribute{
			{Key: "kind", Val: "captions"}, {Key: "src", Val: archiveRelative(entry, video.Captions)}, {Key: "srclang", Val: "en"}, {Key: "label", Val: "English"},
		}})
	}
	return player
}

// appendArchiveText adds plain text as paragraphs, keeping its line breaks.
func appendArchiveText(content *html.Node, text string) {
	for _, block := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		paragraph := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P}
		for index, line := range strings.Split(strings.TrimSpace(block), "\n") {
			if index > 0 {
				paragraph.AppendChild(&html.Node{Type: html.ElementNode, Data: "br", DataAtom: atom.Br})
			}
			paragraph.AppendChild(&html.Node{Type: html.TextNode, Data: line})
		}
		content.AppendChild(paragraph)
	}
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
