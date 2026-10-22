// Package site holds what the poster and the writer share: the data files,
// the static build, the loopback server guard and the git publish step.
package site

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	slugRe = regexp.MustCompile(`[^a-z0-9]+`)
	dirRe  = regexp.MustCompile(`^[0-9]{3,}-[a-z0-9]+(-[a-z0-9]+)*$`)
)

type BlogEntry struct {
	ID      int      `json:"id"`
	Dir     string   `json:"dir"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Excerpt string   `json:"excerpt"`
	Image   string   `json:"image"`
}

type BlogPost struct {
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

// ArchivedPost is one element of blog/all.json.
type ArchivedPost struct {
	BlogPost
	Dir string `json:"dir"`
}

// StoryPost is stories/<dir>/post.json.
type StoryPost struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Updated string   `json:"updated,omitempty"`
	Tags    []string `json:"tags"`
	Author  string   `json:"author"`
	Hex     string   `json:"hex"`
	Summary string   `json:"summary"`
	Image   string   `json:"image"`
	Body    string   `json:"body"`
}

// ArchivedStory is one element of stories/all.json.
type ArchivedStory struct {
	StoryPost
	Dir string `json:"dir"`
}

// StoryEntry is one element of stories/index.json.
type StoryEntry struct {
	ID      int      `json:"id"`
	Dir     string   `json:"dir"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Tags    []string `json:"tags"`
	Summary string   `json:"summary"`
	Words   int      `json:"words"`
	Image   string   `json:"image"`
}

// Memo is one element of memos/index.json. HTML and Videos are derived from
// Body by BuildSite, so hand edits only need to touch Body.
type Memo struct {
	ID     int     `json:"id"`
	Date   string  `json:"date"`
	Body   string  `json:"body"`
	HTML   string  `json:"html"`
	Videos []Video `json:"videos,omitempty"`
}

// Video is a YouTube video a memo links to. ID is checked against the 11
// character id alphabet because site.js puts it into thumbnail and embed URLs.
type Video struct {
	ID    string `json:"id"`
	Start int    `json:"start,omitempty"`
}

// ValidDir reports whether dir has the shape DirName produces. Dirs are joined
// under blog/ or stories/ and removed on unpublish, so "." or ".." from a hand
// edited index must never get that far.
func ValidDir(dir string) bool {
	return dirRe.MatchString(dir)
}

