package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"neocities/internal/site"
)

const testIndex = `[
  {"id":2,"dir":"002-newer","title":"newer","excerpt":"short","future":{"n":9007199254740993}},
  {"id":7,"dir":"007-older","title":"older","excerpt":"older short"}
]
`

const testArchive = `[
  {"id":2,"dir":"002-newer","title":"newer","date":"2026-01-02T00:00:00Z","tags":["old"],"author":"writer","hex":"old-hex","quote":"old quote","image":"image.webp","body":"# Full older body\n\n<&>","future":{"n":9007199254740993,"values":[null,true]}},
  {"id":7,"dir":"007-older","title":"older","date":"2026-01-01T00:00:00Z","tags":[],"author":"","hex":"older-hex","quote":"","image":"","body":"other full body"}
]
`

func TestPostBlogPrependsFullArchive(t *testing.T) {
	for _, tc := range []struct {
		name    string
		index   string
		archive string
		firstID int
	}{
		{"empty", "[]\n", "[]\n", 1},
		{"existing", testIndex, testArchive, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.Mkdir("blog", 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, "blog/index.json", tc.index)
			testWrite(t, "blog/all.json", tc.archive)
			testPostDirs(t)
			for n, title := range []string{"First <&> 雪", "Second post"} {
				oldIndex := testReadJSON[[]json.RawMessage](t, "blog/index.json")
				oldArchive := testReadJSON[[]json.RawMessage](t, "blog/all.json")
				body := "# Heading <&> 雪\n\n" + strings.Repeat("**full markdown** ", 40) + "\n[link](https://example.com/)"
				res := httptest.NewRecorder()
				postBlog(res, blogRequest(t, map[string]string{
					"title": "  " + title + "  ", "body": strings.ReplaceAll(body, "\n", "\r\n") + "\r\n \t",
					"tags": "Tech, 雪, , Life", "author": " Writer ", "quote": " quoted <&> ",
				}))
				if res.Code != http.StatusOK {
					t.Fatalf("POST /blog: %d %s", res.Code, res.Body.String())
				}
				index := testReadJSON[[]json.RawMessage](t, "blog/index.json")
				archive := testReadJSON[[]json.RawMessage](t, "blog/all.json")
				if len(index) != len(oldIndex)+1 || len(archive) != len(oldArchive)+1 {
					t.Fatalf("index/archive lengths = %d/%d, want %d/%d", len(index), len(archive), len(oldIndex)+1, len(oldArchive)+1)
				}
				for i := range oldIndex {
					testEqualJSON(t, index[i+1], oldIndex[i])
					testEqualJSON(t, archive[i+1], oldArchive[i])
				}
				var entry site.BlogEntry
				if err := json.Unmarshal(index[0], &entry); err != nil {
					t.Fatal(err)
				}
				if entry.ID != tc.firstID+n || entry.Title != title || entry.Excerpt == body {
					t.Fatalf("unexpected index entry: %+v", entry)
				}
				post := testReadJSON[map[string]json.RawMessage](t, filepath.Join("blog", entry.Dir, "post.json"))
				if len(post) != 9 {
					t.Fatalf("post fields = %d, want 9: %v", len(post), post)
				}
				want := map[string]any{"id": entry.ID, "title": title, "date": entry.Date, "tags": []string{"Tech", "雪", "Life"}, "author": "Writer", "quote": "quoted <&>", "body": body, "image": ""}
				for key, value := range want {
					raw, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					testEqualJSON(t, post[key], raw)
				}
				if _, err := time.Parse(time.RFC3339, entry.Date); err != nil {
					t.Fatal(err)
				}
				var postID string
				if err := json.Unmarshal(post["hex"], &postID); err != nil {
					t.Fatal(err)
				}
				if decoded, err := hex.DecodeString(postID); err != nil || len(decoded) != 12 {
					t.Fatalf("invalid post hex %q: %v", postID, err)
				}
				post["dir"], _ = json.Marshal(entry.Dir)
				var archived map[string]json.RawMessage
				if err := json.Unmarshal(archive[0], &archived); err != nil {
					t.Fatal(err)
				}
				if len(archived) != len(post) {
					t.Fatalf("archive fields = %d, want %d: %v", len(archived), len(post), archived)
				}
				for key, raw := range post {
					testEqualJSON(t, archived[key], raw)
				}
			}
		})
	}
}

