package builder

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

// TestMain keeps the archive tests off the real Wayback Machine, Printables and
// Thingiverse and GitHub; tests that need them point these at local servers.
func TestMain(m *testing.M) {
	unreachable := "http://127.0.0.1:1"
	archiveWaybackAPI, archiveWaybackHost = unreachable+"/wayback/available", unreachable
	printablesAPI, printablesMedia, thingiverseAPI = unreachable+"/graphql/", unreachable+"/", unreachable
	githubWeb, githubAPI, githubRaw, githubCodeload, githubMedia = unreachable, unreachable, unreachable, unreachable, unreachable
	os.Exit(m.Run())
}

func withArchiveEndpoint(t *testing.T, endpoint *string, value string) {
	previous := *endpoint
	*endpoint = value
	t.Cleanup(func() { *endpoint = previous })
}

// stubYTDLP fakes yt-dlp by writing what it would leave in the -o directory.
func stubYTDLP(t *testing.T, calls *[]string) {
	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, tool string, args []string) (string, error) {
		if tool != "yt-dlp" {
			return previous(context.Background(), "", "", "", tool, args)
		}
		*calls = append(*calls, args[len(args)-1])
		directory := ""
		for index, arg := range args {
			if arg == "-o" {
				directory = filepath.Dir(args[index+1])
			}
		}
		files := map[string]string{
			"video.webm":        "webm",
			"video.jpg":         string(bytes.Repeat([]byte("j"), 600)),
			"video.en.vtt":      "WEBVTT",
			"video.info.json":   `{"title":"Roll Top Sack","uploader":"Jason","description":"Sew it.\n\nThen roll it."}`,
			"video.en-orig.vtt": "WEBVTT",
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
				return "", err
			}
		}
		return "", nil
	}
	t.Cleanup(func() { runToolCommand = previous })
}

func readArchivePage(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestArchiveReaderStoresYouTubeLinkAsVideo(t *testing.T) {
	var calls []string
	stubYTDLP(t, &calls)
	target, _ := url.Parse("https://www.youtube.com/watch?v=RbSKAmLdtpw")
	output := filepath.Join(t.TempDir(), "projects", "video")
	_, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 70), output, testArchiveSite, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
	for _, want := range []string{
		"<h1>Roll Top Sack</h1>", "by Jason", `<video controls="" preload="metadata" poster="_video/RbSKAmLdtpw/video.jpg">`,
		`<source src="_video/RbSKAmLdtpw/video.webm" type="video/webm"/>`, `<track kind="captions" src="_video/RbSKAmLdtpw/video.en.vtt" srclang="en" label="English"/>`,
		"<p>Sew it.</p><p>Then roll it.</p>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
	if len(calls) != 1 || calls[0] != "https://www.youtube.com/watch?v=RbSKAmLdtpw" {
		t.Fatalf("yt-dlp calls = %v", calls)
	}
	if _, err := os.Stat(filepath.Join(output, "_video", "RbSKAmLdtpw", "video.info.json")); !os.IsNotExist(err) {
		t.Errorf("metadata kept in archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "_video", "RbSKAmLdtpw", "video.en-orig.vtt")); !os.IsNotExist(err) {
		t.Errorf("duplicate captions kept in archive: %v", err)
	}
}

func TestArchiveReaderPlacesEmbeddedVideosOnlyWhenEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte(`<html><body><article><p>` + strings.Repeat("Build the bag. ", 30) + `</p>
			<iframe src="https://www.youtube-nocookie.com/embed/CfYNEKoM7tw?rel=0"></iframe><p>After the video.</p></article></body></html>`))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/bag")
	for _, enabled := range []bool{false, true} {
		var calls []string
		stubYTDLP(t, &calls)
		fetcher := newArchiveFetcher(context.Background())
		fetcher.embedVideos = enabled
		output := filepath.Join(t.TempDir(), "projects", "bag")
		if _, err := archiveReader(fetcher, newArchiveSource(target, 0), output, testArchiveSite, nil); err != nil {
			t.Fatal(err)
		}
		page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
		placeholder := "Video, not archived: https://www.youtube.com/watch?v=CfYNEKoM7tw"
		player := `<source src="_video/CfYNEKoM7tw/video.webm"`
		if enabled {
			if len(calls) != 1 || !strings.Contains(page, player) || strings.Contains(page, placeholder) || strings.Index(page, player) > strings.Index(page, "After the video.") {
				t.Errorf("embedded video not placed inline (calls %v): %s", calls, page)
			}
		} else if len(calls) != 0 || !strings.Contains(page, placeholder) || strings.Contains(page, "<video") {
			t.Errorf("embedded video downloaded while disabled (calls %v): %s", calls, page)
		}
	}
}

