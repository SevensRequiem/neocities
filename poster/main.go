package main

import (
	"bytes"
	"cmp"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"neocities/internal/site"
)

//go:embed ui.html
var uiHTML string

//go:embed ui.css ui.js
var uiFiles embed.FS

var (
	markupRe = regexp.MustCompile("<[^>]*>|[#*`]")
	qaClient = &http.Client{Timeout: 10 * time.Second}
)

func main() {
	build := flag.Bool("build", false, "rebuild the static blog and story pages, archives, feeds, sitemap, llms.txt and memo html, then exit")
	flag.Parse()
	for _, p := range []string{"blog/index.json", "gallery/index.json", "stories/all.json", "memos/index.json", "assets/js/marked.min.js"} {
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintln(os.Stderr, "run from the site root: go run ./poster")
			os.Exit(2)
		}
	}
	if *build {
		lock, err := site.LockSite()
		if err != nil {
			fmt.Fprintln(os.Stderr, "lock site:", err)
			os.Exit(1)
		}
		defer lock.Close()
		changed, err := site.BuildSite()
		for _, path := range changed {
			fmt.Println(path)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	// poster/.env keeps QA_TOKEN across restarts; variables already in the
	// environment win so a one-off override still works.
	if raw, err := os.ReadFile("poster/.env"); err == nil {
		for line := range strings.Lines(string(raw)) {
			key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
			key = strings.TrimSpace(key)
			if !ok || key == "" || strings.HasPrefix(key, "#") {
				continue
			}
			if _, set := os.LookupEnv(key); !set {
				_ = os.Setenv(key, strings.Trim(strings.TrimSpace(value), `"'`))
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "read poster/.env:", err)
		os.Exit(1)
	}
	if _, err := exec.LookPath("magick"); err != nil {
		fmt.Fprintln(os.Stderr, "magick (ImageMagick 7) is required for webp conversion")
		os.Exit(1)
	}

	ln, host, token, err := site.Listen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	page := template.Must(template.New("ui").Parse(uiHTML))
	guard := site.Guard(host, token)

	mux := http.NewServeMux()
	site.Mount(mux)
	mux.Handle("GET /ui.css", http.FileServerFS(uiFiles))
	mux.Handle("GET /ui.js", http.FileServerFS(uiFiles))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		// default-src does not cover framing: any site could frame the UI and
		// click it into publishing or deleting.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; frame-ancestors 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		raw, err := os.ReadFile("blog/index.json")
		if err != nil {
			site.Fail(w, "read blog index", err)
			return
		}
		var entries []site.BlogEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			site.Fail(w, "parse blog index", err)
			return
		}
		uses := map[string]int{}
		for _, e := range entries {
			for _, t := range e.Tags {
				uses[t]++
			}
		}
		tags := slices.SortedFunc(maps.Keys(uses), func(a, b string) int {
			return cmp.Or(cmp.Compare(uses[b], uses[a]), cmp.Compare(a, b))
		})
		tagJSON, _ := json.Marshal(tags)
		if err := page.Execute(w, map[string]any{"Token": token, "Tags": string(tagJSON)}); err != nil {
			slog.Error("render ui", "err", err)
		}
	})
	mux.HandleFunc("POST /blog", guard(postBlog))
	mux.HandleFunc("POST /gallery", guard(postGallery))
	mux.HandleFunc("POST /memo", guard(postMemo))
	mux.HandleFunc("GET /qa/inbox", guard(proxyQA))
	mux.HandleFunc("PUT /qa/{id}", guard(proxyQA))
	mux.HandleFunc("DELETE /qa/{id}", guard(proxyQA))

	if err := site.Serve(ln, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func postBlog(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimRight(strings.ReplaceAll(r.FormValue("body"), "\r\n", "\n"), " \t\n")
	if title == "" || body == "" {
		site.Reply(w, http.StatusBadRequest, "error", "title and body are required")
		return
	}
	index, id, err := site.ReadIndex("blog/index.json")
	if err != nil {
		site.Fail(w, "read blog index", err)
		return
	}
	archiveRaw, err := os.ReadFile("blog/all.json")
	if err != nil {
		site.Fail(w, "read blog archive", err)
		return
	}
	var archive []json.RawMessage
	if err := json.Unmarshal(archiveRaw, &archive); err != nil {
		site.Fail(w, "parse blog archive", err)
		return
	}
	if archive == nil {
		site.Fail(w, "parse blog archive", errors.New("expected a JSON array"))
		return
	}
	for _, post := range archive {
		if !bytes.HasPrefix(bytes.TrimSpace(post), []byte("{")) {
			site.Fail(w, "parse blog archive", errors.New("expected post objects"))
			return
		}
	}

	now := time.Now().UTC()
	entry := site.BlogEntry{
		ID:      id,
		Dir:     site.DirName(id, title),
		Title:   title,
		Date:    now.Format(time.RFC3339),
		Tags:    site.SplitTags(r.FormValue("tags")),
		Excerpt: strings.Join(strings.Fields(markupRe.ReplaceAllString(body, "")), " "),
	}
	if len(entry.Excerpt) > 220 {
		cut := entry.Excerpt[:220]
		entry.Excerpt = cut[:strings.LastIndexByte(cut, ' ')+1]
		entry.Excerpt = strings.TrimRight(entry.Excerpt, " ") + "..."
	}

	dir := filepath.Join("blog", entry.Dir)
	if err := os.Mkdir(dir, 0o755); err != nil {
		site.Fail(w, "create post dir", err)
		return
	}
	upload, _, err := r.FormFile("image")
	if err == nil {
		defer upload.Close()
		entry.Image = "image.webp"
		if err := site.Convert(r.Context(), upload, filepath.Join(dir, "image.webp"), "2000x2000>"); err != nil {
			os.RemoveAll(dir)
			site.Fail(w, "convert image", err)
			return
		}
	} else if !errors.Is(err, http.ErrMissingFile) {
		os.RemoveAll(dir)
		site.Fail(w, "read upload", err)
		return
	}

	post := site.BlogPost{ID: id, Title: title, Date: entry.Date, Tags: entry.Tags, Author: strings.TrimSpace(r.FormValue("author")), Hex: site.PostHex(now), Quote: strings.TrimSpace(r.FormValue("quote")), Image: entry.Image, Body: body}
	err = site.WriteJSON(filepath.Join(dir, "post.json"), post)
	if err == nil {
		err = site.WriteIndex("blog/all.json", archive, site.ArchivedPost{BlogPost: post, Dir: entry.Dir})
	}
	if err == nil {
		err = site.WriteIndex("blog/index.json", index, entry)
		if err != nil {
			// Keep the original bytes so a failed publish leaves no archive diff.
			if rollbackErr := site.WriteFile("blog/all.json", archiveRaw); rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("restore blog archive: %w", rollbackErr))
			}
		}
	}
	if err != nil {
		site.Fail(w, "write post", errors.Join(err, os.RemoveAll(dir)))
		return
	}
	// The post is the source of truth and is already saved; a failed page
	// build leaves it in place for `go run ./poster -build` to finish.
	pages, err := site.BuildSite()
	if err != nil {
		site.Fail(w, "build pages", err)
		return
	}
	paths := append([]string{dir, "blog/index.json", "blog/all.json"}, pages...)
	site.Reply(w, http.StatusOK, "done", "written to "+dir+site.Publish(r.Context(), r.FormValue("commit") != "", r.FormValue("push") != "", "post: "+filepath.ToSlash(dir), paths...))
}

