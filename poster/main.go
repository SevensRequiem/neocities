package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed ui.html
var uiHTML string

//go:embed ui.css ui.js
var uiFiles embed.FS

var (
	slugRe   = regexp.MustCompile(`[^a-z0-9]+`)
	markupRe = regexp.MustCompile("<[^>]*>|[#*`]")
	qaClient = &http.Client{Timeout: 10 * time.Second}
)

type blogEntry struct {
	ID      int      `json:"id"`
	Dir     string   `json:"dir"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Excerpt string   `json:"excerpt"`
	Image   string   `json:"image"`
}

type galleryEntry struct {
	ID      int      `json:"id"`
	Dir     string   `json:"dir"`
	Caption string   `json:"caption"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Image   string   `json:"image"`
	Thumb   string   `json:"thumb"`
}

type blogPost struct {
	ID     int      `json:"id"`
	Title  string   `json:"title"`
	Date   string   `json:"date"`
	Tags   []string `json:"tags"`
	Author string   `json:"author"`
	Hex    string   `json:"hex"`
	Quote  string   `json:"quote"`
	Image  string   `json:"image"`
	Body   string   `json:"body"`
}

func main() {
	build := flag.Bool("build", false, "rebuild the static blog pages, archive, feed, sitemap and llms.txt, then exit")
	flag.Parse()
	for _, p := range []string{"blog/index.json", "gallery/index.json", "assets/js/marked.min.js"} {
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintln(os.Stderr, "run from the site root: go run ./poster")
			os.Exit(2)
		}
	}
	if *build {
		changed, err := buildSite()
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

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	host := ln.Addr().String()
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	token := hex.EncodeToString(secret)
	page := template.Must(template.New("ui").Parse(uiHTML))

	guard := posterGuard(host, token)

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.FileServer(http.Dir(".")))
	mux.Handle("GET /ui.css", http.FileServerFS(uiFiles))
	mux.Handle("GET /ui.js", http.FileServerFS(uiFiles))
	// The site fetches its indexes, which browsers refuse on file://, so the
	// poster also serves the deployable paths for local viewing.
	site := http.StripPrefix("/site", http.FileServer(http.Dir(".")))
	mux.Handle("GET /site/assets/", site)
	mux.Handle("GET /site/blog/", site)
	mux.Handle("GET /site/gallery/", site)
	mux.HandleFunc("GET /site/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:")
		w.Header().Set("Cache-Control", "no-store")
		raw, err := os.ReadFile("blog/index.json")
		if err != nil {
			fail(w, "read blog index", err)
			return
		}
		var entries []blogEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			fail(w, "parse blog index", err)
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
	mux.HandleFunc("GET /qa/inbox", guard(proxyQA))
	mux.HandleFunc("PUT /qa/{id}", guard(proxyQA))
	mux.HandleFunc("DELETE /qa/{id}", guard(proxyQA))

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	url := "http://" + host + "/"
	fmt.Println(url)
	_ = exec.Command("xdg-open", url).Start()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func posterGuard(host, token string) func(http.HandlerFunc) http.HandlerFunc {
	var writes sync.Mutex
	// A page on any origin can POST to a loopback port, so writes need the
	// per-run token in a custom header and the Host must be ours (DNS rebinding).
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Host != host || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Poster-Token")), []byte(token)) != 1 {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			// Index read-modify-write and git's staging area are shared by both forms.
			writes.Lock()
			defer writes.Unlock()
			r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
			defer cancel()
			next(w, r.WithContext(ctx))
		}
	}
}