func TestPostBlogRejectsInvalidArchive(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"missing", ""},
		{"truncated", "[{"},
		{"object", "{}"},
		{"null", "null"},
		{"null entry", "[null]"},
		{"array entry", "[[]]"},
		{"number entry", "[1]"},
		{"string entry", `["post"]`},
		{"trailing data", "[] {}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.Mkdir("blog", 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, "blog/index.json", testIndex)
			if tc.raw != "" {
				testWrite(t, "blog/all.json", tc.raw)
			}
			before := testTree(t, "blog")
			res := httptest.NewRecorder()
			postBlog(res, blogRequest(t, map[string]string{"title": "Invalid archive", "body": "body", "commit": "on"}))
			if res.Code != http.StatusInternalServerError {
				t.Errorf("POST /blog: %d %s", res.Code, res.Body.String())
			}
			if after := testTree(t, "blog"); !reflect.DeepEqual(after, before) {
				t.Errorf("failed request mutated blog: got %v, want %v", after, before)
			}
			if strings.Contains(res.Body.String(), tc.raw) && tc.raw != "" {
				t.Errorf("response leaked archive content: %s", res.Body.String())
			}
		})
	}
}

func TestPostBlogWriteFailureRollsBack(t *testing.T) {
	for _, blocked := range []string{"all.json.tmp", "index.json.tmp"} {
		t.Run(blocked, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.MkdirAll(filepath.Join("blog", blocked), 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, "blog/index.json", testIndex)
			testWrite(t, "blog/all.json", testArchive)
			testWrite(t, filepath.Join("blog", blocked, "keep"), "existing file")
			before := testTree(t, "blog")
			res := httptest.NewRecorder()
			postBlog(res, blogRequest(t, map[string]string{"title": "Write failure", "body": "full body", "commit": "on"}))
			if res.Code != http.StatusInternalServerError {
				t.Errorf("POST /blog: %d %s", res.Code, res.Body.String())
			}
			if after := testTree(t, "blog"); !reflect.DeepEqual(after, before) {
				t.Errorf("failed write did not restore original files")
			}
		})
	}
}

func TestPostBlogCommitCheckbox(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	for _, commit := range []bool{false, true} {
		name := "unchecked"
		if commit {
			name = "checked"
		}
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			testGit(t, "init", "--quiet")
			for _, identity := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
				if out, err := exec.CommandContext(t.Context(), "git", "var", identity).CombinedOutput(); err != nil {
					t.Skipf("inherited git identity unavailable: %v: %s", err, out)
				}
			}
			if err := os.Mkdir("blog", 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, "blog/index.json", testIndex)
			testWrite(t, "blog/all.json", testArchive)
			testPostDirs(t)
			testWrite(t, "unrelated.txt", "original\n")
			testGit(t, "add", "--", "blog", "unrelated.txt")
			testGit(t, "commit", "-m", "test: seed poster repository")
			before := testGit(t, "rev-parse", "HEAD")
			testWrite(t, "unrelated.txt", "staged change\n")
			testGit(t, "add", "--", "unrelated.txt")
			testWrite(t, "unrelated.txt", "unstaged change\n")
			staged := testGit(t, "diff", "--cached", "--", "unrelated.txt")
			fields := map[string]string{"title": "Git post", "body": "# Full git post\n\nbody"}
			if commit {
				fields["commit"] = "on"
			}
			res := httptest.NewRecorder()
			postBlog(res, blogRequest(t, fields))
			if res.Code != http.StatusOK || strings.Contains(res.Body.String(), "failed") {
				t.Fatalf("POST /blog: %d %s", res.Code, res.Body.String())
			}
			if got := testGit(t, "diff", "--cached", "--", "unrelated.txt"); got != staged {
				t.Error("unrelated staged changes were modified")
			}
			if got, err := os.ReadFile("unrelated.txt"); err != nil || string(got) != "unstaged change\n" {
				t.Fatalf("unrelated working tree changed: %q, %v", got, err)
			}
			if !commit {
				if got := testGit(t, "rev-parse", "HEAD"); got != before {
					t.Error("unchecked commit created a commit")
				}
				if got := testGit(t, "diff", "--cached", "--name-only"); got != "unrelated.txt\n" {
					t.Errorf("unchecked commit staged files: %s", got)
				}
				if strings.Contains(res.Body.String(), "::") {
					t.Errorf("unchecked commit ran git: %s", res.Body.String())
				}
				return
			}
			if got := testGit(t, "log", "-1", "--format=%s"); got != "post: blog/008-git-post\n" {
				t.Errorf("commit subject = %q", got)
			}
			paths := "blog/002-newer/index.html\nblog/007-older/index.html\nblog/008-git-post/index.html\nblog/008-git-post/post.json\nblog/all.json\nblog/index.html\nblog/index.json\nfeed.xml\nllms.txt\nsitemap.xml\nstories.xml\nstories/index.html\n"
			if got := testGit(t, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"); got != paths {
				t.Errorf("committed paths = %q, want %q", got, paths)
			}
			for _, path := range strings.Fields(paths) {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if got := testGit(t, "show", "HEAD:"+path); got != string(raw) {
					t.Errorf("commit did not include current %s", path)
				}
			}
			if got := testGit(t, "status", "--porcelain", "--", "blog"); got != "" {
				t.Errorf("uncommitted blog files: %s", got)
			}
		})
	}
}