func TestPrintablesArticleReadsModelImagesAndFiles(t *testing.T) {
	picture := bytes.Repeat([]byte("jpeg"), 200)
	var mutations []map[string]any
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/graphql/":
			body, _ := io.ReadAll(request.Body)
			var call struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			_ = json.Unmarshal(body, &call)
			writer.Header().Set("Content-Type", "application/json")
			if strings.HasPrefix(call.Query, "mutation") {
				mutations = append(mutations, call.Variables)
				_, _ = writer.Write([]byte(`{"data":{"getDownloadLink":{"ok":true,"output":{"files":[{"id":"11","link":"` + server.URL + `/files/dpad.stl"}]}}}}`))
				return
			}
			if call.Variables["id"] != "7164" || request.Header.Get("Origin") != "https://www.printables.com" {
				http.Error(writer, "bad request", http.StatusBadRequest)
				return
			}
			_, _ = writer.Write([]byte(`{"data":{"print":{"name":"PiGRRL Zero","summary":"Gameboy","description":"<p>Turn a Pi Zero into a handheld.</p><figure><oembed url=\"https://www.youtube.com/watch?v=UAIILlANNj4\"></oembed></figure>",
				"license":{"name":"CC BY"},"user":{"publicUsername":"Adafruit"},
				"images":[{"filePath":"media/prints/7164/a.jpg"},{"filePath":"media/prints/7164/b.jpg"}],
				"stls":[{"id":"11","name":"dpad.stl","fileSize":100},{"id":"12","name":"huge.stl","fileSize":999999999}],"otherFiles":[]}}}`))
		case "/media/prints/7164/a.jpg", "/media/prints/7164/b.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write(picture)
		case "/files/dpad.stl":
			_, _ = writer.Write([]byte("solid dpad"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	withArchiveEndpoint(t, &printablesAPI, server.URL+"/graphql/")
	withArchiveEndpoint(t, &printablesMedia, server.URL+"/")

	target, _ := url.Parse("https://www.printables.com/model/7164-pigrrl-zero-raspberry-pi-zero-gameboy")
	output := filepath.Join(t.TempDir(), "projects", "pigrrl")
	if _, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 10), output, testArchiveSite, nil); err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
	for _, want := range []string{"<h1>PiGRRL Zero</h1>", "by Adafruit", "CC BY", "Turn a Pi Zero into a handheld.", "<h2>Pictures</h2>", `<img src="../_assets/`, `href="../_files/`, "dpad.stl", "Video, not archived: https://www.youtube.com/watch?v=UAIILlANNj4"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
	if len(mutations) != 1 || !strings.Contains(mustJSON(t, mutations[0]), `"ids":["11"]`) {
		t.Errorf("download request = %v, want only the file that fits", mutations)
	}
}

func TestThingiverseArticleUsesAPIWithToken(t *testing.T) {
	picture := bytes.Repeat([]byte("jpeg"), 200)
	var unauthorized []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/things/") && request.Header.Get("Authorization") != "Bearer secret" {
			unauthorized = append(unauthorized, request.URL.Path)
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/things/3671784":
			_, _ = writer.Write([]byte(`{"name":"simplyRetro Z5","license":"CC BY-NC","creator":{"name":"Geaz"},"details":"<div><h1>Summary</h1><p>Handheld for RetroPie.</p></div>"}`))
		case "/things/3671784/images":
			_, _ = writer.Write([]byte(`[{"sizes":[{"type":"preview","size":"featured","url":"` + server.URL + `/resize/a.jpg"},{"type":"display","size":"large","url":"` + server.URL + `/display/a.jpg"}]}]`))
		case "/things/3671784/files":
			_, _ = writer.Write([]byte(`[{"name":"Frame.stl","size":10,"download_url":"` + server.URL + `/files/1/download"}]`))
		case "/display/a.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write(picture)
		case "/files/1/download":
			_, _ = writer.Write([]byte("solid frame"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	withArchiveEndpoint(t, &thingiverseAPI, server.URL)
	fetcher := newArchiveFetcher(context.Background())
	fetcher.authorization[thingiverseAPIHost()] = "Bearer secret"
	if fetcher.pace[thingiverseAPIHost()] != thingiverseRequestInterval {
		t.Errorf("thingiverse API pace = %v, want %v", fetcher.pace[thingiverseAPIHost()], thingiverseRequestInterval)
	}
	delete(fetcher.pace, thingiverseAPIHost())

	target, _ := url.Parse("https://www.thingiverse.com/thing:3671784")
	output := filepath.Join(t.TempDir(), "projects", "z5")
	if _, err := archiveReader(fetcher, newArchiveSource(target, 9), output, testArchiveSite, nil); err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
	for _, want := range []string{"<h1>simplyRetro Z5</h1>", "by Geaz", "CC BY-NC", "Handheld for RetroPie.", `<img src="_assets/`, `href="_files/`, "Frame.stl"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
	if len(unauthorized) != 0 {
		t.Errorf("API calls without token: %v", unauthorized)
	}
}

func TestArchiveReaderFallsBackToWaybackCapture(t *testing.T) {
	picture := bytes.Repeat([]byte("jpeg"), 200)
	live := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "blocked", http.StatusForbidden)
	}))
	defer live.Close()
	var lookups []string
	wayback := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/wayback/available":
			lookups = append(lookups, request.URL.Query().Get("url"))
			_, _ = writer.Write([]byte(`{"archived_snapshots":{"closest":{"available":true,"status":"200","timestamp":"20240105120000"}}}`))
		case request.URL.Path == "/web/20240105120000id_/"+live.URL+"/tent":
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte(`<html><head><title>Pyramid Tent</title></head><body><article><p>` + strings.Repeat("Sew the tent. ", 30) + `</p><img src="pics/tent.jpg"></article></body></html>`))
		case request.URL.Path == "/web/20240105120000im_/"+live.URL+"/pics/tent.jpg":
			writer.Header().Set("Content-Type", "image/jpeg")
			_, _ = writer.Write(picture)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer wayback.Close()
	withArchiveEndpoint(t, &archiveWaybackAPI, wayback.URL+"/wayback/available")
	withArchiveEndpoint(t, &archiveWaybackHost, wayback.URL)

	target, _ := url.Parse(live.URL + "/tent")
	output := filepath.Join(t.TempDir(), "projects", "tent")
	if _, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 0), output, testArchiveSite, nil); err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
	for _, want := range []string{"<h1>Pyramid Tent</h1>", "Sew the tent.", `<img src="_assets/`, "Wayback Machine copy from 2024-01-05"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
	if len(lookups) != 1 || lookups[0] != target.String() {
		t.Errorf("wayback lookups = %v", lookups)
	}
}

