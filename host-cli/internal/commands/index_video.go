package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/searchdb"
	"github.com/pkronstrom/svalbard/host-cli/internal/videotext"
	"github.com/pkronstrom/svalbard/host-cli/internal/zimext"
)

const (
	videoChaptersVersion = 1001
	videoSpeechVersion   = 1002
)

// VideoIndexOptions are opt-in for a single apply or index run.
type VideoIndexOptions struct {
	Mode    string
	Archive string
	Limit   int
}

func (o VideoIndexOptions) Enabled() bool { return o.Mode == "captions" || o.Mode == "transcribe" }

func (o VideoIndexOptions) Validate() error {
	switch o.Mode {
	case "", "off", "captions", "transcribe":
	default:
		return fmt.Errorf("unknown --video-index mode %q; use off, captions or transcribe", o.Mode)
	}
	if o.Archive != "" && (!o.Enabled() || filepath.Base(o.Archive) != o.Archive ||
		!strings.EqualFold(filepath.Ext(o.Archive), ".zim")) {
		return fmt.Errorf("--video-zim requires video indexing and an installed .zim filename")
	}
	if o.Limit < 0 || o.Limit != 0 && o.Mode != "transcribe" {
		return fmt.Errorf("--video-limit requires transcribe mode and a nonnegative number")
	}
	return nil
}

func isVideoTranscript(source searchdb.Source) bool {
	return source.ReaderVersion == videoChaptersVersion || source.ReaderVersion == videoSpeechVersion
}

func indexVideoTranscriptsUnlocked(ctx context.Context, root string, o VideoIndexOptions, w io.Writer) error {
	files, err := ScanZIMFiles(root)
	if errors.Is(err, os.ErrNotExist) && o.Archive == "" {
		fmt.Fprintln(w, "Video index: no installed ZIM archives")
		return nil
	}
	if err != nil {
		return err
	}
	if o.Archive != "" {
		if !slices.Contains(files, o.Archive) {
			return fmt.Errorf("video archive %q is not installed", o.Archive)
		}
		files = []string{o.Archive}
	}
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		return err
	}
	db, err := searchdb.Open(filepath.Join(data, "search.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	stored, err := db.IndexedSources()
	if err != nil {
		return err
	}
	pass := videoIndexPass{ctx: ctx, db: db, root: root, options: o, progress: w, sources: make(map[string]searchdb.Source)}
	for _, source := range stored {
		if isVideoTranscript(source) {
			pass.sources[source.Filename] = source
		}
	}
	for _, filename := range files {
		if err := pass.indexArchive(filename); err != nil {
			return fmt.Errorf("video indexing %s: %w", filename, err)
		}
	}
	fmt.Fprintf(w, "Video index: %d updated, %d unchanged, %d awaiting transcription\n", pass.updated, pass.unchanged, pass.pending)
	return nil
}

type videoIndexPass struct {
	ctx         context.Context
	db          *searchdb.DB
	root        string
	options     VideoIndexOptions
	progress    io.Writer
	sources     map[string]searchdb.Source
	runner      *videotext.Runner
	transcribed int
	updated     int
	unchanged   int
	pending     int
}

func (p *videoIndexPass) indexArchive(filename string) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(p.root, "zim", filename)
	identity, err := archiveSourceIdentity(p.ctx, path, filename)
	if err != nil {
		return err
	}
	archive, err := zimext.OpenArchive(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	if archive.UUID() != identity.UUID {
		return fmt.Errorf("archive changed while opening")
	}
	videos, err := archive.Videos(p.ctx)
	if err != nil {
		return err
	}
	for _, video := range videos {
		if err := p.indexVideo(path, archive, identity, video); err != nil {
			return fmt.Errorf("%s: %w", video.Path, err)
		}
	}
	return nil
}

func (p *videoIndexPass) indexVideo(path string, archive *zimext.Archive, identity searchdb.Source, video zimext.Video) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	name := embeddedSourceName(identity.Filename, video.Path)
	previous := p.sources[name]
	same := previous.IdentityCurrent(identity)
	if same && previous.ReaderVersion == videoSpeechVersion {
		p.unchanged++
		return nil
	}
	wantSpeech := p.options.Mode == "transcribe" && (p.options.Limit == 0 || p.transcribed < p.options.Limit)
	if same && !wantSpeech {
		p.unchanged++
		p.pending++
		return nil
	}

	var cues []videotext.Cue
	speech := false
	for _, track := range append(slices.Clone(video.Captions), video.Chapters) {
		if track == "" {
			continue
		}
		data, err := archive.ReadEntry(track, 8<<20)
		if err != nil {
			return err
		}
		entries, err := videotext.ParseCaptions(data)
		if err != nil {
			return err
		}
		speech = speech || track != video.Chapters && len(entries) > 0
		cues = append(cues, entries...)
	}
	if !speech && wantSpeech {
		if p.runner == nil {
			var err error
			p.runner, err = videotext.NewRunner(p.ctx, p.progress)
			if err != nil {
				return err
			}
		}
		fmt.Fprintf(p.progress, "Transcribing %s\n", video.Title)
		transcript, err := p.runner.Transcribe(p.ctx, func(w io.Writer) error {
			return archive.CopyVideo(p.ctx, video.Path, w)
		})
		if err != nil {
			return err
		}
		p.transcribed++
		cues = append(cues, transcript...)
		speech = true
	}
	// Recheck the original file after recognition before publishing any cues.
	current, err := archiveSourceIdentity(p.ctx, path, identity.Filename)
	if err != nil || !identity.IdentityCurrent(current) {
		return fmt.Errorf("archive changed during indexing")
	}
	source := identity
	source.Filename, source.Title = name, video.Title
	source.ReaderVersion = videoChaptersVersion
	if speech {
		source.ReaderVersion = videoSpeechVersion
		if len(cues) == 0 {
			source.Note = "No speech recognized"
		}
	} else {
		source.Note = "Speech transcript not indexed"
		p.pending++
		if len(cues) == 0 {
			return nil
		}
	}
	articles := videoArticles(video.Title, cues)
	if err := p.db.ReplaceTranscript(p.ctx, source, articles); err != nil {
		return err
	}
	if len(articles) > 0 {
		fmt.Fprintf(p.progress, "%s: %d timed passages\n", video.Title, len(articles))
	}
	p.updated++
	return nil
}

func videoArticles(title string, cues []videotext.Cue) []searchdb.Article {
	slices.SortStableFunc(cues, func(a, b videotext.Cue) int {
		switch {
		case a.StartMS < b.StartMS:
			return -1
		case a.StartMS > b.StartMS:
			return 1
		}
		return 0
	})
	var articles []searchdb.Article
	for _, cue := range cues {
		path := fmt.Sprintf("time/%d", cue.StartMS)
		if len(articles) > 0 && articles[len(articles)-1].Path == path {
			last := &articles[len(articles)-1]
			if last.Body != cue.Text {
				last.Body += "\n" + cue.Text
			}
			continue
		}
		minutes, seconds := cue.StartMS/60000, cue.StartMS/1000%60
		articles = append(articles, searchdb.Article{
			Path: path, Title: fmt.Sprintf("%s · %d:%02d", title, minutes, seconds), Body: cue.Text,
		})
	}
	return articles
}
