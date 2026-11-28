package search

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

func TestMergeZIMKeywordsUsesBothLanguagesAndSQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE sources (filename TEXT, native_language TEXT);
		INSERT INTO sources VALUES ('north.zim', 'eng'), ('south.zim', 'eng'), ('suomi.zim', 'fin'), ('phet.zim', '');`)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		books := r.URL.Query()["books.name"]
		switch {
		case reflect.DeepEqual(books, []string{"north", "south"}):
			fmt.Fprint(w, `<rss><channel>
				<item><title>Carbon</title><link>/content/north/Carbon</link><description><b>Carbon</b> dioxide</description></item>
				<item><title>Soils</title><link>/content/south/Soils</link><description>Organic &lt;b&gt;carbon&lt;/b&gt; in soil</description></item>
				<item><title>Incorrect</title><link>/content/other/Incorrect</link></item>
			</channel></rss>`)
		case reflect.DeepEqual(books, []string{"suomi"}):
			fmt.Fprint(w, `<rss><channel><item><title>Hiili</title><link>/content/suomi/Hiili</link></item></channel></rss>`)
		default:
			http.Error(w, fmt.Sprint(books), http.StatusBadRequest)
		}
	}))
	defer server.Close()
	start := func(context.Context) (int, error) { return server.Listener.Addr().(*net.TCPAddr).Port, nil }
	local := []Result{{Filename: "books/chemistry.pdf", Title: "Chemistry", Path: "page/3"}}
	got, used, err := MergeZIMKeywords(context.Background(), engine.New(db), start, "carbon", "", 5, local)
	if err != nil || !used || len(got) != 4 {
		t.Fatalf("combined search = %+v, used %v, error %v", got, used, err)
	}
	if got[0].Title != "Carbon" || got[0].Snippet != "Carbon dioxide" || got[0].Path != "/Carbon" {
		t.Errorf("first result = %+v", got[0])
	}
	for _, wanted := range []string{"south.zim", "suomi.zim", "books/chemistry.pdf"} {
		if !slices.ContainsFunc(got, func(r Result) bool { return r.Filename == wanted }) {
			t.Errorf("missing %s in %+v", wanted, got)
		}
	}
	if !slices.ContainsFunc(got, func(r Result) bool { return r.Snippet == "Organic carbon in soil" }) {
		t.Errorf("escaped HTML snippet was not rendered as plain text: %+v", got)
	}
	groups, err := engine.New(db).NativeZIMGroups(context.Background(), "", "north.zim")
	if err != nil || !reflect.DeepEqual(groups, []engine.NativeZIMGroup{{Language: "eng", Books: []string{"south"}}, {Language: "fin", Books: []string{"suomi"}}}) {
		t.Errorf("native search with an excluded archive uses %v, error %v", groups, err)
	}
}

func TestBlendPromotesCloseCultivationMatches(t *testing.T) {
	for _, words := range [][]string{{"farming", "potatoes"}, {"potato", "farming"}} {
		lists := [][]Result{
			{{Filename: "magazine.zim", Title: "Vertical Farming", Snippet: "A farm powered by solar panels."}},
			{{Filename: "garden.zim", Title: "Growing potatoes in tyres", Path: "/Tyres"},
				{Filename: "guide.zim", Title: "Local food", Snippet: "A long note on animal feed."}},
		}
		got := blendKeywordResults(lists, words, GrowingQuery(words), 1, "", 10)
		if len(got) != 2 || got[0].Title != "Growing potatoes in tyres" {
			t.Fatalf("search %v returned %+v", words, got)
		}
	}
}
