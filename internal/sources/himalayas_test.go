package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/client"
)

// notFound, used as a page, makes pagedServer answer that cursor with a 404.
const notFound = "404"

// pagedServer plays the Himalayas feed: it answers each cursor ("" is the first page)
// with its page and counts the requests. The cursor is opaque to the client, so any
// string works. A cursor missing from pages fails the test, because the code under
// test sent a cursor the feed never handed out.
func pagedServer(t *testing.T, pages map[string]string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		cursor := r.URL.Query().Get("cursor")
		page, ok := pages[cursor]
		switch {
		case !ok:
			t.Errorf("unexpected cursor %q", cursor)
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		case page == notFound:
			w.WriteHeader(http.StatusNotFound)
		default:
			if _, err := w.Write([]byte(page)); err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func newTestHimalayas(maxPages int) Himalayas {
	return Himalayas{
		C:        client.NewClient(3, time.Second, 100*time.Millisecond, 0),
		MaxPages: maxPages,
	}
}

func TestCursor(t *testing.T) {
	srv, requests := pagedServer(t, map[string]string{
		"":   `{"jobs": [{"guid": "a"}, {"guid": "b"}], "nextCursor": "p2"}`,
		"p2": `{"jobs": [{"guid": "c"}, {"guid": "d"}], "nextCursor": "p3"}`,
		"p3": `{"jobs": [{"guid": "e"}, {"guid": "f"}], "nextCursor": ""}`,
	})

	items, err := newTestHimalayas(10).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if len(items) != 6 {
		t.Errorf("expected 6 items; got %d", len(items))
	}
	if got := requests.Load(); got != 3 {
		t.Errorf("expected 3 requests; got %d", got)
	}
}

func TestPageCap(t *testing.T) {
	// Every page points to another one: only the cap stops the loop.
	srv, requests := pagedServer(t, map[string]string{
		"":   `{"jobs": [{"guid": "a"}, {"guid": "b"}], "nextCursor": "p2"}`,
		"p2": `{"jobs": [{"guid": "c"}, {"guid": "d"}], "nextCursor": "p3"}`,
		"p3": `{"jobs": [{"guid": "e"}, {"guid": "f"}], "nextCursor": "p4"}`,
		"p4": `{"jobs": [{"guid": "g"}, {"guid": "h"}], "nextCursor": "p5"}`,
		"p5": `{"jobs": [{"guid": "i"}, {"guid": "j"}], "nextCursor": "p6"}`,
	})

	items, err := newTestHimalayas(4).Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if len(items) != 8 {
		t.Errorf("expected 8 items; got %d", len(items))
	}
	if got := requests.Load(); got != 4 {
		t.Errorf("expected 4 requests; got %d", got)
	}
}

func TestPageFailure(t *testing.T) {
	srv, requests := pagedServer(t, map[string]string{
		"":   `{"jobs": [{"guid": "a"}, {"guid": "b"}], "nextCursor": "p2"}`,
		"p2": notFound,
	})

	items, err := newTestHimalayas(10).Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if !strings.Contains(err.Error(), "page 2") {
		t.Errorf("expected err to contain 'page 2'; got %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items; got %d", len(items))
	}
	// 404 is not retried, so page 2 is requested exactly once.
	if got := requests.Load(); got != 2 {
		t.Errorf("expected 2 requests; got %d", got)
	}
}
