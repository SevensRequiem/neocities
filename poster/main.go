package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"syscall"
	"time"
)

//go:embed ui.html
var uiHTML string

var (
	slugRe   = regexp.MustCompile(`[^a-z0-9]+`)
	markupRe = regexp.MustCompile("<[^>]*>|[#*`]")
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

func main() {
	for _, p := range []string{"blog/index.json", "gallery/index.json", "assets/js/marked.min.js"} {
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintln(os.Stderr, "run from the site root: go run ./poster")
			os.Exit(2)
		}
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

	// A page on any origin can POST to a loopback port, so writes need the
	// per-run token in a custom header and the Host must be ours (DNS rebinding).
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Host != host || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Poster-Token")), []byte(token)) != 1 {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
			defer cancel()
			next(w, r.WithContext(ctx))
		}
	}

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.FileServer(http.Dir(".")))
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
		nonce := make([]byte, 16)
		_, _ = rand.Read(nonce)
		n := hex.EncodeToString(nonce)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; style-src 'nonce-"+n+"'; script-src 'self' 'nonce-"+n+"'")
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
		if err := page.Execute(w, map[string]any{"Token": token, "Nonce": n, "Tags": tags}); err != nil {
			slog.Error("render ui", "err", err)
		}
	})
	mux.HandleFunc("POST /blog", guard(postBlog))
	mux.HandleFunc("POST /gallery", guard(postGallery))

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

	md := fmt.Sprintf("---\nid: %d\ntitle: %s\ndate: %s\ntags: %s\nauthor: %s\nhex: %q\nquote: %s\nimage: %s\n---\n\n%s\n",
		id, encode(title), entry.Date, encode(entry.Tags), encode(strings.TrimSpace(r.FormValue("author"))),
		postHex(now), encode(strings.TrimSpace(r.FormValue("quote"))), encode(entry.Image), body)
	js := fmt.Sprintf("window.BLOG_POSTS[%d] = %s;\n", id, encode(body+"\n"))

	err = errors.Join(
		os.WriteFile(filepath.Join(dir, "post.md"), []byte(md), 0o644),
		os.WriteFile(filepath.Join(dir, "post.js"), []byte(js), 0o644),
	)
	if err == nil {
		err = writeIndex("blog/index.json", index, entry)
	}
	if err != nil {
		os.RemoveAll(dir)
		fail(w, "write post", err)
		return
	}
	reply(w, http.StatusOK, "done", "written to "+dir+publish(r, dir, "blog/index.json"))
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

	md := fmt.Sprintf("---\nid: %d\ncaption: %s\ndate: %s\ntags: %s\nhex: %q\nimage: \"image.webp\"\nthumb: \"thumb.webp\"\n---\n",
		id, encode(caption), entry.Date, encode(entry.Tags), postHex(now))
	err = os.WriteFile(filepath.Join(dir, "post.md"), []byte(md), 0o644)
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
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(append([]json.RawMessage{json.RawMessage(encode(entry))}, entries...)); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// encode is json.Marshal without HTML escaping, so &, < and > stay readable
// in the index and post files.
func encode(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(buf.String(), "\n")
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
func publish(r *http.Request, dir, index string) string {
	status := ""
	if r.FormValue("commit") != "" {
		err := git(r.Context(), "add", "--", dir, index)
		if err == nil {
			err = git(r.Context(), "commit", "-m", "post: "+filepath.ToSlash(dir), "--", dir, index)
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
