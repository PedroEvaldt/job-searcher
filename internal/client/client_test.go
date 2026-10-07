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

// arrivals records when each request reaches a test server. Handlers run on the
// server's goroutines, so every access goes through the mutex.
type arrivals struct {
	mu    sync.Mutex
	times []time.Time
}

func (a *arrivals) record() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.times = append(a.times, time.Now())
}

func (a *arrivals) snapshot() []time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]time.Time(nil), a.times...)
}

// assertGaps fails unless got holds exactly want arrivals and the gap before arrival
// i is at least minGap(i). Only a lower bound: a busy machine stretches gaps, it
// never shrinks them.
func assertGaps(t *testing.T, got []time.Time, want int, minGap func(i int) time.Duration) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("expected %d requests; got %d", want, len(got))
	}
	for i := 1; i < len(got); i++ {
		if gap := got[i].Sub(got[i-1]); gap < minGap(i) {
			t.Errorf("gap %d: expected >= %v; got %v", i, minGap(i), gap)
		}
	}
}

func TestGoodServer(t *testing.T) {
	testBody := []byte("dummy")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(testBody); err != nil {
			t.Errorf("failed to write body: %v", err)
		}
	}))
	defer srv.Close()

	c := NewClient(3, 60*time.Second, 10*time.Second, time.Second)
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
		name        string
		retries     int
		wantTries   int
		initialWait time.Duration
		statuses    []int
		wantErr     bool
	}{
		{"recovers after two 503", 3, 3, 30 * time.Millisecond, []int{503, 503, 200}, false},
		{"gives up after max attempts", 3, 3, 30 * time.Millisecond, []int{503, 503, 500}, true},
		// A client that retried 404 would reach the 200 and succeed.
		{"does not retry 404", 3, 1, 30 * time.Millisecond, []int{404, 200}, true},
		{"succeeds at once", 3, 1, 30 * time.Millisecond, []int{200}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Count first, then answer; past the end of the script, repeat the last status.
				n := int(requests.Add(1))
				w.WriteHeader(tt.statuses[min(n, len(tt.statuses))-1])
			}))
			defer srv.Close()

			c := NewClient(tt.retries, 60*time.Second, tt.initialWait, noInterval)
			_, err := c.HTTPGetBody(context.Background(), srv.URL)

			if tt.wantErr && err == nil {
				t.Fatalf("expected err; got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected err nil; got %v", err)
			}
			if got := requests.Load(); got != int64(tt.wantTries) {
				t.Errorf("expected %d requests; got %d", tt.wantTries, got)
			}
		})
	}
}

func TestBackoffGrows(t *testing.T) {
	var arr arrivals
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arr.record()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	initialWait := 10 * time.Millisecond
	c := NewClient(4, 60*time.Second, initialWait, noInterval)
	if _, err := c.HTTPGetBody(context.Background(), srv.URL); err == nil {
		t.Fatalf("expected err; got nil")
	}
	// The wait doubles: 10ms, 20ms, 40ms.
	assertGaps(t, arr.snapshot(), 4, func(i int) time.Duration { return initialWait << (i - 1) })
}

func TestBackoffHonorsCtx(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	wait := time.Second
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
	if got := requests.Load(); got != 1 {
		t.Errorf("expected 1 request; got %d", got)
	}
}

func TestInterval(t *testing.T) {
	var arr arrivals
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arr.record()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	interval := 50 * time.Millisecond
	c := NewClient(1, 60*time.Second, 100*time.Millisecond, interval)
	for range 3 {
		if _, err := c.HTTPGetBody(context.Background(), srv.URL); err != nil {
			t.Fatalf("failed to get body: %v", err)
		}
	}
	assertGaps(t, arr.snapshot(), 3, func(int) time.Duration { return interval })
}

func TestIntervalHonorsCtx(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	interval := time.Second
	c := NewClient(1, 60*time.Second, 100*time.Millisecond, interval)
	if _, err := c.HTTPGetBody(context.Background(), srv.URL); err != nil {
		t.Fatalf("failed to get body: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.HTTPGetBody(ctx, srv.URL)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected err; got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded; got %v", err)
	}
	// Half the interval sits far from both outcomes: ~50ms when ctx cuts the wait,
	// ~1s when it does not.
	if elapsed >= interval/2 {
		t.Errorf("expected return well before the %v interval; took %v", interval, elapsed)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("expected 1 request; got %d", got)
	}
}
