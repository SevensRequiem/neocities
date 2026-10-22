// Command writer is a loopback app for drafting and publishing stories.
// Drafts stay in writer/drafts/, which is gitignored: the repository is
// public, so nothing unpublished may reach a commit.
package main

import (
	"cmp"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"neocities/internal/site"
)

//go:embed ui.html
var uiHTML string

//go:embed ui.css ui.js
var uiFiles embed.FS

const draftDir = "writer/drafts"

var idRe = regexp.MustCompile(`^[0-9a-f]{16}$`)

// Tags stay the comma list the author typed so autosave never reshapes a
// half-written tag; publish splits them.
type draft struct {
	ID      string `json:"id"`
	Story   int    `json:"story"`
	Title   string `json:"title"`
	Tags    string `json:"tags"`
	Summary string `json:"summary"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Cover   bool   `json:"cover"`
	Rev     int    `json:"rev"`
	Created string `json:"created"`
	Saved   string `json:"saved"`
}

func main() {
	for _, p := range []string{"blog/all.json", "stories/index.json", "stories/all.json", "memos/index.json", "assets/js/marked.min.js"} {
		if _, err := os.Stat(p); err != nil {
			fmt.Fprintln(os.Stderr, "run from the site root: go run ./writer")
			os.Exit(2)
		}
	}
	if _, err := exec.LookPath("magick"); err != nil {
		fmt.Fprintln(os.Stderr, "magick (ImageMagick 7) is required for cover conversion")
		os.Exit(1)
	}
	if err := os.MkdirAll(draftDir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ln, host, token, err := site.Listen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := site.Serve(ln, routes(host, token)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func routes(host, token string) *http.ServeMux {
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
		if err := page.Execute(w, map[string]string{"Token": token}); err != nil {
			slog.Error("render ui", "err", err)
		}
	})
	mux.HandleFunc("GET /library", guard(getLibrary))
	mux.HandleFunc("POST /drafts", guard(postDraft))
	mux.HandleFunc("GET /drafts/{id}", guard(getDraft))
	mux.HandleFunc("PUT /drafts/{id}", guard(putDraft))
	mux.HandleFunc("DELETE /drafts/{id}", guard(deleteDraft))
	mux.HandleFunc("GET /drafts/{id}/cover", guard(getCover))
	mux.HandleFunc("PUT /drafts/{id}/cover", guard(putCover))
	mux.HandleFunc("DELETE /drafts/{id}/cover", guard(deleteCover))
	mux.HandleFunc("POST /drafts/{id}/publish", guard(publishDraft))
	mux.HandleFunc("POST /stories/{id}/edit", guard(editStory))
	mux.HandleFunc("POST /stories/{id}/unpublish", guard(unpublishStory))
	return mux
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func draftPath(id, ext string) string {
	return filepath.Join(draftDir, id+ext)
}

// loadDraft validates the {id} path value before it touches the filesystem,
// so no request can name a file outside writer/drafts/.
func loadDraft(w http.ResponseWriter, r *http.Request) (draft, bool) {
	var d draft
	id := r.PathValue("id")
	if !idRe.MatchString(id) {
		site.Reply(w, http.StatusNotFound, "error", "no such draft")
		return d, false
	}
	raw, err := os.ReadFile(draftPath(id, ".json"))
	if errors.Is(err, fs.ErrNotExist) {
		site.Reply(w, http.StatusNotFound, "error", "no such draft")
		return d, false
	}
	if err == nil {
		err = json.Unmarshal(raw, &d)
	}
	if err != nil {
		site.Fail(w, "read draft", err)
		return d, false
	}
	return d, true
}

func listDrafts() ([]draft, error) {
	paths, err := filepath.Glob(filepath.Join(draftDir, "*.json"))
	if err != nil {
		return nil, err
	}
	drafts := make([]draft, 0, len(paths))
	for _, p := range paths {
		if !idRe.MatchString(strings.TrimSuffix(filepath.Base(p), ".json")) {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var d draft
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		drafts = append(drafts, d)
	}
	slices.SortFunc(drafts, func(a, b draft) int { return cmp.Compare(b.Saved, a.Saved) })
	return drafts, nil
}

// newDraft gives d a fresh random id and writes it.
func newDraft(d draft) (draft, error) {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	now := time.Now().UTC().Format(time.RFC3339)
	d.ID, d.Rev, d.Created, d.Saved = hex.EncodeToString(raw), 1, now, now
	if err := os.MkdirAll(draftDir, 0o700); err != nil {
		return d, err
	}
	return d, site.WriteJSON(draftPath(d.ID, ".json"), d)
}

// readStories returns stories/all.json and stories/index.json as raw bytes
// for rollback and as raw elements, so fields the writer does not know
// survive a rewrite, plus the archive decoded.
func readStories() (archiveRaw, indexRaw []byte, archive, index []json.RawMessage, stories []site.ArchivedStory, err error) {
	if archiveRaw, err = os.ReadFile("stories/all.json"); err != nil {
		return
	}
	if indexRaw, err = os.ReadFile("stories/index.json"); err != nil {
		return
	}
	err = errors.Join(json.Unmarshal(archiveRaw, &archive), json.Unmarshal(archiveRaw, &stories), json.Unmarshal(indexRaw, &index))
	if err == nil && (archive == nil || index == nil) {
		err = errors.New("stories/all.json and stories/index.json must be JSON arrays")
	}
	return
}

// rewrite writes path with the element whose id matches replaced by v, or
// dropped when v is nil.
func rewrite(path string, entries []json.RawMessage, id int, v any) error {
	out := make([]any, 0, len(entries))
	found := false
	for _, e := range entries {
		var head struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(e, &head); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if head.ID != id {
			out = append(out, e)
			continue
		}
		found = true
		if v != nil {
			out = append(out, v)
		}
	}
	if !found {
		return fmt.Errorf("%s: story %d is missing", path, id)
	}
	return site.WriteJSON(path, out)
}

// gitFlags also returns the rev the client last saw, 0 when it sent none.
func gitFlags(r *http.Request) (commit, push bool, rev int) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var in struct {
			Commit, Push bool
			Rev          int
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		return in.Commit, in.Push, in.Rev
	}
	rev, _ = strconv.Atoi(r.FormValue("rev"))
	return r.FormValue("commit") != "", r.FormValue("push") != "", rev
}

func getLibrary(w http.ResponseWriter, r *http.Request) {
	drafts, err := listDrafts()
	if err != nil {
		site.Fail(w, "read drafts", err)
		return
	}
	type row struct {
		ID    string `json:"id"`
		Story int    `json:"story"`
		Title string `json:"title"`
		Saved string `json:"saved"`
		Words int    `json:"words"`
	}
	rows := make([]row, 0, len(drafts))
	for _, d := range drafts {
		rows = append(rows, row{d.ID, d.Story, d.Title, d.Saved, site.Words(d.Body)})
	}
	raw, err := os.ReadFile("stories/index.json")
	if err != nil {
		site.Fail(w, "read story index", err)
		return
	}
	published := []site.StoryEntry{}
	if err := json.Unmarshal(raw, &published); err != nil {
		site.Fail(w, "parse story index", err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"drafts": rows, "published": published})
}

func postDraft(w http.ResponseWriter, r *http.Request) {
	d, err := newDraft(draft{Author: "requiem"})
	if err != nil {
		site.Fail(w, "create draft", err)
		return
	}
	respond(w, http.StatusOK, d)
}

func getDraft(w http.ResponseWriter, r *http.Request) {
	if d, ok := loadDraft(w, r); ok {
		respond(w, http.StatusOK, d)
	}
}

// putDraft is the autosave. rev is a compare-and-swap token: a second tab on
// the same draft saves with an older rev and gets 409 instead of silently
// overwriting the other tab's text.
func putDraft(w http.ResponseWriter, r *http.Request) {
	d, ok := loadDraft(w, r)
	if !ok {
		return
	}
	var in struct {
		Rev                                int
		Title, Tags, Summary, Author, Body string
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		site.Reply(w, http.StatusBadRequest, "error", "expected the draft as JSON")
		return
	}
	if in.Rev != d.Rev {
		respond(w, http.StatusConflict, map[string]any{"error": "this draft was saved from another tab", "rev": d.Rev})
		return
	}
	d.Title, d.Tags, d.Summary, d.Author, d.Body = in.Title, in.Tags, in.Summary, in.Author, in.Body
	d.Rev++
	d.Saved = time.Now().UTC().Format(time.RFC3339)
	if err := site.WriteJSON(draftPath(d.ID, ".json"), d); err != nil {
		site.Fail(w, "save draft", err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"rev": d.Rev, "saved": d.Saved})
}

func deleteDraft(w http.ResponseWriter, r *http.Request) {
	d, ok := loadDraft(w, r)
	if !ok {
		return
	}
	err := os.Remove(draftPath(d.ID, ".webp"))
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	if err = errors.Join(err, os.Remove(draftPath(d.ID, ".json"))); err != nil {
		site.Fail(w, "delete draft", err)
		return
	}
	site.Reply(w, http.StatusOK, "done", "draft deleted")
}

// getCover is guarded like every other draft route because an unpublished
// cover is as private as the text; the UI fetches it into a blob URL.
func getCover(w http.ResponseWriter, r *http.Request) {
	d, ok := loadDraft(w, r)
	if !ok {
		return
	}
	if !d.Cover {
		site.Reply(w, http.StatusNotFound, "error", "no cover")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, draftPath(d.ID, ".webp"))
}

// Covers do not bump rev: the autosave never sends the cover flag, so an
// upload cannot conflict with the text being typed in the same tab.
func putCover(w http.ResponseWriter, r *http.Request) {
	d, ok := loadDraft(w, r)
	if !ok {
		return
	}
	upload, _, err := r.FormFile("image")
	if err != nil {
		site.Reply(w, http.StatusBadRequest, "error", "image is required")
		return
	}
	defer upload.Close()
	if err := site.Convert(r.Context(), upload, draftPath(d.ID, ".webp"), "2000x2000>"); err != nil {
		site.Fail(w, "convert image", err)
		return
	}
	d.Cover = true
	if err := site.WriteJSON(draftPath(d.ID, ".json"), d); err != nil {
		site.Fail(w, "save draft", err)
		return
	}
	site.Reply(w, http.StatusOK, "done", "cover saved")
}

func deleteCover(w http.ResponseWriter, r *http.Request) {
	d, ok := loadDraft(w, r)
	if !ok {
		return
	}
	if err := os.Remove(draftPath(d.ID, ".webp")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		site.Fail(w, "remove cover", err)
		return
	}
	d.Cover = false
	if err := site.WriteJSON(draftPath(d.ID, ".json"), d); err != nil {
		site.Fail(w, "save draft", err)
		return
	}
	site.Reply(w, http.StatusOK, "done", "cover removed")
}

// publishDraft turns a draft into a story, new or an edit of the one it is
// linked to. Any failure before the draft is deleted puts the story files
// and both indexes back, so the draft can simply be published again.
func publishDraft(w http.ResponseWriter, r *http.Request) {
	d, ok := loadDraft(w, r)
	if !ok {
		return
	}
	commit, push, rev := gitFlags(r)
	// A stale rev means the draft changed under the tab, for example an
	// unpublish unlinked it, so "publish update" would silently publish new.
	if rev != 0 && rev != d.Rev {
		respond(w, http.StatusConflict, map[string]any{"error": "this draft changed since it was opened, reload it", "rev": d.Rev})
		return
	}
	title := strings.TrimSpace(d.Title)
	body := strings.TrimRight(strings.ReplaceAll(d.Body, "\r\n", "\n"), " \t\n")
	if title == "" || body == "" {
		site.Reply(w, http.StatusBadRequest, "error", "title and body are required")
		return
	}
	archiveRaw, indexRaw, archive, index, stories, err := readStories()
	if err != nil {
		site.Fail(w, "read stories", err)
		return
	}

	now := time.Now().UTC()
	post := site.StoryPost{
		Title:   title,
		Tags:    site.SplitTags(d.Tags),
		Author:  strings.TrimSpace(d.Author),
		Summary: strings.TrimSpace(d.Summary),
		Body:    body,
	}
	var dir string
	var oldPost, oldImage []byte
	if d.Story == 0 {
		post.ID = 1
		for _, s := range stories {
			post.ID = max(post.ID, s.ID+1)
		}
		post.Date, post.Hex = now.Format(time.RFC3339), site.PostHex(now)
		dir = site.DirName(post.ID, title)
	} else {
		i := slices.IndexFunc(stories, func(s site.ArchivedStory) bool { return s.ID == d.Story })
		if i < 0 {
			site.Reply(w, http.StatusConflict, "error", "the story this draft edits is no longer published")
			return
		}
		old := stories[i]
		if !site.ValidDir(old.Dir) {
			site.Fail(w, "read stories", fmt.Errorf("invalid story directory %q", old.Dir))
			return
		}
		post.ID, post.Date, post.Hex, post.Updated = old.ID, old.Date, old.Hex, now.Format(time.RFC3339)
		dir = old.Dir
		if oldPost, err = os.ReadFile(filepath.Join("stories", dir, "post.json")); err != nil {
			site.Fail(w, "read story", err)
			return
		}
		if oldImage, err = os.ReadFile(filepath.Join("stories", dir, "image.webp")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			site.Fail(w, "read story", err)
			return
		}
	}
	path := filepath.Join("stories", dir)
	undo := func() error {
		err := errors.Join(site.WriteFile("stories/all.json", archiveRaw), site.WriteFile("stories/index.json", indexRaw))
		if d.Story == 0 {
			return errors.Join(err, os.RemoveAll(path))
		}
		err = errors.Join(err, site.WriteFile(filepath.Join(path, "post.json"), oldPost))
		if oldImage != nil {
			return errors.Join(err, site.WriteFile(filepath.Join(path, "image.webp"), oldImage))
		}
		if rmErr := os.Remove(filepath.Join(path, "image.webp")); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	if d.Story == 0 {
		if err := os.Mkdir(path, 0o755); err != nil {
			site.Fail(w, "create story dir", err)
			return
		}
	}

	if d.Cover {
		post.Image = "image.webp"
		var cover []byte
		if cover, err = os.ReadFile(draftPath(d.ID, ".webp")); err == nil {
			err = site.WriteFile(filepath.Join(path, "image.webp"), cover)
		}
	} else if err = os.Remove(filepath.Join(path, "image.webp")); errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	if err == nil {
		err = site.WriteJSON(filepath.Join(path, "post.json"), post)
	}
	entry := site.StoryEntry{ID: post.ID, Dir: dir, Title: title, Date: post.Date, Tags: post.Tags, Summary: post.Summary, Words: site.Words(body), Image: post.Image}
	archived := site.ArchivedStory{StoryPost: post, Dir: dir}
	if err == nil && d.Story == 0 {
		err = site.WriteIndex("stories/all.json", archive, archived)
		if err == nil {
			err = site.WriteIndex("stories/index.json", index, entry)
		}
	} else if err == nil {
		err = rewrite("stories/all.json", archive, post.ID, archived)
		if err == nil {
			err = rewrite("stories/index.json", index, post.ID, entry)
		}
	}
	var pages []string
	if err == nil {
		pages, err = site.BuildSite()
	}
	if err != nil {
		err = errors.Join(err, undo())
		// BuildSite may have written some outputs before failing; rebuilding
		// from the restored data keeps them from linking a story that is gone.
		_, rebuildErr := site.BuildSite()
		site.Fail(w, "publish story", errors.Join(err, rebuildErr))
		return
	}

	for _, ext := range []string{".json", ".webp"} {
		if err := os.Remove(draftPath(d.ID, ext)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Error("remove published draft", "id", d.ID, "err", err)
		}
	}
	paths := append([]string{path, "stories/index.json", "stories/all.json"}, pages...)
	status := site.Publish(r.Context(), commit, push, "story: "+filepath.ToSlash(path), paths...)
	respond(w, http.StatusOK, map[string]any{"done": "published " + filepath.ToSlash(path) + status, "dir": dir, "id": post.ID})
}

func loadStory(w http.ResponseWriter, r *http.Request) (site.ArchivedStory, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		site.Reply(w, http.StatusNotFound, "error", "no such story")
		return site.ArchivedStory{}, false
	}
	_, _, _, _, stories, err := readStories()
	if err != nil {
		site.Fail(w, "read stories", err)
		return site.ArchivedStory{}, false
	}
	i := slices.IndexFunc(stories, func(s site.ArchivedStory) bool { return s.ID == id })
	if i < 0 {
		site.Reply(w, http.StatusNotFound, "error", "no such story")
		return site.ArchivedStory{}, false
	}
	s := stories[i]
	if !site.ValidDir(s.Dir) {
		site.Fail(w, "read stories", fmt.Errorf("invalid story directory %q", s.Dir))
		return s, false
	}
	return s, true
}

// storyDraft copies a published story, cover included, into a new draft.
func storyDraft(s site.ArchivedStory, link int) (draft, error) {
	d, err := newDraft(draft{Story: link, Title: s.Title, Tags: strings.Join(s.Tags, ", "), Summary: s.Summary, Author: s.Author, Body: s.Body})
	if err != nil || s.Image == "" {
		return d, err
	}
	cover, err := os.ReadFile(filepath.Join("stories", s.Dir, filepath.Base(s.Image)))
	if errors.Is(err, fs.ErrNotExist) {
		return d, nil
	}
	if err == nil {
		err = site.WriteFile(draftPath(d.ID, ".webp"), cover)
	}
	if err == nil {
		d.Cover = true
		err = site.WriteJSON(draftPath(d.ID, ".json"), d)
	}
	if err != nil {
		return d, errors.Join(err, os.Remove(draftPath(d.ID, ".json")), os.Remove(draftPath(d.ID, ".webp")))
	}
	return d, nil
}

// editStory reopens the draft already linked to a story, so two edit
// clicks never fork the story into competing drafts.
func editStory(w http.ResponseWriter, r *http.Request) {
	s, ok := loadStory(w, r)
	if !ok {
		return
	}
	drafts, err := listDrafts()
	if err != nil {
		site.Fail(w, "read drafts", err)
		return
	}
	if j := slices.IndexFunc(drafts, func(d draft) bool { return d.Story == s.ID }); j >= 0 {
		respond(w, http.StatusOK, drafts[j])
		return
	}
	d, err := storyDraft(s, s.ID)
	if err != nil {
		site.Fail(w, "create draft", err)
		return
	}
	respond(w, http.StatusOK, d)
}

// unpublishStory moves a story back into a new draft before anything is
// removed, so its text survives every failure path.
func unpublishStory(w http.ResponseWriter, r *http.Request) {
	s, ok := loadStory(w, r)
	if !ok {
		return
	}
	id := s.ID
	commit, push, _ := gitFlags(r)
	archiveRaw, indexRaw, archive, index, _, err := readStories()
	if err != nil {
		site.Fail(w, "read stories", err)
		return
	}
	d, err := storyDraft(s, 0)
	if err != nil {
		site.Fail(w, "create draft", err)
		return
	}
	// The story dir moves into the gitignored drafts dir with one atomic
	// rename, so a failure can put it back whole, and the commit can never
	// record a half-removed dir as still published.
	path := filepath.Join("stories", s.Dir)
	trash := draftPath(d.ID, ".unpublished")
	moved := false
	err = rewrite("stories/all.json", archive, id, nil)
	if err == nil {
		err = rewrite("stories/index.json", index, id, nil)
	}
	if err == nil {
		err = os.Rename(path, trash)
		moved = err == nil
	}
	var pages []string
	if err == nil {
		pages, err = site.BuildSite()
	}
	if err != nil {
		err = errors.Join(err, site.WriteFile("stories/all.json", archiveRaw), site.WriteFile("stories/index.json", indexRaw), os.Remove(draftPath(d.ID, ".json")))
		if rmErr := os.Remove(draftPath(d.ID, ".webp")); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
		if moved {
			err = errors.Join(err, os.Rename(trash, path))
		}
		// BuildSite may have written outputs without the story before failing.
		_, rebuildErr := site.BuildSite()
		site.Fail(w, "unpublish story", errors.Join(err, rebuildErr))
		return
	}
	// The story is already out of the site and the commit; a leftover copy
	// only sits in the gitignored drafts dir.
	if err := os.RemoveAll(trash); err != nil {
		slog.Error("remove unpublished story copy", "path", trash, "err", err)
	}
	// A draft that was editing this story now has nothing to update; it
	// publishes as a new story instead. The rev bump makes a tab that has it
	// open hit a conflict instead of still offering "publish update".
	drafts, err := listDrafts()
	if err != nil {
		slog.Error("read drafts", "err", err)
	}
	for _, linked := range drafts {
		if linked.Story != id {
			continue
		}
		linked.Story = 0
		linked.Rev++
		linked.Saved = time.Now().UTC().Format(time.RFC3339)
		if err := site.WriteJSON(draftPath(linked.ID, ".json"), linked); err != nil {
			slog.Error("unlink draft", "id", linked.ID, "err", err)
		}
	}
	paths := append([]string{path, "stories/index.json", "stories/all.json"}, pages...)
	status := site.Publish(r.Context(), commit, push, "story: unpublish "+filepath.ToSlash(path), paths...)
	respond(w, http.StatusOK, map[string]any{"done": "unpublished " + filepath.ToSlash(path) + status, "draft": d})
}
