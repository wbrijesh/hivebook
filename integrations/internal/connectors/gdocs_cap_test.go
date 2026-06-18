package connectors

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestGoogleDocsSyncDrainsModifiedTimePlateau locks P2-2: the per-run page cap must
// not stall the cursor on a single-modifiedTime plateau larger than the budget —
// the connector drains it (same guarantee as GitHub issues), so the watermark
// advances and no doc is missed (design-doc 0011).
func TestGoogleDocsSyncDrainsModifiedTimePlateau(t *testing.T) {
	files := []struct{ id, mtime string }{
		{"d1", "2026-06-01T00:00:00Z"}, {"d2", "2026-06-01T00:00:00Z"},
		{"d3", "2026-06-01T00:00:00Z"}, {"d4", "2026-06-02T00:00:00Z"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/drive/v3/files":
			q := r.URL.Query()
			size, _ := strconv.Atoi(q.Get("pageSize"))
			off := 0
			if tk := q.Get("pageToken"); tk != "" {
				off, _ = strconv.Atoi(tk)
			}
			end := off + size
			if end > len(files) {
				end = len(files)
			}
			next := ""
			if end < len(files) {
				next = strconv.Itoa(end)
			}
			var b strings.Builder
			b.WriteString(`{"files":[`)
			for i, f := range files[off:end] {
				if i > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, `{"id":"%s","name":"%s","modifiedTime":"%s","createdTime":"%s","webViewLink":"http://x/%s","parents":["root"]}`,
					f.id, f.id, f.mtime, f.mtime, f.id)
			}
			fmt.Fprintf(&b, `],"nextPageToken":"%s"}`, next)
			_, _ = w.Write([]byte(b.String()))
		case strings.HasSuffix(r.URL.Path, "/export"):
			_, _ = w.Write([]byte("<html>doc</html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	g := &GoogleDocs{driveBaseURL: srv.URL, userinfoBaseURL: srv.URL, pageSize: 1, maxPages: 2} // budget < plateau
	emitted := map[string]bool{}
	// The unit cursor is the high-water of committed items' Artifact.Cursor (ADR-0038).
	var cursor string
	collect := func(_ context.Context, a Artifact) error {
		emitted[a.ExternalID] = true
		if a.Cursor > cursor {
			cursor = a.Cursor
		}
		return nil
	}
	c := Unit{ID: "drive", Kind: "google_drive", Name: "Drive"}

	if _, err := g.SyncUnit(context.Background(), srv.Client(), c, "", collect); err != nil {
		t.Fatalf("run1: %v", err)
	}
	pos1 := cursor
	if _, err := g.SyncUnit(context.Background(), srv.Client(), c, pos1, collect); err != nil {
		t.Fatalf("run2: %v", err)
	}
	pos2 := cursor

	if pos2 != "2026-06-02T00:00:00Z" {
		t.Fatalf("cursor should advance past the plateau, got %q (pos1=%q)", pos2, pos1)
	}
	for _, id := range []string{"d1", "d2", "d3", "d4"} {
		if !emitted[id] {
			t.Fatalf("doc %s never emitted — the plateau stalled the cursor", id)
		}
	}
}
