package site

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWriteFile_FailurePreservesFiles(t *testing.T) {
	for _, kind := range []string{"rename", "temporary symlink"} {
		t.Run(kind, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if kind == "rename" {
				if err := os.Mkdir("data.json", 0o755); err != nil {
					t.Fatal(err)
				}
				testWrite(t, "data.json/keep", "original")
			} else {
				testWrite(t, "data.json", "original")
				testWrite(t, "other.json", "do not truncate")
				if err := os.Symlink("other.json", "data.json.tmp"); err != nil {
					t.Fatal(err)
				}
			}
			before := testTree(t, ".")
			if err := WriteFile("data.json", []byte("replacement")); err == nil {
				t.Fatal("WriteFile unexpectedly succeeded")
			}
			if after := testTree(t, "."); !reflect.DeepEqual(after, before) {
				t.Errorf("failed write mutated files: got %v, want %v", after, before)
			}
		})
	}
}

func TestGuard_RejectsUnauthorized(t *testing.T) {
	t.Chdir(t.TempDir())
	calls := 0
	handler := Guard("example.com", "test-token")(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if _, ok := r.Context().Deadline(); !ok {
			t.Error("guarded request has no deadline")
		}
	})
	for _, tc := range []struct {
		host, token string
		want        int
	}{
		{"attacker.example", "test-token", http.StatusForbidden},
		{"example.com", "wrong-token", http.StatusForbidden},
		{"example.com", "test-token0", http.StatusForbidden},
		{"example.com", "", http.StatusForbidden},
		{"example.com", "test-token", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodPost, "/memo", nil)
		req.Host = tc.host
		req.Header.Set("X-Poster-Token", tc.token)
		res := httptest.NewRecorder()
		handler(res, req)
		if res.Code != tc.want {
			t.Errorf("host %q token %q: %d, want %d", tc.host, tc.token, res.Code, tc.want)
		}
	}
	if calls != 1 {
		t.Errorf("handler ran %d times, want 1", calls)
	}
}

// The poster and the writer are separate processes; a write in one must wait
// for a write in the other.
func TestGuard_WaitsForOtherProcessLock(t *testing.T) {
	t.Chdir(t.TempDir())
	other, err := os.OpenFile(".site.lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := syscall.Flock(int(other.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	ran := make(chan struct{})
	handler := Guard("example.com", "test-token")(func(w http.ResponseWriter, r *http.Request) {
		close(ran)
	})
	req := httptest.NewRequest(http.MethodPost, "/memo", nil)
	req.Host = "example.com"
	req.Header.Set("X-Poster-Token", "test-token")
	done := make(chan struct{})
	go func() {
		handler(httptest.NewRecorder(), req)
		close(done)
	}()
	select {
	case <-ran:
		t.Fatal("guarded handler ran while another process held the site lock")
	case <-time.After(200 * time.Millisecond):
	}
	if err := syscall.Flock(int(other.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("guarded handler never ran after the lock was released")
	}
}

func TestConvert_RejectsNonRasterUploads(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, body := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg"><image href="file:///etc/passwd"/></svg>`,
		"%!PS-Adobe-3.0\n",
		"push graphic-context\n",
		"",
		"text:/etc/passwd",
	} {
		err := Convert(t.Context(), strings.NewReader(body), "out.webp", "10x10")
		if err == nil || !strings.Contains(err.Error(), "unsupported image type") {
			t.Errorf("Convert(%q) = %v, want an unsupported image type error", body, err)
		}
	}
	if _, err := os.Stat("out.webp"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("rejected upload produced output: %v", err)
	}
}

func TestValidDir(t *testing.T) {
	for dir, want := range map[string]bool{
		"001-x": true, "110-southern-decay": true, "1000-a-b-c": true,
		"": false, ".": false, "..": false, "../x": false, "001-x/y": false, "/tmp": false, "01-x": false, "001-": false, "001-X": false, "001--x": false,
	} {
		if got := ValidDir(dir); got != want {
			t.Errorf("ValidDir(%q) = %v, want %v", dir, got, want)
		}
	}
}

func TestMount_ServesRootFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, file := range []string{"shrine.html", "feed.xml", "stories.xml", "sitemap.xml", "llms.txt", "index.html", "secret.txt"} {
		testWrite(t, file, file)
	}
	mux := http.NewServeMux()
	Mount(mux)
	for path, want := range map[string]int{
		"/site/shrine.html": http.StatusOK, "/site/feed.xml": http.StatusOK, "/site/stories.xml": http.StatusOK,
		"/site/sitemap.xml": http.StatusOK, "/site/llms.txt": http.StatusOK, "/site/secret.txt": http.StatusNotFound,
	} {
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != want {
			t.Errorf("GET %s = %d, want %d", path, res.Code, want)
		}
	}
}

func TestDirNameAndSplitTags(t *testing.T) {
	if got := DirName(7, "  Night <Rain> 雪!! "); got != "007-night-rain" {
		t.Errorf("DirName = %q", got)
	}
	if got := DirName(12, "雪"); got != "012-untitled" {
		t.Errorf("DirName = %q", got)
	}
	if got := SplitTags(" a, ,b ,, c d "); !reflect.DeepEqual(got, []string{"a", "b", "c d"}) {
		t.Errorf("SplitTags = %q", got)
	}
	if got := SplitTags(""); got == nil || len(got) != 0 {
		t.Errorf("SplitTags of nothing = %#v, want an empty slice", got)
	}
}

// An unpublished story removes a directory that may or may not be tracked;
// both must commit without git rejecting the pathspec.
func TestPublish_CommitsRemovedPaths(t *testing.T) {
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
	if err := os.MkdirAll("stories/001-gone", 0o755); err != nil {
		t.Fatal(err)
	}
	testWrite(t, "stories/001-gone/post.json", "{}")
	testWrite(t, "stories/index.json", "[{}]")
	testWrite(t, "unrelated.txt", "x")
	testGit(t, "add", "--", ".")
	testGit(t, "commit", "-m", "seed")
	if err := os.RemoveAll("stories/001-gone"); err != nil {
		t.Fatal(err)
	}
	testWrite(t, "stories/index.json", "[]")
	testWrite(t, "unrelated.txt", "changed")

	status := Publish(t.Context(), true, false, "story: stories/001-gone", "stories/001-gone", "stories/never-tracked", "stories/index.json", "stories/index.json")
	if status != " :: committed" {
		t.Fatalf("Publish = %q", status)
	}
	if got := testGit(t, "log", "-1", "--format=%s"); got != "story: stories/001-gone\n" {
		t.Errorf("subject = %q", got)
	}
	if got := testGit(t, "diff-tree", "--no-commit-id", "--name-status", "-r", "HEAD"); got != "D\tstories/001-gone/post.json\nM\tstories/index.json\n" {
		t.Errorf("committed = %q", got)
	}
	if got := testGit(t, "status", "--porcelain"); got != " M unrelated.txt\n" {
		t.Errorf("status = %q", got)
	}
	if got := Publish(t.Context(), false, false, "unused"); got != "" {
		t.Errorf("no-op Publish = %q", got)
	}
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
