package engine

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
)

func TestQueryWords(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"water purification", []string{"water", "purification"}},
		// A question keeps its subject and loses the grammar around it.
		{"What is the maximum dose of paracetamol for adults?", []string{"maximum", "dose", "paracetamol", "adults"}},
		// A query of nothing but stop words is searched as written.
		{"The Who", []string{"The", "Who"}},
		{"co-codamol (tablets), dose; adults/children", []string{"co-codamol", "tablets", "dose", "adults", "children"}},
		{`"quoted" 'phrase'`, []string{"quoted", "phrase"}},
		{"dose Dose DOSE", []string{"dose"}},
		{" - ? ... ", nil},
		{"", nil},
		{"a1 a2 a3 a4 a5 a6 a7 a8 a9 a10 a11 a12 a13 a14", []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9", "a10", "a11", "a12"}},
	} {
		if got := QueryWords(tc.query); !slices.Equal(got, tc.want) {
			t.Errorf("QueryWords(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func paths(results []Result) []string {
	out := make([]string, len(results))
	for i, result := range results {
		out[i] = result.Path
	}
	return out
}

func search(t *testing.T, db *sql.DB, query string, opts KeywordOptions) KeywordMatches {
	t.Helper()
	if opts.Limit == 0 {
		opts.Limit = 10
	}
	matches, err := New(db).Search(context.Background(), query, opts)
	if err != nil {
		t.Fatalf("Search(%q) error = %v", query, err)
	}
	return matches
}

func TestSearchPutsAllWordMatchesFirstThenPartial(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "med.zim")
	insertArticle(t, db, src, "/all", "Antibiotics", "Doxycycline and alcohol: a known interaction.")
	insertArticle(t, db, src, "/two", "Doxycycline", "An antibiotic. Avoid alcohol.")
	insertArticle(t, db, src, "/one", "Alcohol", "Units, limits and withdrawal.")
	insertArticle(t, db, src, "/none", "Warfarin", "A blood thinner.")

	matches := search(t, db, "doxycycline alcohol interaction", KeywordOptions{})

	if matches.Exact != 1 || matches.Results[0].Path != "/all" || matches.Results[0].Partial {
		t.Fatalf("exact = %d, results = %v, want /all first as the only all-words match", matches.Exact, paths(matches.Results))
	}
	// One word of three is not most of them: /one is left out.
	if got := paths(matches.Results); !slices.Equal(got, []string{"/all", "/two"}) {
		t.Fatalf("results = %v, want the two-word article after the complete match and nothing else", got)
	}
	if partial := matches.Results[1]; !partial.Partial || !slices.Equal(partial.Matched, []string{"doxycycline", "alcohol"}) {
		t.Errorf("partial = %v, matched = %v, want a partial match naming its words", partial.Partial, partial.Matched)
	}
	if len(matches.Missing) != 0 {
		t.Errorf("missing = %v, want none: every word is in some article", matches.Missing)
	}
}

func TestSearchNamesWordsNoArticleHas(t *testing.T) {
	db := setupTestDB(t)
	med := insertSource(t, db, "med.zim")
	cars := insertSource(t, db, "cars.zim")
	insertArticle(t, db, med, "/bp", "Blood pressure", "High blood pressure in 2019 guidance.")
	insertArticle(t, db, cars, "/tyres", "Tyres", "Check the tyre pressure monthly.")

	matches := search(t, db, "2019 Toyota Corolla tyre pressure", KeywordOptions{})
	if len(matches.Results) != 0 || !slices.Equal(matches.Missing, []string{"Toyota", "Corolla"}) {
		t.Fatalf("results = %v, missing = %v, want nothing found and the two absent words named", paths(matches.Results), matches.Missing)
	}
	// Within one source, a word found only elsewhere is missing too.
	matches = search(t, db, "tyre pressure", KeywordOptions{Source: "med.zim"})
	if len(matches.Results) != 0 || !slices.Equal(matches.Missing, []string{"tyre"}) {
		t.Fatalf("results = %v, missing = %v, want \"tyre\" missing from med.zim", paths(matches.Results), matches.Missing)
	}
	// One of two words is a search for that word alone, not a partial match.
	if matches = search(t, db, "pressure cooker", KeywordOptions{}); len(matches.Results) != 0 || !slices.Equal(matches.Missing, []string{"cooker"}) {
		t.Fatalf("results = %v, missing = %v, want no one-of-two matches", paths(matches.Results), matches.Missing)
	}
	if matches = search(t, db, "Toyota", KeywordOptions{}); !slices.Equal(matches.Missing, []string{"Toyota"}) {
		t.Fatalf("missing = %v, want the one word", matches.Missing)
	}
	// A search that fills its limit does not pay for the extra lookups.
	if matches = search(t, db, "pressure", KeywordOptions{Limit: 2}); matches.Exact != 2 || matches.Missing != nil {
		t.Fatalf("exact = %d, missing = %v", matches.Exact, matches.Missing)
	}
}

func TestSearchSkipsPartialMatchesWhenAllWordsFillTheLimit(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "med.zim")
	insertArticle(t, db, src, "/a", "Water filter", "water filter sand one")
	insertArticle(t, db, src, "/b", "Water filter", "water filter sand two")
	insertArticle(t, db, src, "/c", "Water", "water and sand only")

	matches := search(t, db, "water filter sand", KeywordOptions{Limit: 2})
	if matches.Exact != 2 || len(matches.Results) != 2 {
		t.Fatalf("exact = %d, results = %v, want only the two all-words matches", matches.Exact, paths(matches.Results))
	}
}

