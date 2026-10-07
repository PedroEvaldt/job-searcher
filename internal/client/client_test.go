package client

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// noInterval keeps the rate limit out of tests that measure retries, backoff and cancellation.
const noInterval time.Duration = 0

func TestGoodServer(t *testing.T) {
	testBody := []byte("dummy")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(testBody)
		if err != nil {
			t.Errorf("failed to write body: %v", err)
		}
	}))
	defer srv.Close()

	c := NewClient(3, 60*time.Second, 10*time.Second, 1*time.Second)
	body, err := c.HTTPGetBody(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to get http body: %v", err)
	}
	if !bytes.Equal(body, testBody) {
		t.Errorf("expected dummy; got %s", string(body))
	}
}

func TestClientRetries(t *testing.T) {
	tests := []struct {
		name       string
		retries    int
		wantTries  int
		initialTim time.Duration
		statuses   []int
		wantErr    bool
	}{
		{"t1", 3, 3, 30 * time.Millisecond, []int{503, 503, 200}, false},
		{"t2", 3, 3, 30 * time.Millisecond, []int{503, 503, 500}, true},
		{"t3", 3, 1, 30 * time.Millisecond, []int{404, 200}, true},
		{"t4", 3, 1, 30 * time.Millisecond, []int{200}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var i atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if i.Load() > int64(tt.retries-1) {
					w.WriteHeader(tt.statuses[len(tt.statuses)-1])
				} else {
					w.WriteHeader(tt.statuses[i.Load()])
					i.Add(1)
				}
			}))
			defer srv.Close()

			c := NewClient(tt.retries, 60*time.Second, tt.initialTim, noInterval)
			_, err := c.HTTPGetBody(context.Background(), srv.URL)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected err; got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("wanted err nil; got %v", err)
				}
			}
			if i.Load() != int64(tt.wantTries) {
				t.Errorf("expected: %d; got %d", tt.wantTries, i.Load())
			}
		})
	}
}

func TestIncTime(t *testing.T) {
	type times struct {
		mu    sync.Mutex
		items []time.Time
	}
	tim := times{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		tim.mu.Lock()
		tim.items = append(tim.items, time.Now())
		tim.mu.Unlock()
	}))
	defer srv.Close()
	c := NewClient(4, 60*time.Second, 10*time.Millisecond, noInterval)
	_, err := c.HTTPGetBody(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	tim.mu.Lock()
	defer tim.mu.Unlock()
	if len(tim.items) != 4 {
		t.Fatalf("expected 4 times; got %d", len(tim.items))
	}
	for i := range tim.items {
		if i == 0 {
			continue
		}

		expectedInterval := time.Duration(10<<(i-1)) * time.Millisecond
		gotInterval := tim.items[i].Sub(tim.items[i-1])
		if gotInterval < expectedInterval {
			t.Errorf("interval %d: expected >= %v; got %v", i, expectedInterval, gotInterval)
		}
	}
}

func TestCtxCancel(t *testing.T) {
	var i atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		i.Add(1)
	}))
	defer srv.Close()

	wait := 1 * time.Second
	c := NewClient(3, 120*time.Second, wait, noInterval)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.HTTPGetBody(ctx, srv.URL)
	elapsed := time.Since(start)

	// A client that ignores ctx while waiting still fails with DeadlineExceeded on the
	// next request; only the elapsed time tells the two apart.
	if elapsed >= wait/2 {
		t.Errorf("expected return well before the %v wait; took %v", wait, elapsed)
	}
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded; got %v", err)
	}
	if i.Load() != 1 {
		t.Errorf("expected 1 trie; got %d", i.Load())
	}
}

func TestInterval(t *testing.T) {
	type times struct {
		mu    sync.Mutex
		items []time.Time
	}
	tim := times{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		tim.mu.Lock()
		tim.items = append(tim.items, time.Now())
		tim.mu.Unlock()
	}))
	defer srv.Close()
	c := NewClient(1, 60*time.Second, 100*time.Millisecond, 50*time.Millisecond)
	_, err := c.HTTPGetBody(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to get body: %v", err)
	}
	_, err = c.HTTPGetBody(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to get body: %v", err)
	}
	_, err = c.HTTPGetBody(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to get body: %v", err)
	}
	tim.mu.Lock()
	defer tim.mu.Unlock()
	if len(tim.items) != 3 {
		t.Fatalf("expected 3 times; got %d", len(tim.items))
	}
	for i := range tim.items {
		if i == 0 {
			continue
		}

		expectedInterval := 50 * time.Millisecond
		gotInterval := tim.items[i].Sub(tim.items[i-1])
		if gotInterval < expectedInterval {
			t.Errorf("interval %d: expected >= %v; got %v", i, expectedInterval, gotInterval)
		}
	}
}

func TestIntervalCtx(t *testing.T) {
	var counter atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		counter.Add(1)
	}))
	defer srv.Close()
	c := NewClient(1, 60*time.Second, 100*time.Millisecond, 1*time.Second)
	_, err := c.HTTPGetBody(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("failed to get body: %v", err)
	}

	begining := time.Now()
	testCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = c.HTTPGetBody(testCtx, srv.URL)
	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded; got %v", err)
	}
	if time.Since(begining) > 500*time.Millisecond {
		t.Errorf("expeceted duration less than 1 second; got %v", time.Since(begining))
	}
	if counter.Load() != 1 {
		t.Errorf("expected 1 request; got %d", counter.Load())
	}
}