func postBlog(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	body := strings.TrimRight(strings.ReplaceAll(r.FormValue("body"), "\r\n", "\n"), " \t\n")
	if title == "" || body == "" {
		reply(w, http.StatusBadRequest, "error", "title and body are required")
		return
	}
	index, id, err := readIndex("blog/index.json")
	if err != nil {
		fail(w, "read blog index", err)
		return
	}
	archiveRaw, err := os.ReadFile("blog/all.json")
	if err != nil {
		fail(w, "read blog archive", err)
		return
	}
	var archive []json.RawMessage
	if err := json.Unmarshal(archiveRaw, &archive); err != nil {
		fail(w, "parse blog archive", err)
		return
	}
	if archive == nil {
		fail(w, "parse blog archive", errors.New("expected a JSON array"))
		return
	}
	for _, post := range archive {
		if !bytes.HasPrefix(bytes.TrimSpace(post), []byte("{")) {
			fail(w, "parse blog archive", errors.New("expected post objects"))
			return
		}
	}

	now := time.Now().UTC()
	entry := blogEntry{
		ID:      id,
		Dir:     dirName(id, title),
		Title:   title,
		Date:    now.Format(time.RFC3339),
		Tags:    splitTags(r.FormValue("tags")),
		Excerpt: strings.Join(strings.Fields(markupRe.ReplaceAllString(body, "")), " "),
	}
	if len(entry.Excerpt) > 220 {
		cut := entry.Excerpt[:220]
		entry.Excerpt = cut[:strings.LastIndexByte(cut, ' ')+1]
		entry.Excerpt = strings.TrimRight(entry.Excerpt, " ") + "..."
	}

	dir := filepath.Join("blog", entry.Dir)
	if err := os.Mkdir(dir, 0o755); err != nil {
		fail(w, "create post dir", err)
		return
	}
	upload, _, err := r.FormFile("image")
	if err == nil {
		defer upload.Close()
		entry.Image = "image.webp"
		if err := convert(r.Context(), upload, filepath.Join(dir, "image.webp"), "2000x2000>"); err != nil {
			os.RemoveAll(dir)
			fail(w, "convert image", err)
			return
		}
	} else if !errors.Is(err, http.ErrMissingFile) {
		os.RemoveAll(dir)
		fail(w, "read upload", err)
		return
	}

	post := blogPost{id, title, entry.Date, entry.Tags, strings.TrimSpace(r.FormValue("author")), postHex(now), strings.TrimSpace(r.FormValue("quote")), entry.Image, body}
	err = writeJSON(filepath.Join(dir, "post.json"), post)
	if err == nil {
		err = writeIndex("blog/all.json", archive, struct {
			blogPost
			Dir string `json:"dir"`
		}{post, entry.Dir})
	}
	if err == nil {
		err = writeIndex("blog/index.json", index, entry)
		if err != nil {
			// Keep the original bytes so a failed publish leaves no archive diff.
			if rollbackErr := writeFile("blog/all.json", archiveRaw); rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("restore blog archive: %w", rollbackErr))
			}
		}
	}
	if err != nil {
		fail(w, "write post", errors.Join(err, os.RemoveAll(dir)))
		return
	}
	// The post is the source of truth and is already saved; a failed page
	// build leaves it in place for `go run ./poster -build` to finish.
	pages, err := buildSite()
	if err != nil {
		fail(w, "build pages", err)
		return
	}
	reply(w, http.StatusOK, "done", "written to "+dir+publish(r, dir, append([]string{"blog/index.json", "blog/all.json"}, pages...)...))
}

func postGallery(w http.ResponseWriter, r *http.Request) {
	caption := strings.TrimSpace(r.FormValue("caption"))
	upload, _, err := r.FormFile("image")
	if caption == "" || err != nil {
		reply(w, http.StatusBadRequest, "error", "caption and image are required")
		return
	}
	defer upload.Close()
	index, id, err := readIndex("gallery/index.json")
	if err != nil {
		fail(w, "read gallery index", err)
		return
	}

	now := time.Now().UTC()
	entry := galleryEntry{
		ID:      id,
		Dir:     dirName(id, caption),
		Caption: caption,
		Date:    now.Format(time.RFC3339),
		Tags:    splitTags(r.FormValue("tags")),
		Image:   "image.webp",
		Thumb:   "thumb.webp",
	}
	dir := filepath.Join("gallery", entry.Dir)
	if err := os.Mkdir(dir, 0o755); err != nil {
		fail(w, "create gallery dir", err)
		return
	}
	err = convert(r.Context(), upload, filepath.Join(dir, "image.webp"), "2000x2000>")
	if err == nil {
		var full *os.File
		if full, err = os.Open(filepath.Join(dir, "image.webp")); err == nil {
			err = convert(r.Context(), full, filepath.Join(dir, "thumb.webp"), "800x800>")
			full.Close()
		}
	}
	if err != nil {
		os.RemoveAll(dir)
		fail(w, "convert image", err)
		return
	}

	err = writeJSON(filepath.Join(dir, "post.json"), struct {
		ID      int      `json:"id"`
		Caption string   `json:"caption"`
		Date    string   `json:"date"`
		Tags    []string `json:"tags"`
		Hex     string   `json:"hex"`
		Image   string   `json:"image"`
		Thumb   string   `json:"thumb"`
	}{id, caption, entry.Date, entry.Tags, postHex(now), entry.Image, entry.Thumb})
	if err == nil {
		err = writeIndex("gallery/index.json", index, entry)
	}
	if err != nil {
		os.RemoveAll(dir)
		fail(w, "write gallery item", err)
		return
	}
	reply(w, http.StatusOK, "done", "written to "+dir+publish(r, dir, "gallery/index.json"))
}