func TestSearchRanksPartialMatchesByWordsMatched(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "med.zim")
	// Three of the five words, two of them rare and in the title: this
	// article has the best rank. The ones with four words must come first.
	insertArticle(t, db, src, "/three-words", "Rabies vaccine", "rabies vaccine after a bite, rabies vaccine schedule")
	insertArticle(t, db, src, "/four-words", "Prevention", "A long passage about many other things that mentions doxycycline once, a tick once, a bite once and rabies once, among plenty of unrelated words.")
	insertArticle(t, db, src, "/four-more", "Aftercare", "doxycycline and tick and bite again, and rabies, to make those words common")
	insertArticle(t, db, src, "/two-words", "Ticks", "tick bite")

	query := "doxycycline tick bite rabies vaccine"
	byRank, _, err := New(db).ranked(context.Background(), ftsTerms(QueryWords(query), " OR "), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if byRank[0].Path != "/three-words" {
		t.Fatalf("rank order = %v: the fixture no longer ranks the three-word article first, so this test proves nothing", paths(byRank))
	}

	matches := search(t, db, query, KeywordOptions{})

	if matches.Exact != 0 {
		t.Fatalf("exact = %d, want 0", matches.Exact)
	}
	got := paths(matches.Results)
	if len(got) != 3 || got[2] != "/three-words" {
		t.Fatalf("results = %v, want the four-word articles ahead of the three-word one", got)
	}
	// Two words of five are not most of them: that article is left out.
	if slices.Contains(got, "/two-words") {
		t.Errorf("results = %v, want the two-word article dropped", got)
	}
	for _, result := range matches.Results[:2] {
		if want := []string{"doxycycline", "tick", "bite", "rabies"}; !slices.Equal(result.Matched, want) {
			t.Errorf("%s matched = %v, want %v in query order", result.Path, result.Matched, want)
		}
	}
}

func TestSearchRanksTitleMatchesAboveBodyMatches(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "med.zim")
	insertArticle(t, db, src, "/mention", "Co-codamol", "Contains paracetamol. Do not take other paracetamol with it.")
	insertArticle(t, db, src, "/subject", "Paracetamol for adults", "A painkiller for pain and high temperature, widely sold in shops.")

	if got := paths(search(t, db, "paracetamol", KeywordOptions{}).Results); !slices.Equal(got, []string{"/subject", "/mention"}) {
		t.Fatalf("results = %v, want the article titled with the word first", got)
	}
}

func TestSearchRanksTheWholeWordAboveLongerWords(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "med.zim")
	insertArticle(t, db, src, "/lymecycline", "Lymecycline", "Lymecycline is an antibiotic. Lymecycline treats acne.")
	insertArticle(t, db, src, "/lyme", "Tick borne illness", "Lyme disease is carried by ticks.")

	if got := paths(search(t, db, "lyme", KeywordOptions{}).Results); !slices.Equal(got, []string{"/lyme", "/lymecycline"}) {
		t.Fatalf("results = %v, want the article with the word itself first, the longer word still found", got)
	}
}

func TestSearchMatchesSingularForPlural(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "med.zim")
	insertArticle(t, db, src, "/dose", "Dose", "Take one tablet every four hours.")

	if got := paths(search(t, db, "tablets", KeywordOptions{}).Results); !slices.Equal(got, []string{"/dose"}) {
		t.Fatalf("results = %v, want the article that says \"tablet\"", got)
	}
}

