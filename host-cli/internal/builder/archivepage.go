package builder

import (
	"bytes"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
)

const (
	archiveStylesheet     = "archive.css"
	archiveThumbnailName  = "thumb.jpg"
	archiveThumbnailWidth = 360
	archiveImageKeepBytes = 250 << 10
	archiveImageMaxSide   = 1400
)

const archiveCSS = `:root{color-scheme:light dark;--bg:#fbfaf7;--fg:#1f2328;--muted:#6b7280;--line:#e2ded5;--accent:#274057;--card:#fff}
@media (prefers-color-scheme:dark){:root{--bg:#15181c;--fg:#e6e3dc;--muted:#9aa3ad;--line:#2b3138;--accent:#9ec1e0;--card:#1d2126}}
*{box-sizing:border-box}
body{margin:0;padding:0 0 4rem;background:var(--bg);color:var(--fg);font:17px/1.6 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;overflow-wrap:break-word}
body>header,body>main,body>section{max-width:46rem;margin:0 auto;padding:0 1.1rem}
body.index>header,body.index>main{max-width:76rem}
body>header{padding-top:1.4rem;padding-bottom:.6rem;margin-bottom:1.2rem;border-bottom:1px solid var(--line)}
h1{font-size:1.9rem;line-height:1.2;margin:.3rem 0 .5rem}
h2{font-size:1.3rem;line-height:1.3;margin:2rem 0 .6rem}
h3{font-size:1.1rem;margin:1.5rem 0 .5rem}
a{color:var(--accent)}
.back,.byline,.source{margin:.2rem 0;color:var(--muted);font-size:.85rem}
.back a{text-decoration:none;font-weight:600}
.lead{font-size:1.05rem;color:var(--muted)}
main img{max-width:100%;height:auto;display:block;margin:1rem 0;border-radius:6px}
main video{display:block;width:100%;max-height:75vh;margin:1rem 0;border-radius:6px;background:#000}
pre{overflow:auto;background:var(--card);border:1px solid var(--line);padding:.8rem;border-radius:6px;font-size:.85em}
code{font-size:.9em}
table{border-collapse:collapse;display:block;overflow:auto;margin:1rem 0}
td,th{border:1px solid var(--line);padding:.3rem .6rem;vertical-align:top}
blockquote{margin:1rem 0;padding:.1rem 1rem;border-left:3px solid var(--line);color:var(--muted)}
.files{margin-top:2.5rem;padding:1rem 1.1rem;background:var(--card);border:1px solid var(--line);border-radius:8px}
.files h2{margin-top:0}
.files .download{font-weight:600}
.files .size{color:var(--muted);font-size:.8rem}
.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(11rem,1fr));gap:.9rem}
.card{display:flex;flex-direction:column;background:var(--card);border:1px solid var(--line);border-radius:8px;overflow:hidden;text-decoration:none;color:inherit}
.card img,.card .thumb{display:block;width:100%;aspect-ratio:4/3;object-fit:cover;margin:0;border-radius:0;background:var(--line)}
.card .title{padding:.5rem .6rem 0;font-weight:600;font-size:.9rem;line-height:1.3}
.card .host{margin-top:auto;padding:.3rem .6rem .6rem;color:var(--muted);font-size:.75rem}
.card.unavailable{opacity:.55}
[hidden]{display:none!important}
.searchbar{position:sticky;top:0;z-index:1;display:flex;align-items:center;gap:.8rem;max-width:76rem;margin:0 auto;padding:.6rem 1.1rem;background:var(--bg);border-bottom:1px solid var(--line)}
.searchbar input{flex:1;font:inherit;font-size:1rem;padding:.5rem .8rem;border:1px solid var(--line);border-radius:8px;background:var(--card);color:var(--fg)}
.searchbar input:focus{outline:2px solid var(--accent);outline-offset:-1px}
#count{color:var(--muted);font-size:.85rem;white-space:nowrap}
#nothing{color:var(--muted);margin:2rem 0}
.contents{display:flex;flex-wrap:wrap;gap:.3rem .5rem;margin:.8rem 0 .4rem}
.contents a{padding:.15rem .6rem;border:1px solid var(--line);border-radius:999px;background:var(--card);font-size:.8rem;text-decoration:none}
body.index h2{margin-top:2.6rem;padding-bottom:.3rem;border-bottom:1px solid var(--line);letter-spacing:.04em}
body.index h3{color:var(--muted);font-size:.85rem;letter-spacing:.08em;margin:1.4rem 0 .6rem}
`

