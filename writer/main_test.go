package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neocities/internal/site"
)

const (
	testHost  = "127.0.0.1:1234"
	testToken = "test-token"
)

// testSite chdirs into an empty site with every file BuildSite reads.
func testSite(t *testing.T) http.Handler {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, dir := range []string{"blog", "stories", "memos", draftDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"blog/index.json", "blog/all.json", "stories/index.json", "stories/all.json", "memos/index.json"} {
		if err := os.WriteFile(p, []byte("[]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return routes(testHost, testToken)
}

func call(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Host = testHost
	req.Header.Set("X-Poster-Token", testToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func decode[T any](t *testing.T, res *httptest.ResponseRecorder, want int) T {
	t.Helper()
	if res.Code != want {
		t.Fatalf("status %d, want %d: %s", res.Code, want, res.Body.String())
	}
	var v T
	if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func readJSON[T any](t *testing.T, path string) T {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// writeDraft creates a draft and saves content into it through the API.
func writeDraft(t *testing.T, h http.Handler, title, body string) draft {
	t.Helper()
	d := decode[draft](t, call(t, h, "POST", "/drafts", nil), http.StatusOK)
	saved := decode[struct{ Rev int }](t, call(t, h, "PUT", "/drafts/"+d.ID, map[string]any{
		"rev": d.Rev, "title": title, "tags": "night, rain , ", "summary": " a summary ", "author": "requiem", "body": body,
	}), http.StatusOK)
	d.Rev = saved.Rev
	return d
}

func TestGuard_RejectsBadTokenAndHost(t *testing.T) {
	h := testSite(t)
	for _, tc := range []struct{ name, method, path, host, token string }{
		{"no token", "POST", "/drafts", testHost, ""},
		{"wrong token", "GET", "/library", testHost, "nope"},
		{"rebound host", "POST", "/drafts", "evil.example:1234", testToken},
		{"ui rebound host", "GET", "/", "evil.example:1234", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = tc.host
			if tc.token != "" {
				req.Header.Set("X-Poster-Token", tc.token)
			}
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)
			if res.Code != http.StatusForbidden {
				t.Fatalf("status %d, want 403", res.Code)
			}
		})
	}
	paths, _ := filepath.Glob(filepath.Join(draftDir, "*"))
	if len(paths) != 0 {
		t.Fatalf("rejected requests wrote %v", paths)
	}
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = testHost
	h.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `content="`+testToken+`"`) {
		t.Fatalf("GET / = %d, token meta missing", res.Code)
	}
	if csp := res.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") || res.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("GET / framing headers: csp %q, x-frame-options %q", csp, res.Header().Get("X-Frame-Options"))
	}
}

func TestDraftID_RejectsTraversal(t *testing.T) {
	h := testSite(t)
	if err := os.WriteFile("secret.json", []byte(`{"id":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"..%2F..%2Fsecret", "..%2Fsecret", "%2e%2e", "0123456789ABCDEF", "0123456789abcde", "0123456789abcdef0", "0123456789abcdeg"} {
		for _, method := range []string{"GET", "PUT", "DELETE"} {
			res := call(t, h, method, "/drafts/"+id, nil)
			if res.Code != http.StatusNotFound {
				t.Errorf("%s /drafts/%s = %d, want 404", method, id, res.Code)
			}
		}
		if res := call(t, h, "POST", "/drafts/"+id+"/publish", nil); res.Code != http.StatusNotFound {
			t.Errorf("publish %s = %d, want 404", id, res.Code)
		}
	}
	if _, err := os.Stat("secret.json"); err != nil {
		t.Fatal(err)
	}
	if res := call(t, h, "GET", "/drafts/0123456789abcdef", nil); res.Code != http.StatusNotFound {
		t.Fatalf("missing draft = %d, want 404", res.Code)
	}
}

func TestAutosave_StaleRevConflicts(t *testing.T) {
	h := testSite(t)
	d := decode[draft](t, call(t, h, "POST", "/drafts", nil), http.StatusOK)
	if !idRe.MatchString(d.ID) || d.Rev != 1 || d.Author != "requiem" {
		t.Fatalf("new draft = %+v", d)
	}
	first := decode[struct {
		Rev   int
		Saved string
	}](t, call(t, h, "PUT", "/drafts/"+d.ID, map[string]any{"rev": 1, "title": "tab one", "body": "one"}), http.StatusOK)
	if first.Rev != 2 || first.Saved == "" {
		t.Fatalf("first save = %+v", first)
	}
	stale := decode[struct {
		Error string
		Rev   int
	}](t, call(t, h, "PUT", "/drafts/"+d.ID, map[string]any{"rev": 1, "title": "tab two", "body": "two"}), http.StatusConflict)
	if stale.Rev != 2 {
		t.Fatalf("conflict rev = %d, want 2", stale.Rev)
	}
	got := decode[draft](t, call(t, h, "GET", "/drafts/"+d.ID, nil), http.StatusOK)
	if got.Title != "tab one" || got.Body != "one" || got.Rev != 2 {
		t.Fatalf("stale save overwrote the draft: %+v", got)
	}
	lib := decode[struct {
		Drafts []struct {
			ID    string
			Title string
			Words int
		}
		Published []site.StoryEntry
	}](t, call(t, h, "GET", "/library", nil), http.StatusOK)
	if len(lib.Drafts) != 1 || lib.Drafts[0].Title != "tab one" || lib.Drafts[0].Words != 1 || lib.Published == nil {
		t.Fatalf("library = %+v", lib)
	}
}

func TestPublish_NewStory(t *testing.T) {
	h := testSite(t)
	if res := call(t, h, "POST", "/drafts/"+decode[draft](t, call(t, h, "POST", "/drafts", nil), http.StatusOK).ID+"/publish", map[string]bool{}); res.Code != http.StatusBadRequest {
		t.Fatalf("empty draft publish = %d, want 400", res.Code)
	}
	d := writeDraft(t, h, "Rain <&> Song", "first scene, five words\n\n---\n\nsecond scene\n\n")
	if err := os.WriteFile(draftPath(d.ID, ".webp"), []byte("webp"), 0o644); err != nil {
		t.Fatal(err)
	}
	stored := readJSON[draft](t, draftPath(d.ID, ".json"))
	stored.Cover = true
	if err := site.WriteJSON(draftPath(d.ID, ".json"), stored); err != nil {
		t.Fatal(err)
	}
	out := decode[struct{ Done, Dir string }](t, call(t, h, "POST", "/drafts/"+d.ID+"/publish", map[string]bool{"commit": false}), http.StatusOK)
	if out.Dir != "001-rain-song" {
		t.Fatalf("dir = %q", out.Dir)
	}
	post := readJSON[site.StoryPost](t, "stories/001-rain-song/post.json")
	if post.ID != 1 || post.Title != "Rain <&> Song" || post.Updated != "" || post.Image != "image.webp" || post.Summary != "a summary" ||
		strings.Join(post.Tags, "|") != "night|rain" || post.Body != "first scene, five words\n\n---\n\nsecond scene" || len(post.Hex) != 24 {
		t.Fatalf("post = %+v", post)
	}
	index := readJSON[[]site.StoryEntry](t, "stories/index.json")
	if len(index) != 1 || index[0].Words != 6 || index[0].Dir != out.Dir || index[0].Date != post.Date {
		t.Fatalf("index = %+v", index)
	}
	all := readJSON[[]site.ArchivedStory](t, "stories/all.json")
	if len(all) != 1 || all[0].Dir != out.Dir || all[0].Body != post.Body {
		t.Fatalf("all = %+v", all)
	}
	for _, p := range []string{"stories/001-rain-song/index.html", "stories/001-rain-song/image.webp", "stories/index.html", "stories.xml"} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	feed, _ := os.ReadFile("feed.xml")
	if !bytes.Contains(feed, []byte("stories/001-rain-song/")) {
		t.Fatal("feed.xml misses the story")
	}
	for _, ext := range []string{".json", ".webp"} {
		if _, err := os.Stat(draftPath(d.ID, ext)); !os.IsNotExist(err) {
			t.Fatalf("draft%s left after publish: %v", ext, err)
		}
	}
	second := writeDraft(t, h, "Next", "more")
	if out := decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+second.ID+"/publish", nil), http.StatusOK); out.Dir != "002-next" {
		t.Fatalf("second dir = %q", out.Dir)
	}
	if index := readJSON[[]site.StoryEntry](t, "stories/index.json"); len(index) != 2 || index[0].ID != 2 {
		t.Fatalf("second story not prepended: %+v", index)
	}
}

func TestPublish_EditKeepsIdentity(t *testing.T) {
	h := testSite(t)
	d := writeDraft(t, h, "Original", "old words here")
	if err := os.WriteFile(draftPath(d.ID, ".webp"), []byte("cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	stored := readJSON[draft](t, draftPath(d.ID, ".json"))
	stored.Cover = true
	if err := site.WriteJSON(draftPath(d.ID, ".json"), stored); err != nil {
		t.Fatal(err)
	}
	decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+d.ID+"/publish", nil), http.StatusOK)
	other := writeDraft(t, h, "Other", "other story")
	decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+other.ID+"/publish", nil), http.StatusOK)
	before := readJSON[site.StoryPost](t, "stories/001-original/post.json")

	edit := decode[draft](t, call(t, h, "POST", "/stories/1/edit", nil), http.StatusOK)
	if edit.Story != 1 || edit.Title != "Original" || edit.Body != "old words here" || !edit.Cover || edit.Tags != "night, rain" {
		t.Fatalf("edit draft = %+v", edit)
	}
	if again := decode[draft](t, call(t, h, "POST", "/stories/1/edit", nil), http.StatusOK); again.ID != edit.ID {
		t.Fatalf("second edit forked a new draft %s, want %s", again.ID, edit.ID)
	}
	if res := call(t, h, "POST", "/stories/99/edit", nil); res.Code != http.StatusNotFound {
		t.Fatalf("edit missing story = %d", res.Code)
	}
	decode[struct{ Rev int }](t, call(t, h, "PUT", "/drafts/"+edit.ID, map[string]any{"rev": edit.Rev, "title": "Renamed", "tags": "x", "body": "new words, now longer than before"}), http.StatusOK)
	decode[map[string]string](t, call(t, h, "DELETE", "/drafts/"+edit.ID+"/cover", nil), http.StatusOK)
	out := decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+edit.ID+"/publish", nil), http.StatusOK)
	if out.Dir != "001-original" {
		t.Fatalf("edit moved the story to %q", out.Dir)
	}
	after := readJSON[site.StoryPost](t, "stories/001-original/post.json")
	if after.ID != 1 || after.Date != before.Date || after.Hex != before.Hex || after.Updated == "" || after.Title != "Renamed" || after.Image != "" {
		t.Fatalf("edited post = %+v, before %+v", after, before)
	}
	if _, err := os.Stat("stories/001-original/image.webp"); !os.IsNotExist(err) {
		t.Fatalf("removed cover still on disk: %v", err)
	}
	index := readJSON[[]site.StoryEntry](t, "stories/index.json")
	if len(index) != 2 || index[0].ID != 2 || index[1].ID != 1 || index[1].Title != "Renamed" || index[1].Words != 6 || index[1].Image != "" {
		t.Fatalf("index after edit = %+v", index)
	}
	all := readJSON[[]site.ArchivedStory](t, "stories/all.json")
	if len(all) != 2 || all[1].Updated != after.Updated || all[1].Dir != "001-original" {
		t.Fatalf("all after edit = %+v", all)
	}
	page, _ := os.ReadFile("stories/001-original/index.html")
	if !bytes.Contains(page, []byte("Renamed")) {
		t.Fatal("story page was not rebuilt")
	}
}

func TestUnpublish_RoundTrip(t *testing.T) {
	h := testSite(t)
	d := writeDraft(t, h, "Gone Soon", "words that come back")
	decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+d.ID+"/publish", nil), http.StatusOK)
	linked := decode[draft](t, call(t, h, "POST", "/stories/1/edit", nil), http.StatusOK)
	indexBefore, _ := os.ReadFile("stories/index.json")
	if !bytes.Contains(indexBefore, []byte("gone-soon")) {
		t.Fatal("story missing before unpublish")
	}

	out := decode[struct {
		Done  string
		Draft draft
	}](t, call(t, h, "POST", "/stories/1/unpublish", map[string]bool{"commit": false}), http.StatusOK)
	if out.Draft.Story != 0 || out.Draft.Title != "Gone Soon" || out.Draft.Body != "words that come back" {
		t.Fatalf("unpublished draft = %+v", out.Draft)
	}
	if _, err := os.Stat("stories/001-gone-soon"); !os.IsNotExist(err) {
		t.Fatalf("story dir still present: %v", err)
	}
	if index := readJSON[[]site.StoryEntry](t, "stories/index.json"); len(index) != 0 {
		t.Fatalf("index = %+v", index)
	}
	if all := readJSON[[]site.ArchivedStory](t, "stories/all.json"); len(all) != 0 {
		t.Fatalf("all = %+v", all)
	}
	if feed, _ := os.ReadFile("feed.xml"); bytes.Contains(feed, []byte("gone-soon")) {
		t.Fatal("feed.xml still lists the story")
	}
	unlinked := readJSON[draft](t, draftPath(linked.ID, ".json"))
	if unlinked.Story != 0 || unlinked.Rev != linked.Rev+1 {
		t.Fatalf("linked draft after unpublish = story %d rev %d, want story 0 rev %d", unlinked.Story, unlinked.Rev, linked.Rev+1)
	}
	if res := call(t, h, "POST", "/drafts/"+linked.ID+"/publish", map[string]int{"rev": linked.Rev}); res.Code != http.StatusConflict {
		t.Fatalf("publish of unlinked draft at its old rev = %d, want 409", res.Code)
	}
	if res := call(t, h, "PUT", "/drafts/"+linked.ID, map[string]any{"rev": linked.Rev, "body": "x"}); res.Code != http.StatusConflict {
		t.Fatalf("autosave of unlinked draft at its old rev = %d, want 409", res.Code)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(draftDir, "*.unpublished")); len(leftovers) != 0 {
		t.Fatalf("unpublished story copy left behind: %v", leftovers)
	}
	if res := call(t, h, "POST", "/stories/1/unpublish", nil); res.Code != http.StatusNotFound {
		t.Fatalf("second unpublish = %d", res.Code)
	}

	back := decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+out.Draft.ID+"/publish", nil), http.StatusOK)
	if back.Dir != "001-gone-soon" {
		t.Fatalf("republished dir = %q", back.Dir)
	}
	if post := readJSON[site.StoryPost](t, "stories/001-gone-soon/post.json"); post.Body != "words that come back" {
		t.Fatalf("republished body = %q", post.Body)
	}
}

// A build failure after the story dir is moved out must put every file back
// and leave no extra draft behind.
func TestUnpublish_BuildFailureRestoresStory(t *testing.T) {
	h := testSite(t)
	d := writeDraft(t, h, "Stays Put", "still here")
	decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+d.ID+"/publish", nil), http.StatusOK)
	before := map[string][]byte{}
	for _, p := range []string{"stories/all.json", "stories/index.json", "stories/001-stays-put/post.json", "stories/001-stays-put/index.html", "feed.xml", "stories.xml"} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		before[p] = raw
	}
	if err := os.WriteFile("memos/index.json", []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if res := call(t, h, "POST", "/stories/1/unpublish", nil); res.Code != http.StatusInternalServerError {
		t.Fatalf("unpublish with a broken build = %d, want 500", res.Code)
	}
	for p, want := range before {
		if got, err := os.ReadFile(p); err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s not restored: %v", p, err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(draftDir, "*")); len(left) != 0 {
		t.Fatalf("failed unpublish left drafts: %v", left)
	}
}

func TestPublish_StaleRevConflicts(t *testing.T) {
	h := testSite(t)
	d := writeDraft(t, h, "Racing", "two tabs")
	if res := call(t, h, "POST", "/drafts/"+d.ID+"/publish", map[string]int{"rev": d.Rev - 1}); res.Code != http.StatusConflict {
		t.Fatalf("stale publish = %d, want 409", res.Code)
	}
	if _, err := os.Stat("stories/001-racing"); !os.IsNotExist(err) {
		t.Fatalf("stale publish wrote the story: %v", err)
	}
	decode[struct{ Dir string }](t, call(t, h, "POST", "/drafts/"+d.ID+"/publish", map[string]int{"rev": d.Rev}), http.StatusOK)
}

func TestDeleteDraft(t *testing.T) {
	h := testSite(t)
	d := writeDraft(t, h, "Scrap", "bin it")
	if err := os.WriteFile(draftPath(d.ID, ".webp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	decode[map[string]string](t, call(t, h, "DELETE", "/drafts/"+d.ID, nil), http.StatusOK)
	for _, ext := range []string{".json", ".webp"} {
		if _, err := os.Stat(draftPath(d.ID, ext)); !os.IsNotExist(err) {
			t.Fatalf("draft%s survived delete: %v", ext, err)
		}
	}
	if res := call(t, h, "DELETE", "/drafts/"+d.ID, nil); res.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d", res.Code)
	}
}