func TestYouTubeVideoIDRecognizesAddressForms(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.youtube.com/watch?v=RbSKAmLdtpw&t=10":         "RbSKAmLdtpw",
		"https://youtu.be/RbSKAmLdtpw":                             "RbSKAmLdtpw",
		"https://www.youtube-nocookie.com/embed/RbSKAmLdtpw?rel=0": "RbSKAmLdtpw",
		"https://m.youtube.com/shorts/RbSKAmLdtpw":                 "RbSKAmLdtpw",
		"https://www.youtube.com/channel/UC123":                    "",
		"https://vimeo.com/123456":                                 "",
	} {
		target, _ := url.Parse(raw)
		if got := youtubeVideoID(target); got != want {
			t.Errorf("youtubeVideoID(%s) = %q, want %q", raw, got, want)
		}
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestArchiveReaderFailsYouTubeLinkWhenVideoIsNotStored(t *testing.T) {
	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, _ string, _ []string) (string, error) {
		return "", errors.New("yt-dlp: Sign in to confirm you're not a bot")
	}
	t.Cleanup(func() { runToolCommand = previous })
	target, _ := url.Parse("https://youtu.be/RbSKAmLdtpw")
	_, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 0), filepath.Join(t.TempDir(), "video"), testArchiveSite, nil)
	if err == nil || !strings.Contains(err.Error(), "not a bot") {
		t.Fatalf("archiveReader() error = %v, want the yt-dlp failure", err)
	}
}

