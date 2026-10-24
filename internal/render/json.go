package render

import (
	"time"

	"github.com/wjdp/wjdoc/internal/docs"
)

// jsonDoc is the stable JSON shape shared by list and show, per
// docs/001-Design.md "Output": number, path, type, status, priority, tags,
// title, created, modified.
type jsonDoc struct {
	Number   int      `json:"number"`
	Path     string   `json:"path"`
	Type     string   `json:"type"`
	Status   string   `json:"status"`
	Priority string   `json:"priority"`
	Tags     []string `json:"tags"`
	Title    string   `json:"title"`
	Created  *string  `json:"created"`
	Modified *string  `json:"modified"`
	Err      *string  `json:"error"`
}

// jsonShowDoc is jsonDoc plus the body, used by Show.
type jsonShowDoc struct {
	jsonDoc
	Body string `json:"body"`
}

func toJSONDoc(doc docs.Document) jsonDoc {
	jd := jsonDoc{
		Number:   doc.Number,
		Path:     doc.Path,
		Type:     doc.Type,
		Status:   doc.Status,
		Priority: doc.Priority,
		Tags:     doc.Tags,
		Title:    doc.Title,
	}
	if jd.Tags == nil {
		jd.Tags = []string{}
	}
	if !doc.Created.IsZero() {
		created := doc.Created.Format(time.RFC3339)
		jd.Created = &created
	}
	if !doc.Modified.IsZero() {
		modified := doc.Modified.Format(time.RFC3339)
		jd.Modified = &modified
	}
	if doc.Err != nil {
		errMsg := doc.Err.Error()
		jd.Err = &errMsg
	}
	return jd
}
