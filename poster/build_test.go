package main

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestBuildSite_RendersCrawlablePages(t *testing.T) {
	t.Chdir(t.TempDir())
	testPostDirs(t)
	testWrite(t, "blog/index.json", `[
  {"id":2,"dir":"002-newer","title":"newer <&>","date":"2026-01-02T00:00:00Z","tags":["Deep Tech"],"excerpt":"newer excerpt","image":"image.webp"},
  {"id":7,"dir":"007-older","title":"older","date":"2026-01-01T00:00:00Z","tags":[],"excerpt":"","image":""}
]`)
	testWrite(t, "blog/all.json", `[
  {"id":2,"dir":"002-newer","title":"newer <&>","date":"2026-01-02T00:00:00Z","tags":["Deep Tech"],"author":"","hex":"","quote":"","image":"image.webp","body":"# Heading\n\nline<br>next <script>alert(1)</script>\n\n[out](https://example.com/) [in](/shrine.html)\n\n| a | b |\n|---|---|\n| 1 | 2 |"},
  {"id":7,"dir":"007-older","title":"older","date":"2026-01-01T00:00:00Z","tags":[],"author":"guest","hex":"","quote":"","image":"","body":"older body"}
]`)

	changed, err := buildSite()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"blog/002-newer/index.html", "blog/007-older/index.html", "blog/index.html", "feed.xml", "llms.txt", "sitemap.xml"}
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("changed = %v, want %v", changed, want)
	}
	if changed, err := buildSite(); err != nil || len(changed) != 0 {
		t.Fatalf("second build changed %v, %v", changed, err)
	}

	page := string(testRead(t, "blog/002-newer/index.html"))
	for _, s := range []string{
		`<title>newer &lt;&amp;&gt; :: requiem's weblog</title>`,
		`<link rel="canonical" href="https://requiem.moe/blog/002-newer/" />`,
		`<meta property="og:image" content="https://requiem.moe/blog/002-newer/image.webp" />`,
		`<meta name="description" content="newer excerpt" />`,
		`<meta property="article:tag" content="Deep Tech" />`,
		`<span class="entry-tag post-tags">#deep-tech</span>`,
		`<h1 class="post-title">newer &lt;&amp;&gt;</h1>`,
		`<h1>Heading</h1>`,
		`line<br>next`,
		`<a href="https://example.com/" rel="noreferrer noopener" target="_blank">out</a>`,
		`<a href="/shrine.html">in</a>`,
		`<td>1</td>`,
		`<a class="panel-link" href="../007-older/" rel="next">older :: older</a>`,
		`href="../../#page-1"`,
	} {
		if !strings.Contains(page, s) {
			t.Errorf("newer page is missing %q", s)
		}
	}
	if strings.Contains(page, "alert(1)") || strings.Contains(page, `rel="prev"`) {
		t.Error("newer page kept a script or links to a newer post")
	}

	m := regexp.MustCompile(`<script type="application/ld\+json">(.*)</script>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("newer page has no JSON-LD")
	}
	var ld struct {
		Type     string            `json:"@type"`
		Headline string            `json:"headline"`
		Date     string            `json:"datePublished"`
		Author   map[string]string `json:"author"`
		Keywords []string          `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(m[1]), &ld); err != nil {
		t.Fatalf("JSON-LD: %v: %s", err, m[1])
	}
	if ld.Type != "BlogPosting" || ld.Headline != "newer <&>" || ld.Date != "2026-01-02T00:00:00Z" ||
		ld.Author["@id"] != "https://requiem.moe/#requiem" || !reflect.DeepEqual(ld.Keywords, []string{"Deep Tech"}) {
		t.Errorf("unexpected JSON-LD: %+v", ld)
	}

	older := string(testRead(t, "blog/007-older/index.html"))
	for _, s := range []string{
		`<meta name="description" content="older" />`,
		`<meta property="og:image" content="https://requiem.moe/assets/og.jpg" />`,
		`"author":{"@type":"Person","name":"guest"}`,
		`rel="prev">newer :: newer &lt;&amp;&gt;</a>`,
	} {
		if !strings.Contains(older, s) {
			t.Errorf("older page is missing %q", s)
		}
	}
	if strings.Contains(older, "post-frame") || strings.Contains(older, "post-tags") {
		t.Error("older page renders an image or tags it does not have")
	}

	var sitemap struct {
		Locs []string `xml:"url>loc"`
	}
	if err := xml.Unmarshal(testRead(t, "sitemap.xml"), &sitemap); err != nil {
		t.Fatal(err)
	}
	wantLocs := []string{"https://requiem.moe/", "https://requiem.moe/shrine.html", "https://requiem.moe/blog/", "https://requiem.moe/blog/002-newer/", "https://requiem.moe/blog/007-older/"}
	if !reflect.DeepEqual(sitemap.Locs, wantLocs) {
		t.Errorf("sitemap locs = %v", sitemap.Locs)
	}

	var feed struct {
		Items []struct {
			Link    string `xml:"link"`
			Creator string `xml:"http://purl.org/dc/elements/1.1/ creator"`
			Content string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(testRead(t, "feed.xml"), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 2 || feed.Items[0].Link != "https://requiem.moe/blog/002-newer/" || feed.Items[0].Creator != "requiem" ||
		!strings.Contains(feed.Items[0].Content, "<h1>Heading</h1>") || strings.Contains(feed.Items[0].Content, "alert(1)") {
		t.Errorf("unexpected feed items: %+v", feed.Items)
	}

	if llms := string(testRead(t, "llms.txt")); !strings.Contains(llms, "- [newer <&>](https://requiem.moe/blog/002-newer/): newer excerpt\n- [older](https://requiem.moe/blog/007-older/): older\n") {
		t.Errorf("llms.txt post list: %s", llms)
	}
	if archive := string(testRead(t, "blog/index.html")); !strings.Contains(archive, `<a class="panel-link" href="./002-newer/">newer &lt;&amp;&gt;</a>`) || !strings.Contains(archive, "2 entries") {
		t.Errorf("archive page: %s", archive)
	}
}

func TestBuildSite_RejectsBadPosts(t *testing.T) {
	for _, tc := range []struct{ name, archive string }{
		{"escaping dir", `[{"id":1,"dir":"../x","date":"2026-01-01T00:00:00Z"}]`},
		{"empty dir", `[{"id":1,"dir":"","date":"2026-01-01T00:00:00Z"}]`},
		{"bad date", `[{"id":1,"dir":"001-x","date":"yesterday"}]`},
		{"not json", `[{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.MkdirAll("blog/001-x", 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, "blog/index.json", "[]")
			testWrite(t, "blog/all.json", tc.archive)
			if changed, err := buildSite(); err == nil || len(changed) != 0 {
				t.Errorf("buildSite = %v, %v; want an error and no writes", changed, err)
			}
		})
	}
}

func testRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