func TestThingiverseImageURLPrefersDisplayRendition(t *testing.T) {
	var image thingiverseImage
	if err := json.Unmarshal([]byte(`{"sizes":[
		{"type":"thumb","size":"large","url":"https://cdn.thingiverse.com/assets/a.jpg"},
		{"type":"preview","size":"featured","url":"https://resize.thingiverse.com/?url=a&w=628"},
		{"type":"display","size":"large","url":"https://resize.thingiverse.com/?url=a&w=1024"}]}`), &image); err != nil {
		t.Fatal(err)
	}
	if got := thingiverseImageURL(image); got != "https://resize.thingiverse.com/?url=a&w=1024" {
		t.Errorf("thingiverseImageURL() = %q, want the display rendition", got)
	}
}

func TestArchiveFetcherWaitsOutRateLimits(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			writer.Header().Set("Retry-After", "1")
			http.Error(writer, "You can only send 300 requests per 5 minutes", http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/things/1")
	fetcher := newArchiveFetcher(context.Background())
	fetcher.paceMutex.Lock()
	fetcher.pace[target.Host] = 200 * time.Millisecond
	fetcher.paceMutex.Unlock()
	started := time.Now()
	for range 2 {
		if body, _, _, err := fetcher.get(target, 100); err != nil || string(body) != "ok" {
			t.Fatalf("get() = %q, %v", body, err)
		}
	}
	if elapsed := time.Since(started); elapsed < 1200*time.Millisecond {
		t.Errorf("finished in %v, want the Retry-After second plus pacing", elapsed)
	}
	if requests != 3 {
		t.Errorf("requests = %d, want 3", requests)
	}
}

func TestArchiveStoreVideoRetriesWithoutCaptions(t *testing.T) {
	var calls []string
	stubYTDLP(t, &calls)
	stored := runToolCommand
	attempts := 0
	runToolCommand = func(ctx context.Context, root, workdir, image, tool string, args []string) (string, error) {
		attempts++
		if containsString(args, "--write-subs") {
			return "", errors.New("ERROR: Unable to download video subtitles for 'en': HTTP Error 429: Too Many Requests")
		}
		return stored(ctx, root, workdir, image, tool, args)
	}
	video, err := archiveStoreVideo(newArchiveFetcher(context.Background()), "quC84fgN63w", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || video.File != "_video/quC84fgN63w/video.webm" {
		t.Errorf("attempts = %d, video = %+v", attempts, video)
	}
}

func TestArchiveFetcherPacesThingiverseOnlyWithToken(t *testing.T) {
	thing, _ := url.Parse("https://www.thingiverse.com/thing:1")
	other, _ := url.Parse("https://www.printables.com/model/1")
	fetcher := newArchiveFetcher(context.Background())
	delete(fetcher.authorization, thingiverseAPIHost())
	if fetcher.paced(thing) {
		t.Error("thing paced without a token")
	}
	fetcher.authorization[thingiverseAPIHost()] = "Bearer secret"
	if !fetcher.paced(thing) || fetcher.paced(other) {
		t.Errorf("paced(thing) = %v, paced(other) = %v", fetcher.paced(thing), fetcher.paced(other))
	}
}

func TestArchiveReaderDecodesLatin1Pages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = writer.Write([]byte("<html><head><title>Ruche Warr\xe9</title></head><body><p>" + strings.Repeat("La ruche populaire de l'abb\xe9 Warr\xe9. ", 20) + "</p></body></html>"))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/plans.htm")
	output := filepath.Join(t.TempDir(), "hive")
	if _, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 0), output, testArchiveSite, nil); err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
	if !strings.Contains(page, "<h1>Ruche Warré</h1>") || !strings.Contains(page, "abbé Warré") {
		t.Errorf("Latin-1 text not decoded: %s", page)
	}
}

func TestArchiveFetcherHoldsPacedHostAfterRateLimit(t *testing.T) {
	fetcher := newArchiveFetcher(context.Background())
	fetcher.pace["api.example.test"] = time.Millisecond
	until := time.Now().Add(time.Minute)
	fetcher.holdHost("api.example.test", until)
	if next := fetcher.turn("api.example.test"); next.Before(until) {
		t.Errorf("turn() = %v, want at or after %v", next, until)
	}
	if fetcher.pace["api.example.test"] != 2*time.Millisecond {
		t.Errorf("pace after rate limit = %v, want it doubled", fetcher.pace["api.example.test"])
	}
	fetcher.holdHost("unpaced.example.test", until)
	if next := fetcher.turn("unpaced.example.test"); !next.IsZero() {
		t.Errorf("unpaced host held until %v", next)
	}
}

