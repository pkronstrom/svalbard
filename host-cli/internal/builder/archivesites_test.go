package builder

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain keeps the archive tests off the real Wayback Machine, Printables and
// Thingiverse; tests that need them point these at local servers.
func TestMain(m *testing.M) {
	unreachable := "http://127.0.0.1:1"
	archiveWaybackAPI, archiveWaybackHost = unreachable+"/wayback/available", unreachable
	printablesAPI, printablesMedia, thingiverseAPI = unreachable+"/graphql/", unreachable+"/", unreachable
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