func TestPostBlogPublishWithoutCommit(t *testing.T) {
	for _, tc := range []struct{ name, push, status string }{
		{"no git", "", "written to blog/001-no-commit"},
		{"push only", "on", "written to blog/001-no-commit :: push failed, see the terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())
			if err := os.Mkdir("blog", 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, "blog/index.json", "[]\n")
			testWrite(t, "blog/all.json", "[]\n")
			testPostDirs(t)
			res := httptest.NewRecorder()
			postBlog(res, blogRequest(t, map[string]string{"title": "No commit", "body": "body", "push": tc.push}))
			var reply map[string]string
			if err := json.Unmarshal(res.Body.Bytes(), &reply); err != nil {
				t.Fatal(err)
			}
			if res.Code != http.StatusOK || reply["done"] != tc.status {
				t.Fatalf("publish status = %d %s, want %q", res.Code, res.Body.String(), tc.status)
			}
		})
	}
}

func TestPostBlogConcurrentRequests(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("blog", 0o755); err != nil {
		t.Fatal(err)
	}
	testWrite(t, "blog/index.json", "[]\n")
	testWrite(t, "blog/all.json", "[]\n")
	testPostDirs(t)
	guard := site.Guard("example.com", "test-token")
	handler := guard(postBlog)
	const count = 12
	responses := make([]*httptest.ResponseRecorder, count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for n := range count {
		responses[n] = httptest.NewRecorder()
		req := blogRequest(t, map[string]string{"title": fmt.Sprintf("Concurrent %d", n), "body": "full body"})
		req.Header.Set("X-Poster-Token", "test-token")
		workers.Go(func() {
			<-start
			handler(responses[n], req)
		})
	}
	close(start)
	workers.Wait()
	for _, res := range responses {
		if res.Code != http.StatusOK {
			t.Errorf("concurrent POST /blog: %d %s", res.Code, res.Body.String())
		}
	}
	index := testReadJSON[[]site.BlogEntry](t, "blog/index.json")
	if len(index) != count {
		t.Fatalf("index entries = %d, want %d", len(index), count)
	}
	for n, entry := range index {
		if entry.ID != count-n {
			t.Errorf("index[%d].ID = %d, want %d", n, entry.ID, count-n)
		}
	}
	testArchiveMatchesPosts(t, "blog")
}

func TestPostImageMetadata(t *testing.T) {
	if _, err := exec.LookPath("magick"); err != nil {
		t.Skipf("ImageMagick unavailable: %v", err)
	}
	for _, section := range []string{"blog", "gallery"} {
		t.Run(section, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.Mkdir(section, 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, filepath.Join(section, "index.json"), "[]\n")
			if section == "blog" {
				testWrite(t, "blog/all.json", "[]\n")
				testPostDirs(t)
			}
			var upload bytes.Buffer
			if err := png.Encode(&upload, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
				t.Fatal(err)
			}
			req := blogRequest(t, map[string]string{"title": "Image post", "caption": "Image post", "body": "image body"}, upload.Bytes())
			req.URL.Path = "/" + section
			res := httptest.NewRecorder()
			if section == "blog" {
				postBlog(res, req)
			} else {
				postGallery(res, req)
			}
			if res.Code != http.StatusOK {
				t.Fatalf("POST /%s: %d %s", section, res.Code, res.Body.String())
			}
			post := testReadJSON[map[string]json.RawMessage](t, filepath.Join(section, "001-image-post/post.json"))
			testEqualJSON(t, post["image"], []byte(`"image.webp"`))
			images := []string{"image.webp"}
			if section == "blog" {
				testArchiveMatchesPosts(t, "blog")
			} else {
				testEqualJSON(t, post["thumb"], []byte(`"thumb.webp"`))
				images = append(images, "thumb.webp")
				if _, err := os.Stat("gallery/all.json"); !os.IsNotExist(err) {
					t.Fatalf("gallery unexpectedly has an archive: %v", err)
				}
			}
			for _, image := range images {
				raw, err := os.ReadFile(filepath.Join(section, "001-image-post", image))
				if err != nil || len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WEBP" {
					t.Errorf("invalid generated webp %s: %v", image, err)
				}
			}
		})
	}
}