func postGallery(w http.ResponseWriter, r *http.Request) {
	caption := strings.TrimSpace(r.FormValue("caption"))
	upload, _, err := r.FormFile("image")
	if caption == "" || err != nil {
		site.Reply(w, http.StatusBadRequest, "error", "caption and image are required")
		return
	}
	defer upload.Close()
	index, id, err := site.ReadIndex("gallery/index.json")
	if err != nil {
		site.Fail(w, "read gallery index", err)
		return
	}

	now := time.Now().UTC()
	entry := struct {
		ID      int      `json:"id"`
		Dir     string   `json:"dir"`
		Caption string   `json:"caption"`
		Date    string   `json:"date"`
		Tags    []string `json:"tags"`
		Image   string   `json:"image"`
		Thumb   string   `json:"thumb"`
	}{
		ID:      id,
		Dir:     site.DirName(id, caption),
		Caption: caption,
		Date:    now.Format(time.RFC3339),
		Tags:    site.SplitTags(r.FormValue("tags")),
		Image:   "image.webp",
		Thumb:   "thumb.webp",
	}
	dir := filepath.Join("gallery", entry.Dir)
	if err := os.Mkdir(dir, 0o755); err != nil {
		site.Fail(w, "create gallery dir", err)
		return
	}
	err = site.Convert(r.Context(), upload, filepath.Join(dir, "image.webp"), "2000x2000>")
	if err == nil {
		var full *os.File
		if full, err = os.Open(filepath.Join(dir, "image.webp")); err == nil {
			err = site.Convert(r.Context(), full, filepath.Join(dir, "thumb.webp"), "800x800>")
			full.Close()
		}
	}
	if err != nil {
		os.RemoveAll(dir)
		site.Fail(w, "convert image", err)
		return
	}

	err = site.WriteJSON(filepath.Join(dir, "post.json"), struct {
		ID      int      `json:"id"`
		Caption string   `json:"caption"`
		Date    string   `json:"date"`
		Tags    []string `json:"tags"`
		Hex     string   `json:"hex"`
		Image   string   `json:"image"`
		Thumb   string   `json:"thumb"`
	}{id, caption, entry.Date, entry.Tags, site.PostHex(now), entry.Image, entry.Thumb})
	if err == nil {
		err = site.WriteIndex("gallery/index.json", index, entry)
	}
	if err != nil {
		os.RemoveAll(dir)
		site.Fail(w, "write gallery item", err)
		return
	}
	site.Reply(w, http.StatusOK, "done", "written to "+dir+site.Publish(r.Context(), r.FormValue("commit") != "", r.FormValue("push") != "", "post: "+filepath.ToSlash(dir), dir, "gallery/index.json"))
}

