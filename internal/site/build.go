package site

import (
	"bytes"
	"cmp"
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

const siteURL = "https://requiem.moe/"

//go:embed page.html archive.html story.html stories.html
var pageFiles embed.FS

var (
	pageTmpl = template.Must(template.ParseFS(pageFiles, "*.html"))
	markdown = goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithRendererOptions(gmhtml.WithUnsafe()))
	// Old bodies carry raw <br> and <a>, so raw HTML is rendered and then
	// sanitized, the same split the site had with marked and DOMPurify.
	sanitizer = bluemonday.UGCPolicy().
			RequireNoFollowOnLinks(false).
			RequireNoReferrerOnFullyQualifiedLinks(true).
			AddTargetBlankToFullyQualifiedLinks(true)
	textOnly = bluemonday.StrictPolicy().AddSpaceWhenStrippingTag(true)
	// Matched against sanitizer output, which always double-quotes attributes.
	hrefPattern    = regexp.MustCompile(`<a href="([^"]+)"`)
	youtubeIDShape = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

type pageData struct {
	Post        ArchivedPost
	URL         string
	Author      string
	Description string
	Image       string
	Stamp       string
	Tags        string
	Body        template.HTML
	LD          template.JS
	Newer       *ArchivedPost
	Older       *ArchivedPost
}

type storyData struct {
	Story       ArchivedStory
	URL         string
	Author      string
	Description string
	Image       string
	Stamp       string
	Updated     string
	Tags        string
	Words       int
	Minutes     int
	Body        template.HTML
	LD          template.JS
	Newer       *ArchivedStory
	Older       *ArchivedStory
}

type archiveRow struct {
	Dir, Title, Date, Stamp string
}

type storyRow struct {
	Dir, Title, Date, Stamp, Summary string
	Words, Minutes                   int
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        string   `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
	Author      string   `xml:"dc:creator"`
	Category    []string `xml:"category"`
	Description string   `xml:"description"`
	Content     string   `xml:"content:encoded"`
	date        time.Time
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssFeed struct {
	XMLName     xml.Name  `xml:"rss"`
	Version     string    `xml:"version,attr"`
	Atom        string    `xml:"xmlns:atom,attr"`
	Content     string    `xml:"xmlns:content,attr"`
	DC          string    `xml:"xmlns:dc,attr"`
	Title       string    `xml:"channel>title"`
	Link        string    `xml:"channel>link"`
	Self        atomLink  `xml:"channel>atom:link"`
	Description string    `xml:"channel>description"`
	Language    string    `xml:"channel>language"`
	Items       []rssItem `xml:"channel>item"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func render(body string) ([]byte, error) {
	var md bytes.Buffer
	if err := markdown.Convert([]byte(body), &md); err != nil {
		return nil, err
	}
	return sanitizer.SanitizeBytes(md.Bytes()), nil
}

// Words counts the words a reader sees: markdown and HTML syntax, link
// targets and scene-break rules do not count.
func Words(body string) int {
	// goldmark cannot fail writing to a buffer.
	rendered, _ := render(body)
	return len(strings.Fields(html.UnescapeString(textOnly.Sanitize(string(rendered)))))
}

func hashTags(tags []string) string {
	out := make([]string, len(tags))
	for n, t := range tags {
		out[n] = "#" + strings.Join(strings.Fields(strings.ToLower(t)), "-")
	}
	return strings.Join(out, " ")
}

func ldAuthor(name string) map[string]string {
	author := map[string]string{"@type": "Person", "name": name}
	if name == "requiem" {
		author["@id"] = siteURL + "#requiem"
		author["url"] = siteURL
	}
	return author
}

func newFeed(title, link, self, description string, items []rssItem) rssFeed {
	return rssFeed{
		Version:     "2.0",
		Atom:        "http://www.w3.org/2005/Atom",
		Content:     "http://purl.org/rss/1.0/modules/content/",
		DC:          "http://purl.org/dc/elements/1.1/",
		Title:       title,
		Link:        link,
		Self:        atomLink{siteURL + self, "self", "application/rss+xml"},
		Description: description,
		Language:    "en",
		Items:       items,
	}
}

// BuildSite renders every crawlable view of the weblog and the stories from
// blog/all.json, blog/index.json, stories/all.json and memos/index.json, fills
// each memo's html, and returns the paths whose bytes changed, so a commit
// only picks up what this build touched. Nothing is written unless every
// input parses.
func BuildSite() ([]string, error) {
	var posts []ArchivedPost
	var entries []BlogEntry
	var stories []ArchivedStory
	var memos []Memo
	for _, in := range []struct {
		path string
		v    any
	}{{"blog/all.json", &posts}, {"blog/index.json", &entries}, {"stories/all.json", &stories}, {"memos/index.json", &memos}} {
		raw, err := os.ReadFile(in.path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, in.v); err != nil {
			return nil, fmt.Errorf("%s: %w", in.path, err)
		}
	}
	excerpts := map[string]string{}
	for _, e := range entries {
		excerpts[e.Dir] = e.Excerpt
	}

	files := map[string][]byte{}
	postItems := make([]rssItem, 0, 20)
	storyItems := make([]rssItem, 0, len(stories))
	urls := []sitemapURL{{Loc: siteURL}, {Loc: siteURL + "shrine.html"}, {Loc: siteURL + "blog/"}, {Loc: siteURL + "stories/"}}
	rows := make([]archiveRow, 0, len(posts))
	storyRows := make([]storyRow, 0, len(stories))
	var llms strings.Builder
	llms.WriteString("# ::Requiem\n\n> requiem's corner of cyberspace: a weblog on self-taught development, side projects, mental health and healing, short fiction, plus a photo gallery.\n\n")
	llms.WriteString("Every weblog entry is a static page under " + siteURL + "blog/. The full archive with markdown bodies is " + siteURL + "blog/all.json and the RSS feed is " + siteURL + "feed.xml.\n\n")
	llms.WriteString("Every story is a static page under " + siteURL + "stories/. The full archive with markdown bodies is " + siteURL + "stories/all.json and the stories feed is " + siteURL + "stories.xml.\n\n## Posts\n\n")

	for i, p := range posts {
		if !ValidDir(p.Dir) {
			return nil, fmt.Errorf("blog/all.json: invalid post directory %q", p.Dir)
		}
		date, err := time.Parse(time.RFC3339, p.Date)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Dir, err)
		}
		body, err := render(p.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: render markdown: %w", p.Dir, err)
		}
		url := siteURL + "blog/" + p.Dir + "/"
		image := siteURL + "assets/og.jpg"
		if p.Image != "" {
			image = url + p.Image
		}
		d := pageData{
			Post:        p,
			URL:         url,
			Author:      cmp.Or(p.Author, "requiem"),
			Description: cmp.Or(excerpts[p.Dir], p.Title),
			Image:       image,
			Stamp:       date.Format("2006.01.02"),
			Tags:        hashTags(p.Tags),
			Body:        template.HTML(body),
		}
		if i > 0 {
			d.Newer = &posts[i-1]
		}
		if i+1 < len(posts) {
			d.Older = &posts[i+1]
		}
		ld, err := json.Marshal(map[string]any{
			"@context":         "https://schema.org",
			"@type":            "BlogPosting",
			"@id":              url + "#post",
			"url":              url,
			"mainEntityOfPage": url,
			"headline":         p.Title,
			"description":      d.Description,
			"datePublished":    p.Date,
			"dateModified":     p.Date,
			"inLanguage":       "en",
			"image":            image,
			"keywords":         p.Tags,
			"author":           ldAuthor(d.Author),
			"isPartOf":         map[string]string{"@id": siteURL + "#site"},
		})
		if err != nil {
			return nil, err
		}
		d.LD = template.JS(ld)
		var page bytes.Buffer
		if err := pageTmpl.ExecuteTemplate(&page, "page.html", d); err != nil {
			return nil, fmt.Errorf("%s: render page: %w", p.Dir, err)
		}
		files[filepath.Join("blog", p.Dir, "index.html")] = page.Bytes()

		// Feed readers refetch the whole file on every poll, so it stays small;
		// the sitemap, archive and llms.txt still list every post.
		if len(postItems) < 20 {
			postItems = append(postItems, rssItem{
				Title:       p.Title,
				Link:        url,
				GUID:        url,
				PubDate:     date.Format(time.RFC1123Z),
				Author:      d.Author,
				Category:    p.Tags,
				Description: d.Description,
				Content:     string(body),
				date:        date,
			})
		}
		urls = append(urls, sitemapURL{Loc: url, LastMod: p.Date})
		rows = append(rows, archiveRow{p.Dir, p.Title, p.Date, d.Stamp})
		fmt.Fprintf(&llms, "- [%s](%s): %s\n", p.Title, url, d.Description)
	}

	if len(stories) > 0 {
		llms.WriteString("\n## Stories\n\n")
	}
	for i, s := range stories {
		if !ValidDir(s.Dir) {
			return nil, fmt.Errorf("stories/all.json: invalid story directory %q", s.Dir)
		}
		date, err := time.Parse(time.RFC3339, s.Date)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Dir, err)
		}
		modified := cmp.Or(s.Updated, s.Date)
		urls[3].LastMod = max(urls[3].LastMod, modified)
		updated, err := time.Parse(time.RFC3339, modified)
		if err != nil {
			return nil, fmt.Errorf("%s: updated: %w", s.Dir, err)
		}
		body, err := render(s.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: render markdown: %w", s.Dir, err)
		}
		url := siteURL + "stories/" + s.Dir + "/"
		image := siteURL + "assets/og.jpg"
		if s.Image != "" {
			image = url + s.Image
		}
		words := Words(s.Body)
		d := storyData{
			Story:       s,
			URL:         url,
			Author:      cmp.Or(s.Author, "requiem"),
			Description: cmp.Or(s.Summary, s.Title),
			Image:       image,
			Stamp:       date.Format("2006.01.02"),
			Tags:        hashTags(s.Tags),
			Words:       words,
			Minutes:     max(1, words/230),
			Body:        template.HTML(body),
		}
		if s.Updated != "" {
			d.Updated = updated.Format("2006.01.02")
		}
		if i > 0 {
			d.Newer = &stories[i-1]
		}
		if i+1 < len(stories) {
			d.Older = &stories[i+1]
		}
		ld, err := json.Marshal(map[string]any{
			"@context":         "https://schema.org",
			"@type":            "ShortStory",
			"@id":              url + "#story",
			"url":              url,
			"mainEntityOfPage": url,
			"name":             s.Title,
			"headline":         s.Title,
			"description":      d.Description,
			"datePublished":    s.Date,
			"dateModified":     modified,
			"inLanguage":       "en",
			"image":            image,
			"keywords":         s.Tags,
			"wordCount":        words,
			"timeRequired":     fmt.Sprintf("PT%dM", d.Minutes),
			"author":           ldAuthor(d.Author),
			"isPartOf":         map[string]string{"@id": siteURL + "#site"},
		})
		if err != nil {
			return nil, err
		}
		d.LD = template.JS(ld)
		var page bytes.Buffer
		if err := pageTmpl.ExecuteTemplate(&page, "story.html", d); err != nil {
			return nil, fmt.Errorf("%s: render story: %w", s.Dir, err)
		}
		files[filepath.Join("stories", s.Dir, "index.html")] = page.Bytes()

		storyItems = append(storyItems, rssItem{
			Title:       s.Title,
			Link:        url,
			GUID:        url,
			PubDate:     date.Format(time.RFC1123Z),
			Author:      d.Author,
			Category:    append([]string{"story"}, s.Tags...),
			Description: d.Description,
			Content:     string(body),
			date:        date,
		})
		urls = append(urls, sitemapURL{Loc: url, LastMod: modified})
		storyRows = append(storyRows, storyRow{s.Dir, s.Title, s.Date, d.Stamp, s.Summary, words, d.Minutes})
		fmt.Fprintf(&llms, "- [%s](%s): %s (%d words)\n", s.Title, url, d.Description, words)
	}

	for n, m := range memos {
		if _, err := time.Parse(time.RFC3339, m.Date); err != nil {
			return nil, fmt.Errorf("memos/index.json: memo %d: %w", m.ID, err)
		}
		body, err := render(m.Body)
		if err != nil {
			return nil, fmt.Errorf("memos/index.json: memo %d: render markdown: %w", m.ID, err)
		}
		memos[n].HTML = string(body)
		memos[n].Videos = nil
		// Watch, youtu.be, shorts, live and embed links each become a video.
		for _, href := range hrefPattern.FindAllSubmatch(body, -1) {
			u, err := url.Parse(html.UnescapeString(string(href[1])))
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
				continue
			}
			var id string
			segs := strings.Split(strings.Trim(u.Path, "/"), "/")
			switch strings.TrimPrefix(u.Hostname(), "www.") {
			case "youtu.be":
				id = segs[0]
			case "youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com":
				switch {
				case segs[0] == "watch":
					id = u.Query().Get("v")
				case len(segs) == 2 && (segs[0] == "shorts" || segs[0] == "live" || segs[0] == "embed"):
					id = segs[1]
				}
			}
			if !youtubeIDShape.MatchString(id) || slices.ContainsFunc(memos[n].Videos, func(seen Video) bool { return seen.ID == id }) {
				continue
			}
			v := Video{ID: id}
			// t=90, t=90s and t=1m30s all appear in shared links.
			t := cmp.Or(u.Query().Get("t"), u.Query().Get("start"))
			if t != "" && strings.Trim(t, "0123456789") == "" {
				t += "s"
			}
			if d, err := time.ParseDuration(t); err == nil && d > 0 {
				v.Start = int(d.Seconds())
			}
			memos[n].Videos = append(memos[n].Videos, v)
		}
	}
	var memoJSON bytes.Buffer
	enc := json.NewEncoder(&memoJSON)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(memos); err != nil {
		return nil, err
	}
	files["memos/index.json"] = memoJSON.Bytes()

	var archive bytes.Buffer
	if err := pageTmpl.ExecuteTemplate(&archive, "archive.html", rows); err != nil {
		return nil, fmt.Errorf("render archive: %w", err)
	}
	files["blog/index.html"] = archive.Bytes()
	var storyList bytes.Buffer
	if err := pageTmpl.ExecuteTemplate(&storyList, "stories.html", storyRows); err != nil {
		return nil, fmt.Errorf("render story list: %w", err)
	}
	files["stories/index.html"] = storyList.Bytes()
	files["llms.txt"] = []byte(llms.String())

	if len(posts) > 0 {
		urls[2].LastMod = posts[0].Date
	}

	// Both lists are already newest first, so a merge keeps the weblog's own
	// order even where its dates are out of sequence.
	merged := make([]rssItem, 0, 20)
	for p, s := 0, 0; len(merged) < 20 && (p < len(postItems) || s < len(storyItems)); {
		if s < len(storyItems) && (p == len(postItems) || storyItems[s].date.After(postItems[p].date)) {
			merged = append(merged, storyItems[s])
			s++
			continue
		}
		merged = append(merged, postItems[p])
		p++
	}
	sitemap := struct {
		XMLName xml.Name     `xml:"urlset"`
		NS      string       `xml:"xmlns,attr"`
		URLs    []sitemapURL `xml:"url"`
	}{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls}
	for path, v := range map[string]any{
		"feed.xml":    newFeed("::Requiem weblog", siteURL, "feed.xml", "requiem's weblog on self-taught development, side projects, mental health and healing.", merged),
		"stories.xml": newFeed("::Requiem stories", siteURL+"stories/", "stories.xml", "requiem's short fiction.", storyItems),
		"sitemap.xml": sitemap,
	} {
		raw, err := xml.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		files[path] = append([]byte(xml.Header), append(raw, '\n')...)
	}

	var changed []string
	for path, data := range files {
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
			continue
		}
		if err := WriteFile(path, data); err != nil {
			return changed, err
		}
		changed = append(changed, path)
	}
	slices.Sort(changed)
	return changed, nil
}