func TestPosterGuardRejectsUnauthorizedWrites(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("blog", 0o755); err != nil {
		t.Fatal(err)
	}
	testWrite(t, "blog/index.json", testIndex)
	testWrite(t, "blog/all.json", testArchive)
	before := testTree(t, "blog")
	handler := site.Guard("example.com", "test-token")(postBlog)
	for _, tc := range []struct{ host, token string }{
		{"attacker.example", "test-token"},
		{"example.com", "wrong-token"},
		{"example.com", ""},
	} {
		req := blogRequest(t, map[string]string{"title": "Unauthorized", "body": "body"})
		req.Host = tc.host
		req.Header.Set("X-Poster-Token", tc.token)
		res := httptest.NewRecorder()
		handler(res, req)
		if res.Code != http.StatusForbidden {
			t.Errorf("unauthorized request: %d %s", res.Code, res.Body.String())
		}
	}
	if after := testTree(t, "blog"); !reflect.DeepEqual(after, before) {
		t.Error("unauthorized request changed the blog")
	}
}

func TestProxyQA(t *testing.T) {
	var got struct{ method, path, auth, body string }
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got.method, got.path, got.auth, got.body = r.Method, r.URL.Path, r.Header.Get("Authorization"), string(body)
		if r.URL.Path == "/qa/9" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"done":"answered"}`)
	}))
	defer api.Close()
	t.Setenv("QA_API", api.URL)

	t.Setenv("QA_TOKEN", "")
	res := httptest.NewRecorder()
	proxyQA(res, httptest.NewRequest("GET", "/qa/inbox", nil))
	if res.Code != http.StatusServiceUnavailable || got.method != "" {
		t.Fatalf("without a token: %d %s, api saw %q", res.Code, res.Body.String(), got.method)
	}

	t.Setenv("QA_TOKEN", "secret")
	res = httptest.NewRecorder()
	proxyQA(res, httptest.NewRequest("PUT", "/qa/3", strings.NewReader(`{"answer":"hi"}`)))
	if res.Code != http.StatusOK || res.Body.String() != `{"done":"answered"}` {
		t.Errorf("answer: %d %s", res.Code, res.Body.String())
	}
	if got.method != "PUT" || got.path != "/qa/3" || got.auth != "Bearer secret" || got.body != `{"answer":"hi"}` {
		t.Errorf("api saw %+v", got)
	}

	res = httptest.NewRecorder()
	proxyQA(res, httptest.NewRequest("DELETE", "/qa/9", nil))
	var out map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil || res.Code != http.StatusBadGateway || out["error"] != "api answered 404: not found" {
		t.Errorf("api error: %d %s", res.Code, res.Body.String())
	}
}

func TestRepositoryBlogArchive(t *testing.T) {
	testArchiveMatchesPosts(t, "../blog")
}

func testArchiveMatchesPosts(t *testing.T, root string) {
	t.Helper()
	index := testReadJSON[[]site.BlogEntry](t, filepath.Join(root, "index.json"))
	archive := testReadJSON[[]map[string]json.RawMessage](t, filepath.Join(root, "all.json"))
	paths, err := filepath.Glob(filepath.Join(root, "*", "post.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive) != len(index) || len(archive) != len(paths) {
		t.Fatalf("archive/index/post counts = %d/%d/%d", len(archive), len(index), len(paths))
	}
	seen := map[string]bool{}
	for n, entry := range index {
		if entry.Dir == "" || filepath.Base(entry.Dir) != entry.Dir || !filepath.IsLocal(entry.Dir) || seen[entry.Dir] {
			t.Fatalf("invalid or duplicate post directory %q", entry.Dir)
		}
		seen[entry.Dir] = true
		post := testReadJSON[map[string]json.RawMessage](t, filepath.Join(root, entry.Dir, "post.json"))
		post["dir"], _ = json.Marshal(entry.Dir)
		if len(archive[n]) != len(post) {
			t.Errorf("archive[%d] fields = %d, want %d", n, len(archive[n]), len(post))
		}
		if _, ok := archive[n]["excerpt"]; ok {
			t.Errorf("archive[%d] has an excerpt", n)
		}
		for key, raw := range post {
			testEqualJSON(t, archive[n][key], raw)
		}
	}
	t.Logf("verified %d complete posts in index order", len(archive))
}

func testGit(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func testTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[path] = "<directory>"
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func blogRequest(t *testing.T, fields map[string]string, images ...[]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	for _, image := range images {
		part, err := form.CreateFormFile("image", "upload.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(image); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/blog", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

// testPostDirs creates the directories of the posts in testArchive and the
// empty story and memo files, which a real site always has and the page
// build reads and writes into.
func testPostDirs(t *testing.T) {
	t.Helper()
	for _, dir := range []string{"blog/002-newer", "blog/007-older", "stories", "memos"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	testWrite(t, "stories/index.json", "[]\n")
	testWrite(t, "stories/all.json", "[]\n")
	testWrite(t, "memos/index.json", "[]\n")
}

func testWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testReadJSON[T any](t *testing.T, path string) T {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func testEqualJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var values [2]any
	for i, raw := range [][]byte{got, want} {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&values[i]); err != nil {
			t.Fatalf("decode JSON %q: %v", raw, err)
		}
	}
	if !reflect.DeepEqual(values[0], values[1]) {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestPostMemo_Validation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"empty", "", http.StatusBadRequest},
		{"whitespace", " \r\n\t ", http.StatusBadRequest},
		{"one", "x", http.StatusOK},
		{"at limit", strings.Repeat("a", 1000), http.StatusOK},
		{"at limit multibyte", strings.Repeat("雪", 1000), http.StatusOK},
		{"at limit after trim", "  " + strings.Repeat("a", 1000) + "\n\n", http.StatusOK},
		{"over limit", strings.Repeat("a", 1001), http.StatusBadRequest},
		{"over limit multibyte", strings.Repeat("雪", 1001), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.Mkdir("blog", 0o755); err != nil {
				t.Fatal(err)
			}
			testWrite(t, "blog/index.json", "[]\n")
			testWrite(t, "blog/all.json", "[]\n")
			testPostDirs(t)
			res := httptest.NewRecorder()
			postMemo(res, blogRequest(t, map[string]string{"body": tc.body}))
			if res.Code != tc.want {
				t.Fatalf("POST /memo: %d %s, want %d", res.Code, res.Body.String(), tc.want)
			}
			memos := testReadJSON[[]site.Memo](t, "memos/index.json")
			if tc.want != http.StatusOK {
				if len(memos) != 0 {
					t.Errorf("rejected memo was stored: %+v", memos)
				}
				return
			}
			if len(memos) != 1 || memos[0].Body != strings.TrimSpace(tc.body) {
				t.Errorf("memos = %+v", memos)
			}
		})
	}
}

func TestPostMemo_PrependsSanitized(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("blog", 0o755); err != nil {
		t.Fatal(err)
	}
	testWrite(t, "blog/index.json", "[]\n")
	testWrite(t, "blog/all.json", "[]\n")
	testPostDirs(t)
	testWrite(t, "memos/index.json", `[{"id":4,"date":"2026-01-01T00:00:00Z","body":"older","html":"<p>older</p>\n"}]`)
	for _, body := range []string{"first\r\nline", "**loud** <script>alert(1)</script><img src=x onerror=alert(2)> [x](https://example.com/)"} {
		res := httptest.NewRecorder()
		postMemo(res, blogRequest(t, map[string]string{"body": body}))
		if res.Code != http.StatusOK {
			t.Fatalf("POST /memo: %d %s", res.Code, res.Body.String())
		}
	}
	memos := testReadJSON[[]site.Memo](t, "memos/index.json")
	if len(memos) != 3 || memos[0].ID != 6 || memos[1].ID != 5 || memos[2].ID != 4 {
		t.Fatalf("memos = %+v", memos)
	}
	if memos[1].Body != "first\nline" || memos[1].HTML != "<p>first\nline</p>\n" {
		t.Errorf("memo 5 = %+v", memos[1])
	}
	if html := memos[0].HTML; strings.Contains(html, "script") || strings.Contains(html, "onerror") || strings.Contains(html, "alert") ||
		!strings.Contains(html, "<strong>loud</strong>") || !strings.Contains(html, `<a href="https://example.com/" rel="noreferrer noopener" target="_blank">x</a>`) {
		t.Errorf("memo html not sanitized: %q", html)
	}
	for _, m := range memos[:2] {
		if date, err := time.Parse(time.RFC3339, m.Date); err != nil || date.Location() != time.UTC || time.Since(date) > time.Minute {
			t.Errorf("memo %d date %q: %v", m.ID, m.Date, err)
		}
	}
}

func TestPostMemo_BuildFailureRestoresIndex(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("blog", 0o755); err != nil {
		t.Fatal(err)
	}
	testWrite(t, "blog/index.json", "[]\n")
	testWrite(t, "blog/all.json", `[{"id":1,"dir":"../escape","date":"2026-01-01T00:00:00Z"}]`)
	testPostDirs(t)
	before := testTree(t, ".")
	res := httptest.NewRecorder()
	postMemo(res, blogRequest(t, map[string]string{"body": "lost"}))
	if res.Code != http.StatusInternalServerError {
		t.Errorf("POST /memo: %d %s", res.Code, res.Body.String())
	}
	if after := testTree(t, "."); !reflect.DeepEqual(after, before) {
		t.Errorf("failed memo changed files: %v", after)
	}
}

func TestPostMemo_Commit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	t.Chdir(t.TempDir())
	testGit(t, "init", "--quiet")
	for _, identity := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
		if out, err := exec.CommandContext(t.Context(), "git", "var", identity).CombinedOutput(); err != nil {
			t.Skipf("inherited git identity unavailable: %v: %s", err, out)
		}
	}
	if err := os.Mkdir("blog", 0o755); err != nil {
		t.Fatal(err)
	}
	testWrite(t, "blog/index.json", "[]\n")
	testWrite(t, "blog/all.json", "[]\n")
	testPostDirs(t)
	if _, err := site.BuildSite(); err != nil {
		t.Fatal(err)
	}
	testGit(t, "add", "--", ".")
	testGit(t, "commit", "-m", "seed")
	res := httptest.NewRecorder()
	postMemo(res, blogRequest(t, map[string]string{"body": "hello", "commit": "on"}))
	if res.Code != http.StatusOK || !strings.HasSuffix(res.Body.String(), "\"memo: 001 written :: committed\"}\n") {
		t.Fatalf("POST /memo: %d %s", res.Code, res.Body.String())
	}
	if got := testGit(t, "log", "-1", "--format=%s"); got != "memo: 001\n" {
		t.Errorf("subject = %q", got)
	}
	if got := testGit(t, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD"); got != "memos/index.json\n" {
		t.Errorf("committed = %q", got)
	}
}
