// Package web serves a browser viewer and editor for a project's docs/.
package web

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	"github.com/wjdp/wjdoc/internal/docs"
	"github.com/wjdp/wjdoc/internal/gitx"
)

//go:embed static
var static embed.FS

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

// Handler serves the viewer for the project at root.
func Handler(root string) http.Handler {
	server := &server{root: root}
	assets, _ := fs.Sub(static, "static")

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(assets))
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir(filepath.Join(root, "docs")))))
	mux.HandleFunc("GET /api/docs", server.list)
	mux.HandleFunc("GET /api/docs/{name}", server.show)
	mux.HandleFunc("PUT /api/docs/{name}", server.save)
	mux.HandleFunc("PATCH /api/docs/{name}", server.setFields)
	return localOnly(mux)
}

type server struct {
	root string
}

type summary struct {
	Name     string     `json:"name"`
	Number   int        `json:"number"`
	Path     string     `json:"path"`
	Type     string     `json:"type"`
	Status   string     `json:"status"`
	Title    string     `json:"title"`
	Created  *time.Time `json:"created"`
	Modified *time.Time `json:"modified"`
	Error    string     `json:"error,omitempty"`
}

type detail struct {
	summary
	Source  string `json:"source"`
	HTML    string `json:"html"`
	Version string `json:"version"`
}

type vocabulary struct {
	Types    []string `json:"types"`
	Statuses []string `json:"statuses"`
	Inactive []string `json:"inactive"`
}

func (s *server) list(writer http.ResponseWriter, _ *http.Request) {
	documents, err := s.loadWithDates()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	summaries := make([]summary, 0, len(documents))
	for _, document := range documents {
		summaries = append(summaries, toSummary(document))
	}

	var inactive []string
	for _, status := range docs.KnownStatuses {
		if docs.Inactive(status) {
			inactive = append(inactive, status)
		}
	}

	writeJSON(writer, http.StatusOK, struct {
		Project    string     `json:"project"`
		Docs       []summary  `json:"docs"`
		Vocabulary vocabulary `json:"vocabulary"`
	}{filepath.Base(s.root), summaries, vocabulary{docs.KnownTypes, docs.KnownStatuses, inactive}})
}

func (s *server) show(writer http.ResponseWriter, request *http.Request) {
	document, ok := s.find(writer, request)
	if !ok {
		return
	}
	s.writeDetail(writer, document)
}

// save replaces a document's source, refusing when the file changed on disk
// since the client loaded the version it is editing.
func (s *server) save(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Source  string `json:"source"`
		Version string `json:"version"`
	}
	if !decode(writer, request, &body) {
		return
	}
	s.edit(writer, request, body.Version, func([]byte) ([]byte, error) {
		return []byte(body.Source), nil
	})
}

// setFields edits frontmatter keys the same way wj set does: only the keys'
// own lines change.
func (s *server) setFields(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Fields  map[string]string `json:"fields"`
		Version string            `json:"version"`
	}
	if !decode(writer, request, &body) {
		return
	}
	s.edit(writer, request, body.Version, func(source []byte) ([]byte, error) {
		for key, value := range body.Fields {
			var err error
			if value == "" {
				source, err = docs.UnsetField(source, key)
			} else {
				source, err = docs.SetField(source, key, value)
			}
			if err != nil {
				return nil, err
			}
		}
		return source, nil
	})
}

func (s *server) edit(writer http.ResponseWriter, request *http.Request, version string, change func([]byte) ([]byte, error)) {
	document, ok := s.find(writer, request)
	if !ok {
		return
	}

	source, err := os.ReadFile(document.AbsPath)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	if version != "" && version != hash(source) {
		writeError(writer, http.StatusConflict, errors.New("the file changed on disk since it was loaded"))
		return
	}

	edited, err := change(source)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if !bytes.Equal(edited, source) {
		info, err := os.Stat(document.AbsPath)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
		if err := os.WriteFile(document.AbsPath, edited, info.Mode().Perm()); err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
	}

	reloaded, found, err := s.byName(filepath.Base(document.AbsPath))
	if err != nil || !found {
		writeError(writer, http.StatusInternalServerError, errors.New("could not reload the document"))
		return
	}
	s.writeDetail(writer, reloaded)
}

func (s *server) writeDetail(writer http.ResponseWriter, document docs.Document) {
	source, err := os.ReadFile(document.AbsPath)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	var rendered bytes.Buffer
	if err := markdown.Convert([]byte(document.Body), &rendered); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	writeJSON(writer, http.StatusOK, detail{
		summary: toSummary(document),
		Source:  string(source),
		HTML:    rendered.String(),
		Version: hash(source),
	})
}

// find resolves the {name} path value to a document, writing a 404 when there
// is none. Documents are addressed by filename, which is unique where numbers
// may not be.
func (s *server) find(writer http.ResponseWriter, request *http.Request) (docs.Document, bool) {
	document, found, err := s.byName(request.PathValue("name"))
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return docs.Document{}, false
	}
	if !found {
		writeError(writer, http.StatusNotFound, errors.New("no such document"))
		return docs.Document{}, false
	}
	return document, true
}

func (s *server) byName(name string) (docs.Document, bool, error) {
	documents, err := docs.Load(s.root)
	if err != nil {
		return docs.Document{}, false, err
	}
	for _, document := range documents {
		if filepath.Base(document.AbsPath) == name {
			found := []docs.Document{document}
			s.fillDates(found)
			return found[0], true, nil
		}
	}
	return docs.Document{}, false, nil
}

func (s *server) loadWithDates() ([]docs.Document, error) {
	documents, err := docs.Load(s.root)
	if err != nil {
		return nil, err
	}
	s.fillDates(documents)
	return documents, nil
}

// fillDates sets git dates on the documents. Outside a git repository they
// stay zero.
func (s *server) fillDates(documents []docs.Document) {
	paths := make([]string, 0, len(documents))
	for _, document := range documents {
		paths = append(paths, document.Path)
	}
	dates, err := gitx.DocumentDates(s.root, paths)
	if err != nil {
		return
	}
	for index := range documents {
		documents[index].Created = dates[documents[index].Path].Created
		documents[index].Modified = dates[documents[index].Path].Modified
	}
}

func toSummary(document docs.Document) summary {
	result := summary{
		Name:   filepath.Base(document.AbsPath),
		Number: document.Number,
		Path:   document.Path,
		Type:   document.Type,
		Status: document.Status,
		Title:  document.Title,
	}
	if !document.Created.IsZero() {
		result.Created = &document.Created
	}
	if !document.Modified.IsZero() {
		result.Modified = &document.Modified
	}
	if document.Err != nil {
		result.Error = document.Err.Error()
	}
	return result
}

func hash(source []byte) string {
	sum := sha256.Sum256(source)
	return hex.EncodeToString(sum[:])
}

func decode(writer http.ResponseWriter, request *http.Request, into any) bool {
	if err := json.NewDecoder(request.Body).Decode(into); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, struct {
		Error string `json:"error"`
	}{err.Error()})
}

// localOnly refuses requests that do not name a loopback host, which stops DNS
// rebinding, and writes that do not carry JSON from this origin, which stops
// other sites posting forms at the editor.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !loopbackHost(request.Host) {
			http.Error(writer, "forbidden host", http.StatusForbidden)
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") ||
				request.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(writer, "forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func loopbackHost(hostPort string) bool {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
