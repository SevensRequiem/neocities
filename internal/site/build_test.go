package site

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestBuildSite_RendersCrawlablePages(t *testing.T) {
	t.Chdir(t.TempDir())
	testPostDirs(t)
	testSeed(t)
	testWrite(t, "blog/index.json", `[
  {"id":2,"dir":"002-newer","title":"newer <&>","date":"2026-01-02T00:00:00Z","tags":["Deep Tech"],"excerpt":"newer excerpt","image":"image.webp"},
  {"id":7,"dir":"007-older","title":"older","date":"2026-01-01T00:00:00Z","tags":[],"excerpt":"","image":""}
]`)
	testWrite(t, "blog/all.json", `[
  {"id":2,"dir":"002-newer","title":"newer <&>","date":"2026-01-02T00:00:00Z","tags":["Deep Tech"],"author":"","hex":"","quote":"","image":"image.webp","body":"# Heading\n\nline<br>next <script>alert(1)</script>\n\n[out](https://example.com/) [in](/shrine.html)\n\n| a | b |\n|---|---|\n| 1 | 2 |"},
  {"id":7,"dir":"007-older","title":"older","date":"2026-01-01T00:00:00Z","tags":[],"author":"guest","hex":"","quote":"","image":"","body":"older body"}
]`)

	changed, err := BuildSite()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"blog/002-newer/index.html", "blog/007-older/index.html", "blog/index.html", "feed.xml", "llms.txt", "sitemap.xml", "stories.xml", "stories/index.html"}
	if !reflect.DeepEqual(changed, want) {
		t.Fatalf("changed = %v, want %v", changed, want)
	}
	if changed, err := BuildSite(); err != nil || len(changed) != 0 {
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
		`<a class="panel-link pager-next" href="../007-older/" rel="next">older :: older</a>`,
		`href="../../#entry-2"`,
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
	wantLocs := []string{"https://requiem.moe/", "https://requiem.moe/shrine.html", "https://requiem.moe/blog/", "https://requiem.moe/stories/", "https://requiem.moe/blog/002-newer/", "https://requiem.moe/blog/007-older/"}
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

	llms := string(testRead(t, "llms.txt"))
	if !strings.Contains(llms, "- [newer <&>](https://requiem.moe/blog/002-newer/): newer excerpt\n- [older](https://requiem.moe/blog/007-older/): older\n") || strings.Contains(llms, "## Stories") {
		t.Errorf("llms.txt post list: %s", llms)
	}
	if archive := string(testRead(t, "blog/index.html")); !strings.Contains(archive, `<a class="panel-link" href="./002-newer/">newer &lt;&amp;&gt;</a>`) || !strings.Contains(archive, "2 entries") || !strings.Contains(archive, `href="../stories/">stories</a>`) {
		t.Errorf("archive page: %s", archive)
	}
	if list := string(testRead(t, "stories/index.html")); !strings.Contains(list, "no stories yet.") || !strings.Contains(list, "0 stories") || !strings.Contains(list, `href="../stories.xml"`) {
		t.Errorf("empty story list: %s", list)
	}
}