// postMemo prepends a memo. BuildSite fills its html, so a failed build puts
// the old index back instead of leaving a memo the homepage cannot show.
func postMemo(w http.ResponseWriter, r *http.Request) {
	body := strings.TrimSpace(strings.ReplaceAll(r.FormValue("body"), "\r\n", "\n"))
	if n := utf8.RuneCountInString(body); n < 1 || n > 1000 {
		site.Reply(w, http.StatusBadRequest, "error", "a memo is 1 to 1000 characters")
		return
	}
	raw, err := os.ReadFile("memos/index.json")
	if err != nil {
		site.Fail(w, "read memos", err)
		return
	}
	index, id, err := site.ReadIndex("memos/index.json")
	if err != nil {
		site.Fail(w, "read memos", err)
		return
	}
	memo := site.Memo{ID: id, Date: time.Now().UTC().Format(time.RFC3339), Body: body}
	if err := site.WriteIndex("memos/index.json", index, memo); err != nil {
		site.Fail(w, "write memo", err)
		return
	}
	pages, err := site.BuildSite()
	if err != nil {
		if rollbackErr := site.WriteFile("memos/index.json", raw); rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("restore memos: %w", rollbackErr))
		}
		site.Fail(w, "build pages", err)
		return
	}
	name := fmt.Sprintf("memo: %03d", id)
	site.Reply(w, http.StatusOK, "done", name+" written"+site.Publish(r.Context(), r.FormValue("commit") != "", r.FormValue("push") != "", name, append([]string{"memos/index.json"}, pages...)...))
}

// proxyQA forwards Q&A moderation to the worker so the admin token stays in
// the poster's environment instead of the browser. QA_API points it at
// `wrangler dev` for local testing.
func proxyQA(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("QA_TOKEN")
	if token == "" {
		site.Reply(w, http.StatusServiceUnavailable, "error", "set QA_TOKEN to the worker's secret and restart the poster")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, cmp.Or(os.Getenv("QA_API"), "https://api.requiem.moe")+r.URL.Path, r.Body)
	if err != nil {
		site.Fail(w, "build api request", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := qaClient.Do(req)
	if err != nil {
		site.Fail(w, "reach api", err)
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		site.Fail(w, "read api reply", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		site.Reply(w, http.StatusBadGateway, "error", fmt.Sprintf("api answered %d: %s", res.StatusCode, bytes.TrimSpace(body)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}