// proxyQA forwards Q&A moderation to the worker so the admin token stays in
// the poster's environment instead of the browser. QA_API points it at
// `wrangler dev` for local testing.
func proxyQA(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("QA_TOKEN")
	if token == "" {
		reply(w, http.StatusServiceUnavailable, "error", "set QA_TOKEN to the worker's secret and restart the poster")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, cmp.Or(os.Getenv("QA_API"), "https://api.requiem.moe")+r.URL.Path, r.Body)
	if err != nil {
		fail(w, "build api request", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := qaClient.Do(req)
	if err != nil {
		fail(w, "reach api", err)
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		fail(w, "read api reply", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		reply(w, http.StatusBadGateway, "error", fmt.Sprintf("api answered %d: %s", res.StatusCode, bytes.TrimSpace(body)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

// convert re-encodes any image ImageMagick can read as webp, dropping EXIF
// (camera, timestamp, GPS) and shrinking it to fit the given geometry.
func convert(ctx context.Context, src io.Reader, dst, geometry string) error {
	tmp, err := os.CreateTemp("", "poster-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, src)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "magick", tmp.Name(), "-auto-orient", "-strip", "-resize", geometry, "-quality", "82", "webp:"+dst).CombinedOutput()
	if err != nil {
		return fmt.Errorf("magick: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// readIndex returns the index entries and the next free id. Entries stay raw
// so fields the poster does not know about survive a rewrite.
func readIndex(path string) ([]json.RawMessage, int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	var entries []json.RawMessage
	var ids []struct {
		ID int `json:"id"`
	}
	if err := errors.Join(json.Unmarshal(raw, &entries), json.Unmarshal(raw, &ids)); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	next := 1
	for _, e := range ids {
		next = max(next, e.ID+1)
	}
	return entries, next, nil
}

func writeIndex(path string, entries []json.RawMessage, entry any) error {
	all := []any{entry}
	for _, e := range entries {
		all = append(all, e)
	}
	return writeJSON(path, all)
}

// writeJSON skips HTML escaping so &, < and > stay readable in the index and
// post files.
func writeJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return writeFile(path, buf.Bytes())
}

func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	defer func() {
		if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Error("remove temporary file", "path", tmp, "err", err)
		}
	}()
	_, err = file.Write(data)
	if err := errors.Join(err, file.Close()); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func dirName(id int, title string) string {
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if slug == "" {
		slug = "untitled"
	}
	return fmt.Sprintf("%03d-%s", id, slug)
}

func splitTags(s string) []string {
	tags := []string{}
	for t := range strings.SplitSeq(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func reply(w http.ResponseWriter, status int, key, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{key: value})
}

func fail(w http.ResponseWriter, stage string, err error) {
	slog.Error(stage, "err", err)
	reply(w, http.StatusInternalServerError, "error", stage+" failed, see the terminal")
}

// postHex mirrors the Mongo ObjectID the old site stored in each post's hex
// field: a 4-byte timestamp followed by 8 random bytes.
func postHex(now time.Time) string {
	oid := make([]byte, 12)
	binary.BigEndian.PutUint32(oid, uint32(now.Unix()))
	_, _ = rand.Read(oid[4:])
	return hex.EncodeToString(oid)
}

// publish runs the git steps ticked in the form. The post is already on disk,
// so a git failure is reported in the status text instead of failing the request.
func publish(r *http.Request, dir string, indexes ...string) string {
	status := ""
	if r.FormValue("commit") != "" {
		paths := append([]string{dir}, indexes...)
		err := git(r.Context(), append([]string{"add", "--"}, paths...)...)
		if err == nil {
			err = git(r.Context(), append([]string{"commit", "-m", "post: " + filepath.ToSlash(dir), "--"}, paths...)...)
		}
		if err != nil {
			slog.Error("git commit", "err", err)
			return " :: commit failed, see the terminal"
		}
		status = " :: committed"
	}
	if r.FormValue("push") == "" {
		return status
	}
	if err := git(r.Context(), "push", "-u", "origin", "HEAD"); err != nil {
		slog.Error("git push", "err", err)
		return status + " :: push failed, see the terminal"
	}
	return status + " :: pushed"
}

func git(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	// Without a terminal a credential or passphrase prompt would hang until the deadline.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", args[0], err, bytes.TrimSpace(out))
	}
	return nil
}
