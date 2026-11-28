package frontpage

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestAdaptArticleReadsStackExchangeQuestion(t *testing.T) {
	page := `<html lang="en"><head><title>Q - Gardening</title><script src="x.js"></script></head><body>
<div id="left-sidebar"><a href="../../questions">Questions</a></div>
<div id="content"><div id="question-header"><h1><a class="question-hyperlink" href="#">Do tomatoes need support?</a></h1></div>
<div id="mainbar"><div id="question" class="question"><div class="js-vote-count">12</div>
<div class="s-prose js-post-body"><p>They sprawl. See <a href="../19000/staking">staking</a>.</p></div>
<div class="post-taglist"><a href="../../questions/tagged/tomatoes">tomatoes</a><a href="../../questions/tagged/support">support</a></div>
<div class="comments"><div class="comment-body">nice</div></div></div>
<div id="answers"><div id="answer-1" class="answer accepted-answer"><div class="js-vote-count">9</div><div class="s-prose js-post-body"><p>Yes, mostly.</p></div></div>
<div id="answer-2" class="answer"><div class="js-vote-count">-1</div><div class="s-prose js-post-body"><p>No.</p></div></div></div>
</div></div></body></html>`
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	root, _ := url.Parse("http://127.0.0.1:1/content/garden/")
	at, _ := url.Parse("http://127.0.0.1:1/content/garden/questions/1/q")
	got := adaptArticle(doc, at, root)
	data, _ := json.Marshal(got)
	text := string(data)
	for _, want := range []string{`"title":"Do tomatoes need support?"`, "Question · 12 votes · tomatoes, support", "2 answers", "Answer · 9 votes · Accepted", "Answer · -1 votes", `"path":"questions/19000/staking"`} {
		if !strings.Contains(text, want) {
			t.Errorf("adapted question lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "nice") || strings.Contains(text, "Questions") {
		t.Errorf("comments or sidebar leaked into the article: %s", text)
	}
}