func TestArchiveFetcherSkipsUnreachableHost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	target, _ := url.Parse("http://" + address + "/picture.jpg")
	fetcher := newArchiveFetcher(context.Background())
	for range archiveUnreachableAfter {
		if _, _, _, err := fetcher.get(target, 100); err == nil || strings.Contains(err.Error(), "skipped") {
			t.Fatalf("get() error = %v, want a connection error", err)
		}
	}
	if _, _, _, err := fetcher.get(target, 100); err == nil || !strings.Contains(err.Error(), "host unreachable, skipped") {
		t.Fatalf("get() error = %v, want the host skipped", err)
	}
}

func TestArchiveFetcherKeepsHostThatAnswersWithErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.NotFound(writer, nil)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/missing.jpg")
	fetcher := newArchiveFetcher(context.Background())
	for range archiveUnreachableAfter + 1 {
		if _, _, _, err := fetcher.get(target, 100); err == nil || strings.Contains(err.Error(), "skipped") {
			t.Fatalf("get() error = %v, want the 404", err)
		}
	}
}

func TestArchiveReaderBundlesManyFilesIntoOneZip(t *testing.T) {
	links := ""
	for index := range 5 {
		links += fmt.Sprintf(`<a href="/parts/Part %d.stl">Part %d</a> `, index, index)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/parts/") {
			_, _ = writer.Write([]byte("solid " + strings.Repeat("facet ", 200)))
			return
		}
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte(`<html><head><title>Telescope</title></head><body><article><p>` + strings.Repeat("Print the parts. ", 30) + `</p>` + links + `</article></body></html>`))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/telescope")
	output := filepath.Join(t.TempDir(), "scope")
	pages, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 0), output, testArchiveSite, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
	for _, want := range []string{`<a class="download" href="_files/telescope-files.zip">Download all 5 files</a>`, "<li>Part 0.stl <span", `<a href="_files/telescope-files.zip">Part 3</a>`} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
	archive, err := zip.OpenReader(filepath.Join(output, "_files", "telescope-files.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	var names []string
	for _, file := range archive.File {
		names = append(names, file.Name)
	}
	if strings.Join(names, ",") != "Part_0.stl,Part_1.stl,Part_2.stl,Part_3.stl,Part_4.stl" {
		t.Errorf("zip entries = %v", names)
	}
	loose, _ := filepath.Glob(filepath.Join(output, "_files", "*.stl"))
	if len(loose) != 0 {
		t.Errorf("loose files kept beside the zip: %v", loose)
	}
	recorded := 0
	for _, page := range pages {
		if strings.HasPrefix(page.Path, "_files/") {
			recorded++
		}
	}
	if recorded != 1 {
		t.Errorf("recorded %d file pages, want only the zip", recorded)
	}
}

func TestWriteArchiveIndexAddsSearchableCards(t *testing.T) {
	root := t.TempDir()
	lamp, _ := archiveURL("https://example.test/lamp")
	bench, _ := archiveURL("https://example.test/bench")
	sources := []archiveSource{newArchiveSource(lamp, 1), newArchiveSource(bench, 2)}
	sources[0].Category, sources[0].Label, sources[0].Author = "HOME / LIGHTING", "Concrete Pendant Lamp", "Modern Builds"
	sources[1].Category, sources[1].Label = "FURNITURE / CHAIRS, STOOLS & BENCHES", "Garden Bench"
	index := filepath.Join(root, "index.html")
	if err := writeArchiveIndex(index, root, sources, "Make It Yourself", "https://example.test/list.pdf"); err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, index)
	for _, want := range []string{
		`<input id="search" type="search"`, `data-search="concrete pendant lamp  by modern builds · example.test home / lighting"`,
		`<section class="top"><h2 id="section-0">HOME</h2><section class="group"><h3>LIGHTING</h3>`, `</div></section></section><p id="nothing" hidden>`, "card.text.indexOf(term)",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("index lacks %q: %s", want, page)
		}
	}
}