// Convert re-encodes a raster image as webp, dropping EXIF (camera,
// timestamp, GPS) and shrinking it to fit the given geometry.
func Convert(ctx context.Context, src io.Reader, dst, geometry string) error {
	// ImageMagick picks its decoder from content, which would expose the SVG,
	// MVG, PostScript and URL coders to any upload; the sniffed coder is passed
	// explicitly instead.
	head := make([]byte, 12)
	n, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return err
	}
	head = head[:n]
	brand := ""
	if n == 12 && string(head[4:8]) == "ftyp" {
		brand = string(head[8:12])
	}
	var coder string
	switch {
	case bytes.HasPrefix(head, []byte("\xff\xd8\xff")):
		coder = "jpeg"
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		coder = "png"
	case bytes.HasPrefix(head, []byte("GIF8")):
		coder = "gif"
	case n == 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		coder = "webp"
	case bytes.HasPrefix(head, []byte("BM")):
		coder = "bmp"
	case bytes.HasPrefix(head, []byte("II*\x00")), bytes.HasPrefix(head, []byte("MM\x00*")):
		coder = "tiff"
	case brand == "avif" || brand == "avis":
		coder = "avif"
	case slices.Contains([]string{"heic", "heix", "heim", "heis", "hevc", "mif1", "msf1"}, brand):
		coder = "heic"
	default:
		return errors.New("unsupported image type: use jpeg, png, gif, webp, bmp, tiff, avif or heic")
	}
	tmp, err := os.CreateTemp("", "poster-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, io.MultiReader(bytes.NewReader(head), src))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	// A tiny header can declare a huge canvas; the limits stop magick from
	// filling memory or the shared /tmp quota with its pixel cache.
	out, err := exec.CommandContext(ctx, "magick", "-limit", "memory", "256MiB", "-limit", "disk", "1GiB", "-limit", "width", "16KP", "-limit", "height", "16KP", "-limit", "area", "128MP", coder+":"+tmp.Name(), "-auto-orient", "-strip", "-resize", geometry, "-quality", "82", "webp:"+dst).CombinedOutput()
	if err != nil {
		return fmt.Errorf("magick: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// ReadIndex returns the index entries and the next free id. Entries stay raw
// so fields the binaries do not know about survive a rewrite.
func ReadIndex(path string) ([]json.RawMessage, int, error) {
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

// WriteIndex writes entry followed by entries, newest first.
func WriteIndex(path string, entries []json.RawMessage, entry any) error {
	all := []any{entry}
	for _, e := range entries {
		all = append(all, e)
	}
	return WriteJSON(path, all)
}

// WriteJSON skips HTML escaping so &, < and > stay readable in the index and
// post files.
func WriteJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return WriteFile(path, buf.Bytes())
}

// WriteFile replaces path atomically through a sibling temporary file.
func WriteFile(path string, data []byte) error {
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

func DirName(id int, title string) string {
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if slug == "" {
		slug = "untitled"
	}
	return fmt.Sprintf("%03d-%s", id, slug)
}

func SplitTags(s string) []string {
	tags := []string{}
	for t := range strings.SplitSeq(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// PostHex mirrors the Mongo ObjectID the old site stored in each post's hex
// field: a 4-byte timestamp followed by 8 random bytes.
func PostHex(now time.Time) string {
	oid := make([]byte, 12)
	binary.BigEndian.PutUint32(oid, uint32(now.Unix()))
	_, _ = rand.Read(oid[4:])
	return hex.EncodeToString(oid)
}

func Reply(w http.ResponseWriter, status int, key, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{key: value})
}

func Fail(w http.ResponseWriter, stage string, err error) {
	slog.Error(stage, "err", err)
	Reply(w, http.StatusInternalServerError, "error", stage+" failed, see the terminal")
}

// Publish runs the git steps the author ticked. The content is already on
// disk, so a git failure is reported in the returned status text instead of
// failing the request.
func Publish(ctx context.Context, commit, push bool, message string, paths ...string) string {
	status := ""
	if commit {
		paths = slices.Compact(slices.Sorted(slices.Values(paths)))
		// An unpublished story's directory is gone: git still needs it when it
		// was tracked, and rejects the whole pathspec when it never was.
		staged := make([]string, 0, len(paths))
		for _, p := range paths {
			if _, err := os.Lstat(p); err == nil {
				staged = append(staged, p)
				continue
			}
			if out, err := git(ctx, "ls-files", "--", p); err == nil && len(out) > 0 {
				staged = append(staged, p)
			}
		}
		_, err := git(ctx, append([]string{"add", "--"}, staged...)...)
		if err == nil {
			_, err = git(ctx, append([]string{"commit", "-m", message, "--"}, staged...)...)
		}
		if err != nil {
			slog.Error("git commit", "err", err)
			return " :: commit failed, see the terminal"
		}
		status = " :: committed"
	}
	if !push {
		return status
	}
	if _, err := git(ctx, "push", "-u", "origin", "HEAD"); err != nil {
		slog.Error("git push", "err", err)
		return status + " :: push failed, see the terminal"
	}
	return status + " :: pushed"
}

func git(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	// Without a terminal a credential or passphrase prompt would hang until the deadline.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s", args[0], err, bytes.TrimSpace(out))
	}
	return out, nil
}

// Guard wraps every writing route. A page on any origin can send requests to
// a loopback port, so writes need the per-run token in a custom header and
// the Host must be ours (DNS rebinding).
func Guard(host, token string) func(http.HandlerFunc) http.HandlerFunc {
	var writes sync.Mutex
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Host != host || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Poster-Token")), []byte(token)) != 1 {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			// Index read-modify-write, the build outputs and git's staging area
			// are shared by every route here and by the other binary: the mutex
			// orders this process, the flock orders the poster against the writer.
			writes.Lock()
			defer writes.Unlock()
			lock, err := LockSite()
			if err != nil {
				Fail(w, "lock site", err)
				return
			}
			defer lock.Close()
			r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
			defer cancel()
			next(w, r.WithContext(ctx))
		}
	}
}

// LockSite takes the cross-process site lock that orders the poster, the
// writer and `-build`; closing the file releases it.
func LockSite() (*os.File, error) {
	lock, err := os.OpenFile(".site.lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
}

// Listen opens a loopback port and returns it with its host:port and a fresh
// per-run token for Guard.
func Listen() (net.Listener, string, string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", "", err
	}
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	return ln, ln.Addr().String(), hex.EncodeToString(secret), nil
}

// Serve prints the app's URL, opens it in a browser and serves until
// SIGINT or SIGTERM.
func Serve(ln net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	url := "http://" + ln.Addr().String() + "/"
	fmt.Println(url)
	_ = exec.Command("xdg-open", url).Start()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Mount serves the deployable site under /site/ and the shared scripts under
// /assets/. The homepage fetches its indexes, which browsers refuse on
// file://, so both apps serve it for local viewing.
func Mount(mux *http.ServeMux) {
	mux.Handle("GET /assets/", http.FileServer(http.Dir(".")))
	files := http.StripPrefix("/site", http.FileServer(http.Dir(".")))
	for _, dir := range []string{"assets", "blog", "gallery", "stories", "memos"} {
		mux.Handle("GET /site/"+dir+"/", files)
	}
	mux.HandleFunc("GET /site/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	for _, file := range []string{"shrine.html", "feed.xml", "stories.xml", "sitemap.xml", "llms.txt"} {
		mux.Handle("GET /site/"+file, files)
	}
}
