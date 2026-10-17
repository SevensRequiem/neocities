package main

import (
	"bytes"
	"cmp"
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

const siteURL = "https://requiem.moe/"

//go:embed page.html archive.html
var pageFiles embed.FS

var (
	pageTmpl = template.Must(template.ParseFS(pageFiles, "page.html", "archive.html"))
	markdown = goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithRendererOptions(html.WithUnsafe()))
	// Old bodies carry raw <br> and <a>, so raw HTML is rendered and then
	// sanitized, the same split the site had with marked and DOMPurify.
	sanitizer = bluemonday.UGCPolicy().
			RequireNoFollowOnLinks(false).
			RequireNoReferrerOnFullyQualifiedLinks(true).
			AddTargetBlankToFullyQualifiedLinks(true)
)

type sitePost struct {
	blogPost
	Dir string `json:"dir"`
}

type pageData struct {
	Post        sitePost
	URL         string
	Author      string
	Description string
	Image       string
	Stamp       string
	Tags        string
	Body        template.HTML
	LD          template.JS
	Newer       *sitePost
	Older       *sitePost
}

type archiveRow struct {
	Dir, Title, Date, Stamp string
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
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// buildSite renders every crawlable view of the blog from blog/all.json and
// blog/index.json and returns the paths whose bytes changed, so a commit only
// picks up what this build touched.
func buildSite() ([]string, error) {
	var posts []sitePost
	var entries []blogEntry
	for path, v := range map[string]any{"blog/all.json": &posts, "blog/index.json": &entries} {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, v); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	excerpts := map[string]string{}
	for _, e := range entries {
		excerpts[e.Dir] = e.Excerpt
	}

	files := map[string][]byte{}
	items := make([]rssItem, 0, 20)
	urls := []sitemapURL{{Loc: siteURL}, {Loc: siteURL + "shrine.html"}, {Loc: siteURL + "blog/"}}
	rows := make([]archiveRow, 0, len(posts))
	var llms strings.Builder
	llms.WriteString("# ::Requiem\n\n> requiem's corner of cyberspace: a weblog on self-taught development, side projects, mental health and healing, plus a photo gallery.\n\n")
	llms.WriteString("Every weblog entry is a static page under " + siteURL + "blog/. The full archive with markdown bodies is " + siteURL + "blog/all.json and the RSS feed is " + siteURL + "feed.xml.\n\n## Posts\n\n")

	for i, p := range posts {
		if p.Dir == "" || filepath.Base(p.Dir) != p.Dir || !filepath.IsLocal(p.Dir) {
			return nil, fmt.Errorf("blog/all.json: invalid post directory %q", p.Dir)
		}
		date, err := time.Parse(time.RFC3339, p.Date)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Dir, err)
		}
		var md bytes.Buffer
		if err := markdown.Convert([]byte(p.Body), &md); err != nil {
			return nil, fmt.Errorf("%s: render markdown: %w", p.Dir, err)
		}
		body := sanitizer.SanitizeBytes(md.Bytes())
		url := siteURL + "blog/" + p.Dir + "/"
		image := siteURL + "assets/og.jpg"
		if p.Image != "" {
			image = url + p.Image
		}
		tags := make([]string, len(p.Tags))
		for n, t := range p.Tags {
			tags[n] = "#" + strings.Join(strings.Fields(strings.ToLower(t)), "-")
		}
		d := pageData{
			Post:        p,
			URL:         url,
			Author:      cmp.Or(p.Author, "requiem"),
			Description: cmp.Or(excerpts[p.Dir], p.Title),
			Image:       image,
			Stamp:       date.Format("2006.01.02"),
			Tags:        strings.Join(tags, " "),
			Body:        template.HTML(body),
		}
		if i > 0 {
			d.Newer = &posts[i-1]
		}
		if i+1 < len(posts) {
			d.Older = &posts[i+1]
		}
		author := map[string]string{"@type": "Person", "name": d.Author}
		if d.Author == "requiem" {
			author["@id"] = siteURL + "#requiem"
			author["url"] = siteURL
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
			"author":           author,
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
		if len(items) < 20 {
			items = append(items, rssItem{
				Title:       p.Title,
				Link:        url,
				GUID:        url,
				PubDate:     date.Format(time.RFC1123Z),
				Author:      d.Author,
				Category:    p.Tags,
				Description: d.Description,
				Content:     string(body),
			})
		}
		urls = append(urls, sitemapURL{Loc: url, LastMod: p.Date})
		rows = append(rows, archiveRow{p.Dir, p.Title, p.Date, d.Stamp})
		fmt.Fprintf(&llms, "- [%s](%s): %s\n", p.Title, url, d.Description)
	}

	var archive bytes.Buffer
	if err := pageTmpl.ExecuteTemplate(&archive, "archive.html", rows); err != nil {
		return nil, fmt.Errorf("render archive: %w", err)
	}
	files["blog/index.html"] = archive.Bytes()
	files["llms.txt"] = []byte(llms.String())

	newest := ""
	if len(posts) > 0 {
		newest = posts[0].Date
		urls[2].LastMod = newest
	}
	type link struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
	}
	feed := struct {
		XMLName     xml.Name  `xml:"rss"`
		Version     string    `xml:"version,attr"`
		Atom        string    `xml:"xmlns:atom,attr"`
		Content     string    `xml:"xmlns:content,attr"`
		DC          string    `xml:"xmlns:dc,attr"`
		Title       string    `xml:"channel>title"`
		Link        string    `xml:"channel>link"`
		Self        link      `xml:"channel>atom:link"`
		Description string    `xml:"channel>description"`
		Language    string    `xml:"channel>language"`
		Items       []rssItem `xml:"channel>item"`
	}{
		Version:     "2.0",
		Atom:        "http://www.w3.org/2005/Atom",
		Content:     "http://purl.org/rss/1.0/modules/content/",
		DC:          "http://purl.org/dc/elements/1.1/",
		Title:       "::Requiem weblog",
		Link:        siteURL,
		Self:        link{siteURL + "feed.xml", "self", "application/rss+xml"},
		Description: "requiem's weblog on self-taught development, side projects, mental health and healing.",
		Language:    "en",
		Items:       items,
	}
	sitemap := struct {
		XMLName xml.Name     `xml:"urlset"`
		NS      string       `xml:"xmlns,attr"`
		URLs    []sitemapURL `xml:"url"`
	}{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls}
	for path, v := range map[string]any{"feed.xml": feed, "sitemap.xml": sitemap} {
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
		if err := writeFile(path, data); err != nil {
			return changed, err
		}
		changed = append(changed, path)
	}
	slices.Sort(changed)
	return changed, nil
}
