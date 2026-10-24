package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureRoot = "../docs/testdata/basic"

// copyFixture copies the basic fixture so that writes never touch testdata.
func copyFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	if err := os.Mkdir(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(fixtureRoot, "docs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		source, err := os.ReadFile(filepath.Join(fixtureRoot, "docs", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(docsDir, entry.Name()), source, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func request(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:1234"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeDetail(t *testing.T, recorder *httptest.ResponseRecorder) detail {
	t.Helper()

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body)
	}
	var got detail
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestListReturnsEveryDocumentAndTheVocabulary(t *testing.T) {
	recorder := request(t, Handler(fixtureRoot), http.MethodGet, "/api/docs", "")

	var got struct {
		Docs       []summary  `json:"docs"`
		Vocabulary vocabulary `json:"vocabulary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Docs) != 9 {
		t.Errorf("got %d docs, want 9", len(got.Docs))
	}
	if !strings.Contains(strings.Join(got.Vocabulary.Inactive, ","), "done") {
		t.Errorf("inactive %v does not include done", got.Vocabulary.Inactive)
	}
}

func TestShowRendersMarkdownByFilename(t *testing.T) {
	got := decodeDetail(t, request(t, Handler(fixtureRoot), http.MethodGet, "/api/docs/01-Sync%20robustness.md", ""))

	if got.Number != 1 || !strings.Contains(got.HTML, "<h1") || got.Version == "" {
		t.Errorf("unexpected detail: number %d, html %q, version %q", got.Number, got.HTML, got.Version)
	}
}

func TestShowUnknownIs404(t *testing.T) {
	if code := request(t, Handler(fixtureRoot), http.MethodGet, "/api/docs/99-Nope.md", "").Code; code != http.StatusNotFound {
		t.Errorf("status %d, want 404", code)
	}
}

func TestSaveWritesTheSourceAndRefusesAStaleVersion(t *testing.T) {
	root := copyFixture(t)
	handler := Handler(root)
	target := "/api/docs/04-Ship-the-loader.md"

	loaded := decodeDetail(t, request(t, handler, http.MethodGet, target, ""))
	edited := loaded.Source + "\nMore.\n"
	body, _ := json.Marshal(map[string]string{"source": edited, "version": loaded.Version})

	saved := decodeDetail(t, request(t, handler, http.MethodPut, target, string(body)))
	if saved.Source != edited {
		t.Errorf("saved source %q, want %q", saved.Source, edited)
	}
	onDisk, _ := os.ReadFile(filepath.Join(root, "docs", "04-Ship-the-loader.md"))
	if string(onDisk) != edited {
		t.Errorf("disk has %q", onDisk)
	}

	if code := request(t, handler, http.MethodPut, target, string(body)).Code; code != http.StatusConflict {
		t.Errorf("stale save: status %d, want 409", code)
	}
}

func TestPatchSetsStatusOnly(t *testing.T) {
	root := copyFixture(t)
	handler := Handler(root)
	target := "/api/docs/04-Ship-the-loader.md"

	before, _ := os.ReadFile(filepath.Join(root, "docs", "04-Ship-the-loader.md"))
	got := decodeDetail(t, request(t, handler, http.MethodPatch, target, `{"fields":{"status":"done"}}`))

	if got.Status != "done" {
		t.Errorf("status %q, want done", got.Status)
	}
	_, beforeBody, _ := strings.Cut(string(before), "\n---\n")
	_, afterBody, _ := strings.Cut(got.Source, "\n---\n")
	if beforeBody != afterBody {
		t.Errorf("body changed:\n%q\n%q", beforeBody, afterBody)
	}
}

func TestWritesNeedJSONAndALoopbackHost(t *testing.T) {
	handler := Handler(copyFixture(t))

	form := httptest.NewRequest(http.MethodPut, "/api/docs/04-Ship-the-loader.md", strings.NewReader("source=x"))
	form.Host = "127.0.0.1"
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, form)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("form post: status %d, want 403", recorder.Code)
	}

	rebound := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rebound.Host = "evil.example:1234"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, rebound)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("foreign host: status %d, want 403", recorder.Code)
	}
}

func TestServesTheApp(t *testing.T) {
	recorder := request(t, Handler(fixtureRoot), http.MethodGet, "/", "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "app.js") {
		t.Errorf("status %d, body %q", recorder.Code, recorder.Body)
	}
}

func TestPatchSetsPriorityAndTagsComeBack(t *testing.T) {
	root := copyFixture(t)
	path := filepath.Join(root, "docs", "04-Ship-the-loader.md")
	source, _ := os.ReadFile(path)
	tagged := strings.Replace(string(source), "status: todo\n", "status: todo\ntags: [area:engine]\n", 1)
	if err := os.WriteFile(path, []byte(tagged), 0o644); err != nil {
		t.Fatal(err)
	}

	got := decodeDetail(t, request(t, Handler(root), http.MethodPatch, "/api/docs/04-Ship-the-loader.md", `{"fields":{"priority":"p1"}}`))
	if got.Priority != "p1" {
		t.Errorf("priority %q, want p1", got.Priority)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "area:engine" {
		t.Errorf("tags %q, want [area:engine]", got.Tags)
	}
}
