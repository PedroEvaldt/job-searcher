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

func TestCursor(t *testing.T) {
	var counter atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("cursor")
		counter.Add(1)
		switch {
		case c == "":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "a"}, {"guid": "b"}], "nextCursor": "p2"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		case c == "p2":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "c"}, {"guid": "d"}], "nextCursor": "p3"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		case c == "p3":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "e"}, {"guid": "f"}], "nextCursor": ""}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		default:
			t.Errorf("unexpected cursor %q", c)
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	c := &client.Client{
		HTTP:        &http.Client{Timeout: 1 * time.Second},
		Retries:     3,
		InitialTime: 100 * time.Millisecond,
	}
	h := Himalayas{
		C:    c,
		Ceil: 10,
	}
	items, err := h.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if len(items) != 6 {
		t.Errorf("expected 6 items; got %d", len(items))
	}
	if counter.Load() != 3 {
		t.Errorf("expected 3 fetchs; got %d", counter.Load())
	}
}

func TestHighCeil(t *testing.T) {
	var counter atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("cursor")
		counter.Add(1)
		switch {
		case c == "":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "a"}, {"guid": "b"}], "nextCursor": "p2"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		case c == "p2":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "c"}, {"guid": "d"}], "nextCursor": "p3"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		case c == "p3":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "e"}, {"guid": "f"}], "nextCursor": "p4"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		case c == "p4":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "g"}, {"guid": "h"}], "nextCursor": "p5"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		case c == "p5":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "i"}, {"guid": "j"}], "nextCursor": "p6"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		default:
			t.Errorf("unexpected cursor %q", c)
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	c := &client.Client{
		HTTP:        &http.Client{Timeout: 1 * time.Second},
		Retries:     3,
		InitialTime: 100 * time.Millisecond,
	}
	h := Himalayas{
		C:    c,
		Ceil: 4,
	}
	items, err := h.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if len(items) != 8 {
		t.Errorf("expected 8 items; got %d", len(items))
	}
	if counter.Load() != 4 {
		t.Errorf("expected 4 fetchs; got %d", counter.Load())
	}
}

func TestMiddleErr(t *testing.T) {
	var counter atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("cursor")
		counter.Add(1)
		switch {
		case c == "":
			_, err := w.Write([]byte(`{"jobs": [{"guid": "a"}, {"guid": "b"}], "nextCursor": "p2"}`))
			if err != nil {
				t.Errorf("failed to write body: %v", err)
			}
		case c == "p2":
			w.WriteHeader(404)
		default:
			t.Errorf("unexpected cursor %q", c)
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	c := &client.Client{
		HTTP:        &http.Client{Timeout: 1 * time.Second},
		Retries:     3,
		InitialTime: 100 * time.Millisecond,
	}
	h := Himalayas{
		C:    c,
		Ceil: 10,
	}
	items, err := h.Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if !strings.Contains(err.Error(), "page 2") {
		t.Errorf("expeceted err to containg 'page 2'; got %s", err.Error())
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items; got %d", len(items))
	}

	if counter.Load() != 2 {
		t.Errorf("expected 2 req; got %d", counter.Load())
	}
}