func archivePageHead(page *bytes.Buffer, title, stylesheet, class string) {
	fmt.Fprintf(page, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>%s</title><link rel="stylesheet" href="%s"></head><body class="%s">`,
		template.HTMLEscapeString(title), template.HTMLEscapeString(stylesheet), class)
}

func renderArchiveArticle(article *archiveArticle, source archiveSource, files string, site archiveSiteInfo, entry string) ([]byte, error) {
	var page bytes.Buffer
	title := article.Title
	if title == "" {
		title = source.Label
	}
	if title == "" {
		title = source.URL.String()
	}
	archivePageHead(&page, title, archiveRelative(entry, site.stylesheet), "project")
	fmt.Fprintf(&page, `<header><p class="back"><a href="%s">%s</a>`, template.HTMLEscapeString(archiveRelative(entry, site.home)), template.HTMLEscapeString(site.title))
	if source.Category != "" {
		fmt.Fprintf(&page, ` · %s`, template.HTMLEscapeString(source.Category))
	}
	fmt.Fprintf(&page, `</p><h1>%s</h1>`, template.HTMLEscapeString(title))
	author := article.Author
	if author == "" {
		author = source.Author
	}
	byline := strings.TrimPrefix(source.URL.Host, "www.")
	if author != "" {
		byline = "by " + author + " · " + byline
	}
	if source.Page > 0 {
		byline += fmt.Sprintf(" · catalog page %d", source.Page)
	}
	if article.License != "" {
		byline += " · " + article.License
	}
	if source.Label != "" && source.Label != title {
		fmt.Fprintf(&page, `<p class="lead">%s</p>`, template.HTMLEscapeString(source.Label))
	}
	fmt.Fprintf(&page, `<p class="byline">%s</p><p class="source">Source: %s`, template.HTMLEscapeString(byline), template.HTMLEscapeString(source.URL.String()))
	if article.Wayback != "" {
		fmt.Fprintf(&page, " · Wayback Machine copy from %s", archiveWaybackDate(article.Wayback))
	}
	page.WriteString("</p></header><main>")
	if article.Description != "" && len(strings.TrimSpace(archiveNodeText(article.Content))) < 300 {
		fmt.Fprintf(&page, `<p class="lead">%s</p>`, template.HTMLEscapeString(article.Description))
	}
	for child := article.Content.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&page, child); err != nil {
			return nil, err
		}
	}
	page.WriteString(files)
	page.WriteString(`</main></body></html>`)
	return page.Bytes(), nil
}

// writeArchiveThumbnail scales the project's first real picture for the index.
func writeArchiveThumbnail(project string, pages []archivedPage) string {
	for _, page := range pages {
		if page.HTML || !archiveImageExtension(strings.ToLower(filepath.Ext(page.Path))) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(page.Path)))
		if err != nil {
			continue
		}
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < 200 || config.Height < 150 || config.Width*config.Height > 60_000_000 {
			continue
		}
		source, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		height := max(1, config.Height*archiveThumbnailWidth/config.Width)
		scaled := image.NewRGBA(image.Rect(0, 0, archiveThumbnailWidth, height))
		draw.Draw(scaled, scaled.Bounds(), image.White, image.Point{}, draw.Src)
		draw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Over, nil)
		var encoded bytes.Buffer
		if err := jpeg.Encode(&encoded, scaled, &jpeg.Options{Quality: 78}); err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(project, archiveThumbnailName), encoded.Bytes(), 0o644); err != nil {
			continue
		}
		return archiveThumbnailName
	}
	return ""
}

func writeArchiveIndex(path, root string, sources []archiveSource, title, origin string) error {
	var cards, contents bytes.Buffer
	archived := 0
	lastTop, lastGroup := "", ""
	openTop, openGroup := false, false
	for index, source := range sources {
		local := filepath.ToSlash(filepath.Join("projects", source.ID, archiveEntryPath(source.URL)))
		host := strings.TrimPrefix(source.URL.Host, "www.")
		name, class, thumbnail, pageTitle := source.Label, "card", "", ""
		if project, err := readArchiveProject(filepath.Join(root, "projects", source.ID, "project.json")); err == nil {
			pageTitle = project.Title
			if name == "" {
				name = project.Title
			}
			if project.Status == "unavailable" {
				class = "card unavailable"
				host += " · unavailable"
			} else {
				archived++
			}
			if project.Thumbnail != "" {
				thumbnail = filepath.ToSlash(filepath.Join("projects", source.ID, project.Thumbnail))
			}
		}
		if name == "" {
			name = source.URL.String()
		}
		group := source.Category
		if group == "" && source.Page > 0 {
			group = fmt.Sprintf("Page %d", source.Page)
		}
		top, sub, _ := strings.Cut(group, " / ")
		if index == 0 || group != lastGroup {
			if openGroup {
				cards.WriteString("</div></section>")
			}
			if index == 0 || top != lastTop {
				if openTop {
					cards.WriteString("</section>")
				}
				cards.WriteString(`<section class="top">`)
				openTop = true
				if top != "" {
					anchor := fmt.Sprintf("section-%d", index)
					fmt.Fprintf(&cards, `<h2 id="%s">%s</h2>`, anchor, template.HTMLEscapeString(top))
					fmt.Fprintf(&contents, `<a href="#%s">%s</a>`, anchor, template.HTMLEscapeString(top))
				}
			}
			cards.WriteString(`<section class="group">`)
			if sub != "" {
				fmt.Fprintf(&cards, "<h3>%s</h3>", template.HTMLEscapeString(sub))
			}
			cards.WriteString(`<div class="cards">`)
			lastTop, lastGroup, openGroup = top, group, true
		}
		if source.Author != "" {
			host = "by " + source.Author + " · " + host
		}
		search := strings.ToLower(strings.Join([]string{name, pageTitle, host, group}, " "))
		fmt.Fprintf(&cards, `<a class="%s" href="%s" data-search="%s">`, class, archiveIndexHref(local), template.HTMLEscapeString(search))
		if thumbnail != "" {
			fmt.Fprintf(&cards, `<img loading="lazy" alt="" src="%s">`, archiveIndexHref(thumbnail))
		} else {
			cards.WriteString(`<span class="thumb"></span>`)
		}
		fmt.Fprintf(&cards, `<span class="title">%s</span><span class="host">%s</span></a>`,
			template.HTMLEscapeString(name), template.HTMLEscapeString(host))
	}
	if openGroup {
		cards.WriteString("</div></section>")
	}
	if openTop {
		cards.WriteString("</section>")
	}
	var page bytes.Buffer
	archivePageHead(&page, title, archiveStylesheet, "index")
	fmt.Fprintf(&page, `<header><h1>%s</h1><p class="byline">%d of %d projects archived from %s</p>`,
		template.HTMLEscapeString(title), archived, len(sources), template.HTMLEscapeString(origin))
	if contents.Len() > 0 {
		fmt.Fprintf(&page, `<p class="contents">%s</p>`, contents.Bytes())
	}
	page.WriteString(`</header><div class="searchbar"><input id="search" type="search" placeholder="Filter projects: words or parts of words, e.g. lamp wood" autocomplete="off" aria-label="Filter projects"><span id="count"></span></div><main>`)
	page.Write(cards.Bytes())
	page.WriteString(`<p id="nothing" hidden>No projects match. Kiwix's own search covers the full text of every page.</p></main>`)
	page.WriteString(archiveIndexScript)
	page.WriteString("</body></html>")
	return os.WriteFile(path, page.Bytes(), 0o644)
}

// archiveIndexScript filters the index cards as the reader types: every word
// must appear, as a substring, in the card's title, author, site or category.
// #q=... in the address fills the box, so a filter can be bookmarked.
const archiveIndexScript = `<script>
(function () {
  var box = document.getElementById("search"), count = document.getElementById("count");
  var fold = function (text) { return text.toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, ""); };
  var cards = Array.prototype.map.call(document.querySelectorAll(".card"), function (card) {
    return { node: card, text: fold(card.getAttribute("data-search") || "") };
  });
  var sections = document.querySelectorAll(".group, .top");
  function run() {
    var terms = fold(box.value).split(/\s+/).filter(Boolean), shown = 0;
    cards.forEach(function (card) {
      var match = terms.every(function (term) { return card.text.indexOf(term) >= 0; });
      card.node.hidden = !match;
      if (match) shown++;
    });
    Array.prototype.forEach.call(sections, function (section) {
      section.hidden = !section.querySelector(".card:not([hidden])");
    });
    count.textContent = terms.length ? shown + " of " + cards.length : "";
    document.getElementById("nothing").hidden = shown > 0;
    try { history.replaceState(null, "", terms.length ? "#q=" + encodeURIComponent(box.value) : location.pathname + location.search); } catch (error) {}
  }
  box.addEventListener("input", run);
  box.addEventListener("keydown", function (event) { if (event.key === "Escape") { box.value = ""; run(); } });
  var query = location.hash.match(/^#q=(.*)$/);
  if (query) { box.value = decodeURIComponent(query[1]); run(); }
})();
</script>`

// archiveOptimizeImage shrinks large photos; it reports false when the
// original bytes should be kept.
func archiveOptimizeImage(body []byte) ([]byte, bool) {
	if len(body) < archiveImageKeepBytes {
		return nil, false
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || format == "gif" || config.Width*config.Height > 60_000_000 {
		return nil, false
	}
	source, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, false
	}
	if opaque, ok := source.(interface{ Opaque() bool }); format != "jpeg" && (!ok || !opaque.Opaque()) {
		return nil, false
	}
	width, height := config.Width, config.Height
	if longest := max(width, height); longest > archiveImageMaxSide {
		width, height = max(1, width*archiveImageMaxSide/longest), max(1, height*archiveImageMaxSide/longest)
	}
	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, scaled, &jpeg.Options{Quality: 82}); err != nil || encoded.Len() >= len(body) {
		return nil, false
	}
	return encoded.Bytes(), true
}