func TestArchiveSubpagesFollowGuideChaptersOnly(t *testing.T) {
	cases := []struct {
		base  string
		links []string
		want  []string
	}{
		{"https://freesewing.org/docs/designs/aaron/", []string{"/docs/designs/aaron/instructions/", "/docs/designs/aaron/options/", "/docs/designs/brian/", "/docs/designs/aaron/feed/"},
			[]string{"/docs/designs/aaron/instructions/", "/docs/designs/aaron/options/"}},
		{"https://woodgears.ca/ukulele/index.html", []string{"body.html", "neck.html", "/other/x.html", "plans.pdf", "index.html"},
			[]string{"/ukulele/body.html", "/ukulele/neck.html"}},
		{"https://example.com/blog/post.html", []string{"other.html"}, nil},
		{"https://example.com/", []string{"/anything"}, nil},
		{"https://blog.example/2019/05/bivvy/", []string{"/2019/05/bivvy/img_3146/", "/2019/05/bivvy/2/"}, []string{"/2019/05/bivvy/2/"}},
		{"https://learn.adafruit.com/pigrrl-2/overview", []string{"/pigrrl-2/parts", "/other-guide/x"}, []string{"/pigrrl-2/parts"}},
		{"https://hackaday.io/project/26157-pipod", []string{"/project/26157/log/145836-update-4", "/project/26157/gallery", "/project/999/log/1"},
			[]string{"/project/26157/instructions", "/project/26157/files", "/project/26157/log/145836-update-4"}},
		{"https://github.com/dekuNukem/duckyPad", []string{"/dekuNukem/duckyPad/blob/master/getting_started.md", "/dekuNukem/duckyPad/issues", "/dekuNukem/duckyPad/wiki/Build", "/dekuNukem/duckyPad/wiki", "/other/repo/blob/main/README.md"},
			[]string{"/dekuNukem/duckyPad/blob/master/getting_started.md", "/dekuNukem/duckyPad/wiki/Build", "/dekuNukem/duckyPad/wiki"}},
	}
	for _, test := range cases {
		base, _ := url.Parse(test.base)
		links := make([]*url.URL, 0, len(test.links))
		for _, link := range test.links {
			resolved, _ := base.Parse(link)
			links = append(links, resolved)
		}
		var got []string
		for _, subpage := range archiveSubpages(base, links) {
			got = append(got, subpage.Path)
		}
		if fmt.Sprint(got) != fmt.Sprint(test.want) {
			t.Errorf("%s: subpages %v, want %v", test.base, got, test.want)
		}
	}
}

func TestArchiveReaderAppendsSubpagesAsSections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		switch request.URL.Path {
		case "/designs/aaron/":
			_, _ = writer.Write([]byte(`<html><head><title>Aaron</title></head><body><nav><a href="instructions/">Instructions</a> <a href="/designs/brian/">Brian</a></nav><main><p>` + strings.Repeat("A sleeveless shirt. ", 20) + `</p></main></body></html>`))
		case "/designs/aaron/instructions/":
			_, _ = writer.Write([]byte(`<html><head><title>Aaron instructions | FreeSewing</title></head><body><main><p>` + strings.Repeat("Sew the side seams. ", 20) + `</p><a href="pattern.pdf">Pattern</a></main></body></html>`))
		case "/designs/aaron/instructions/pattern.pdf":
			writer.Header().Set("Content-Type", "application/pdf")
			_, _ = writer.Write([]byte("%PDF-1.4 pattern"))
		default:
			t.Errorf("unexpected request %s", request.URL.Path)
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL + "/designs/aaron/")
	output := filepath.Join(t.TempDir(), "aaron")
	if _, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 0), output, testArchiveSite, nil); err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, archiveEntryPath(target)))
	for _, want := range []string{`<section><h2>Aaron instructions</h2>`, "Sew the side seams.", "pattern.pdf"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
}

func TestArchiveReaderRejectsChallengePagesAndMislabelledFiles(t *testing.T) {
	document, _ := html.Parse(strings.NewReader(`<html><body><p>Please wait while your request is being verified...</p></body></html>`))
	if !archiveChallengePage(document) {
		t.Error("challenge page not recognized")
	}
	for name, body := range map[string]string{"hive.zip": "<!DOCTYPE html><html>Not found</html>", "plan.pdf": "<html>gone</html>", "part.stl": "<!doctype html><title>404</title>"} {
		if archiveFileMatches([]byte(body), name) {
			t.Errorf("%s accepted an HTML page", name)
		}
	}
	for name, body := range map[string]string{"hive.zip": "PK\x03\x04", "plan.pdf": "%PDF-1.7", "part.stl": "solid part"} {
		if !archiveFileMatches([]byte(body), name) {
			t.Errorf("%s rejected a real file", name)
		}
	}
}