func TestBuildSite_RendersStoriesAndMemos(t *testing.T) {
	t.Chdir(t.TempDir())
	testPostDirs(t)
	testSeed(t)
	for _, dir := range []string{"stories/010-newest", "stories/004-oldest"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	testWrite(t, "blog/index.json", `[{"id":2,"dir":"002-newer","excerpt":"e2"},{"id":7,"dir":"007-older","excerpt":"e7"}]`)
	testWrite(t, "blog/all.json", `[
  {"id":2,"dir":"002-newer","title":"p2","date":"2026-01-02T00:00:00Z","tags":[],"body":"b"},
  {"id":7,"dir":"007-older","title":"p7","date":"2026-01-01T00:00:00Z","tags":[],"body":"b"}
]`)
	testWrite(t, "stories/all.json", `[
  {"id":10,"dir":"010-newest","title":"Night <Rain>","date":"2026-01-03T00:00:00Z","updated":"2026-02-01T10:00:00Z","tags":["Sci Fi"],"author":"","hex":"","summary":"a summary","image":"image.webp","body":"First scene has five words.\n\n---\n\nSecond [scene](https://example.com/) <script>alert(1)</script>"},
  {"id":4,"dir":"004-oldest","title":"Old","date":"2026-01-01T12:00:00Z","tags":[],"author":"guest","hex":"","summary":"","image":"","body":"tiny"}
]`)
	testWrite(t, "memos/index.json", `[
  {"id":2,"date":"2026-01-02T05:00:00Z","body":"**bold** <script>alert(1)</script> [x](https://example.com/)","html":"stale"},
  {"id":1,"date":"2026-01-01T05:00:00Z","body":"plain","html":""}
]`)

	changed, err := BuildSite()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"memos/index.json", "stories/010-newest/index.html", "stories/004-oldest/index.html", "stories/index.html", "stories.xml"} {
		if !slices.Contains(changed, path) {
			t.Errorf("changed %v misses %s", changed, path)
		}
	}
	if changed, err := BuildSite(); err != nil || len(changed) != 0 {
		t.Fatalf("second build changed %v, %v", changed, err)
	}

	memos := testReadJSON[[]Memo](t, "memos/index.json")
	if len(memos) != 2 || memos[0].HTML != "<p><strong>bold</strong>  <a href=\"https://example.com/\" rel=\"noreferrer noopener\" target=\"_blank\">x</a></p>\n" || memos[1].HTML != "<p>plain</p>\n" || memos[0].Body != "**bold** <script>alert(1)</script> [x](https://example.com/)" {
		t.Errorf("memos = %+v", memos)
	}

	page := string(testRead(t, "stories/010-newest/index.html"))
	for _, s := range []string{
		`<title>Night &lt;Rain&gt; :: requiem's stories</title>`,
		`<link rel="canonical" href="https://requiem.moe/stories/010-newest/" />`,
		`<meta property="og:image" content="https://requiem.moe/stories/010-newest/image.webp" />`,
		`<meta name="description" content="a summary" />`,
		`<meta property="article:modified_time" content="2026-02-01T10:00:00Z" />`,
		`<span class="panel-title">::STORY.010</span>`,
		`<span class="entry-tag story-meta">7 words :: 1 min read :: updated 2026.02.01</span>`,
		`<span class="entry-tag post-tags">#sci-fi</span>`,
		`<div class="post-body story-body"><p>First scene has five words.</p>` + "\n<hr>\n",
		`<a class="panel-link pager-next" href="../004-oldest/" rel="next">older :: Old</a>`,
		`<a class="panel-link" href="../">stories</a>`,
	} {
		if !strings.Contains(page, s) {
			t.Errorf("story page is missing %q", s)
		}
	}
	if strings.Contains(page, "alert(1)") || strings.Contains(page, `rel="prev"`) {
		t.Error("story page kept a script or links to a newer story")
	}
	m := regexp.MustCompile(`<script type="application/ld\+json">(.*)</script>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("story page has no JSON-LD")
	}
	var ld struct {
		Type     string `json:"@type"`
		Name     string `json:"name"`
		Words    int    `json:"wordCount"`
		Time     string `json:"timeRequired"`
		Modified string `json:"dateModified"`
	}
	if err := json.Unmarshal([]byte(m[1]), &ld); err != nil {
		t.Fatal(err)
	}
	if ld != (struct {
		Type     string `json:"@type"`
		Name     string `json:"name"`
		Words    int    `json:"wordCount"`
		Time     string `json:"timeRequired"`
		Modified string `json:"dateModified"`
	}{"ShortStory", "Night <Rain>", 7, "PT1M", "2026-02-01T10:00:00Z"}) {
		t.Errorf("story JSON-LD = %+v", ld)
	}
	older := string(testRead(t, "stories/004-oldest/index.html"))
	if !strings.Contains(older, `rel="prev">newer :: Night &lt;Rain&gt;</a>`) || strings.Contains(older, "modified_time") || strings.Contains(older, "post-frame") || !strings.Contains(older, `"author":{"@type":"Person","name":"guest"}`) {
		t.Errorf("older story page: %s", older)
	}

	list := string(testRead(t, "stories/index.html"))
	for _, s := range []string{"2 stories", `<a class="panel-link" href="./010-newest/">Night &lt;Rain&gt;</a>`, `2026.01.03</time> :: 7 words :: 1 min read`, `<p class="story-summary">a summary</p>`, `href="./004-oldest/"`} {
		if !strings.Contains(list, s) {
			t.Errorf("story list is missing %q", s)
		}
	}

	var feed struct {
		Items []struct {
			Link     string   `xml:"link"`
			Category []string `xml:"category"`
			Content  string   `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(testRead(t, "feed.xml"), &feed); err != nil {
		t.Fatal(err)
	}
	var links []string
	for _, item := range feed.Items {
		links = append(links, item.Link)
	}
	wantLinks := []string{"https://requiem.moe/stories/010-newest/", "https://requiem.moe/blog/002-newer/", "https://requiem.moe/stories/004-oldest/", "https://requiem.moe/blog/007-older/"}
	if !reflect.DeepEqual(links, wantLinks) || !reflect.DeepEqual(feed.Items[0].Category, []string{"story", "Sci Fi"}) || !reflect.DeepEqual(feed.Items[2].Category, []string{"story"}) || len(feed.Items[1].Category) != 0 {
		t.Errorf("feed = %+v", feed.Items)
	}
	feed.Items = nil
	if err := xml.Unmarshal(testRead(t, "stories.xml"), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 2 || feed.Items[0].Link != "https://requiem.moe/stories/010-newest/" || !strings.Contains(feed.Items[0].Content, "<hr>") || strings.Contains(feed.Items[0].Content, "alert(1)") {
		t.Errorf("stories.xml = %+v", feed.Items)
	}

	var sitemap struct {
		URLs []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(testRead(t, "sitemap.xml"), &sitemap); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, u := range sitemap.URLs {
		got[u.Loc] = u.LastMod
	}
	if len(sitemap.URLs) != 8 || got["https://requiem.moe/stories/"] != "2026-02-01T10:00:00Z" || got["https://requiem.moe/stories/010-newest/"] != "2026-02-01T10:00:00Z" || got["https://requiem.moe/stories/004-oldest/"] != "2026-01-01T12:00:00Z" {
		t.Errorf("sitemap = %+v", sitemap.URLs)
	}
	if llms := string(testRead(t, "llms.txt")); !strings.Contains(llms, "\n## Stories\n\n- [Night <Rain>](https://requiem.moe/stories/010-newest/): a summary (7 words)\n- [Old](https://requiem.moe/stories/004-oldest/): Old (1 words)\n") {
		t.Errorf("llms.txt: %s", llms)
	}
}

func TestBuildSite_FeedMergeKeepsNewestTwenty(t *testing.T) {
	t.Chdir(t.TempDir())
	testSeed(t)
	var posts, stories []string
	for day := 30; day >= 1; day-- {
		dir := fmt.Sprintf("%03d-d", day)
		section, list := "blog", &posts
		if day%3 == 0 {
			section, list = "stories", &stories
		}
		if err := os.MkdirAll(section+"/"+dir, 0o755); err != nil {
			t.Fatal(err)
		}
		*list = append(*list, fmt.Sprintf(`{"id":%d,"dir":%q,"title":"t","date":"2026-01-%02dT00:00:00Z","tags":[],"body":"b"}`, day, dir, day))
	}
	testWrite(t, "blog/all.json", "["+strings.Join(posts, ",")+"]")
	testWrite(t, "blog/index.json", "[]")
	testWrite(t, "stories/all.json", "["+strings.Join(stories, ",")+"]")
	if _, err := BuildSite(); err != nil {
		t.Fatal(err)
	}
	var feed struct {
		Links []string `xml:"channel>item>link"`
	}
	if err := xml.Unmarshal(testRead(t, "feed.xml"), &feed); err != nil {
		t.Fatal(err)
	}
	var want []string
	for day := 30; day > 10; day-- {
		section := "blog"
		if day%3 == 0 {
			section = "stories"
		}
		want = append(want, fmt.Sprintf("https://requiem.moe/%s/%03d-d/", section, day))
	}
	if !reflect.DeepEqual(feed.Links, want) {
		t.Errorf("feed links = %v\nwant %v", feed.Links, want)
	}
	feed.Links = nil
	if err := xml.Unmarshal(testRead(t, "stories.xml"), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Links) != 10 {
		t.Errorf("stories.xml has %d items, want every story", len(feed.Links))
	}
}

func TestBuildSite_RejectsBadInput(t *testing.T) {
	for _, tc := range []struct{ name, path, raw string }{
		{"escaping post dir", "blog/all.json", `[{"id":1,"dir":"../x","date":"2026-01-01T00:00:00Z"}]`},
		{"empty post dir", "blog/all.json", `[{"id":1,"dir":"","date":"2026-01-01T00:00:00Z"}]`},
		{"bad post date", "blog/all.json", `[{"id":1,"dir":"001-x","date":"yesterday"}]`},
		{"post not json", "blog/all.json", `[{`},
		{"escaping story dir", "stories/all.json", `[{"id":1,"dir":"../x","date":"2026-01-01T00:00:00Z"}]`},
		{"nested story dir", "stories/all.json", `[{"id":1,"dir":"001-x/y","date":"2026-01-01T00:00:00Z"}]`},
		{"absolute story dir", "stories/all.json", `[{"id":1,"dir":"/tmp","date":"2026-01-01T00:00:00Z"}]`},
		{"empty story dir", "stories/all.json", `[{"id":1,"dir":"","date":"2026-01-01T00:00:00Z"}]`},
		{"dot story dir", "stories/all.json", `[{"id":1,"dir":".","date":"2026-01-01T00:00:00Z"}]`},
		{"dotdot story dir", "stories/all.json", `[{"id":1,"dir":"..","date":"2026-01-01T00:00:00Z"}]`},
		{"dot post dir", "blog/all.json", `[{"id":1,"dir":".","date":"2026-01-01T00:00:00Z"}]`},
		{"bad story date", "stories/all.json", `[{"id":1,"dir":"001-x","date":"yesterday"}]`},
		{"bad story updated", "stories/all.json", `[{"id":1,"dir":"001-x","date":"2026-01-01T00:00:00Z","updated":"later"}]`},
		{"bad memo date", "memos/index.json", `[{"id":1,"date":"now","body":"x"}]`},
		{"memos not json", "memos/index.json", `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for _, dir := range []string{"blog/001-x", "stories/001-x/y"} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			testSeed(t)
			testWrite(t, "blog/index.json", "[]")
			testWrite(t, "blog/all.json", "[]")
			testWrite(t, tc.path, tc.raw)
			if changed, err := BuildSite(); err == nil || len(changed) != 0 {
				t.Errorf("BuildSite = %v, %v; want an error and no writes", changed, err)
			}
		})
	}
}

func TestBuildSite_FeedKeepsNewestTwenty(t *testing.T) {
	t.Chdir(t.TempDir())
	testSeed(t)
	var all []string
	for i := 21; i >= 1; i-- {
		dir := fmt.Sprintf("%03d-p", i)
		if err := os.MkdirAll("blog/"+dir, 0o755); err != nil {
			t.Fatal(err)
		}
		all = append(all, fmt.Sprintf(`{"id":%d,"dir":%q,"title":"p%d","date":"2026-01-%02dT00:00:00Z","tags":[],"author":"","hex":"","quote":"","image":"","body":"b"}`, i, dir, i, i))
	}
	testWrite(t, "blog/all.json", "["+strings.Join(all, ",")+"]")
	testWrite(t, "blog/index.json", "["+strings.Join(all, ",")+"]")

	if _, err := BuildSite(); err != nil {
		t.Fatal(err)
	}
	var feed struct {
		Links []string `xml:"channel>item>link"`
	}
	if err := xml.Unmarshal(testRead(t, "feed.xml"), &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Links) != 20 || feed.Links[0] != "https://requiem.moe/blog/021-p/" || feed.Links[19] != "https://requiem.moe/blog/002-p/" {
		t.Errorf("feed links = %d, first %q, last %q", len(feed.Links), feed.Links[0], feed.Links[len(feed.Links)-1])
	}
	var sitemap struct {
		Locs []string `xml:"url>loc"`
	}
	if err := xml.Unmarshal(testRead(t, "sitemap.xml"), &sitemap); err != nil {
		t.Fatal(err)
	}
	if len(sitemap.Locs) != 25 || !strings.Contains(string(testRead(t, "llms.txt")), "/blog/001-p/") {
		t.Errorf("sitemap has %d urls or llms.txt dropped the oldest post", len(sitemap.Locs))
	}
}

func TestWords(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int
	}{
		{"", 0},
		{"one two  three", 3},
		{"# Title\n\n**bold** and *it*", 4},
		{"a\n\n---\n\nb", 2},
		{"[link text](https://example.com/very/long)", 2},
		{"<script>alert(1)</script>word", 1},
		{"don&#39;t &amp; stop", 3},
		{"one<br>two three", 3},
		{"verse<br/>line<br>end", 3},
	} {
		if got := Words(tc.body); got != tc.want {
			t.Errorf("Words(%q) = %d, want %d", tc.body, got, tc.want)
		}
	}
}

// testSeed writes the empty story and memo data files a real site starts with.
func testSeed(t *testing.T) {
	t.Helper()
	for _, dir := range []string{"blog", "stories", "memos"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	testWrite(t, "stories/index.json", "[]\n")
	testWrite(t, "stories/all.json", "[]\n")
	testWrite(t, "memos/index.json", "[]\n")
}

func testPostDirs(t *testing.T) {
	t.Helper()
	for _, dir := range []string{"blog/002-newer", "blog/007-older"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
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

func testWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testReadJSON[T any](t *testing.T, path string) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(testRead(t, path), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