func TestSearchIgnoresStopWordsAndPunctuation(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "med.zim")
	insertArticle(t, db, src, "/dose", "Paracetamol", "One dose is two tablets.")

	matches := search(t, db, "What is the dose of paracetamol?", KeywordOptions{})
	if matches.Exact != 1 {
		t.Fatalf("exact = %d, want the article found as an all-words match although it lacks \"what\"", matches.Exact)
	}
	for _, query := range []string{"", "?? --", `"`, `a "b`} {
		if _, err := New(db).Search(context.Background(), query, KeywordOptions{Limit: 5}); err != nil {
			t.Errorf("Search(%q) error = %v", query, err)
		}
	}
}

func TestSearchRestrictsToSource(t *testing.T) {
	db := setupTestDB(t)
	big := insertSource(t, db, "big.zim")
	small := insertSource(t, db, "small.zim")
	for i := 0; i < 30; i++ {
		insertArticle(t, db, big, "/big"+strings.Repeat("x", i), "Water water water", "water water water")
	}
	insertArticle(t, db, small, "/small", "Rain", "collecting water from a roof")

	matches := search(t, db, "water", KeywordOptions{Source: "small.zim", Limit: 5})
	if got := paths(matches.Results); !slices.Equal(got, []string{"/small"}) {
		t.Fatalf("results = %v, want the one article of the named source however it ranks overall", got)
	}
}

func TestSearchReturnsCancellation(t *testing.T) {
	db := setupTestDB(t)
	insertArticle(t, db, insertSource(t, db, "med.zim"), "/a", "Water", "water")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(db).Search(ctx, "water filter", KeywordOptions{Limit: 5}); err == nil {
		t.Fatal("Search() with a cancelled context returned no error")
	}
}

func TestNearPaths(t *testing.T) {
	db := setupTestDB(t)
	src := insertSource(t, db, "site.zim")
	insertArticle(t, db, src, "/www.example.org/medicines/paracetamol-for-adults/", "Paracetamol for adults", "x")
	insertArticle(t, db, src, "/www.example.org/medicines/paracetamol-for-children/", "Paracetamol for children", "x")
	insertArticle(t, db, src, "/www.example.org/medicines/ibuprofen/about/", "About ibuprofen", "x")
	insertArticle(t, db, src, "/www.example.org/medicines/codeine/about/", "About codeine", "x")
	other := insertSource(t, db, "other.zim")
	insertArticle(t, db, other, "/paracetamol-for-adults/", "Elsewhere", "x")
	e := New(db)

	for _, tc := range []struct {
		name       string
		candidates []string
		segment    string
		want       string
	}{
		{"case and leading slash", []string{"WWW.example.org/Medicines/Paracetamol-For-Adults/"}, "Paracetamol-For-Adults", "/www.example.org/medicines/paracetamol-for-adults/"},
		{"second candidate", []string{"paracetamol-for-adults", "www.example.org/medicines/paracetamol-for-adults/"}, "paracetamol-for-adults", "/www.example.org/medicines/paracetamol-for-adults/"},
		{"only path with this ending", []string{"paracetamol-for-adults/"}, "paracetamol-for-adults", "/www.example.org/medicines/paracetamol-for-adults/"},
		{"ending shared by two paths", []string{"about/"}, "about", ""},
		{"no such path", []string{"paracetamol"}, "paracetamol", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match, near, err := e.NearPaths(context.Background(), "site.zim", tc.candidates, tc.segment, 5)
			if err != nil {
				t.Fatal(err)
			}
			if match.Path != tc.want {
				t.Errorf("match = %q, want %q", match.Path, tc.want)
			}
			if len(near) == 0 {
				t.Error("no near paths offered")
			}
			for _, found := range near {
				if found.Path == "/paracetamol-for-adults/" {
					t.Error("near paths include another source's article")
				}
			}
		})
	}

	_, near, err := e.NearPaths(context.Background(), "site.zim", []string{"paracetamol"}, "paracetamol", 1)
	if err != nil || len(near) != 1 {
		t.Fatalf("near = %v (%v), want the limit of one path", near, err)
	}
	if _, near, _ := e.NearPaths(context.Background(), "site.zim", []string{"100%_real"}, "100%_real", 5); len(near) != 0 {
		t.Errorf("near = %v: %% and _ in a path must not act as wildcards", near)
	}
}

func TestSourceFilenames(t *testing.T) {
	db := setupTestDB(t)
	insertSource(t, db, "wiki.zim")
	insertSource(t, db, "med.zim")
	got, err := New(db).SourceFilenames(context.Background())
	if err != nil || !slices.Equal(got, []string{"med.zim", "wiki.zim"}) {
		t.Fatalf("SourceFilenames() = %v, %v", got, err)
	}
}
