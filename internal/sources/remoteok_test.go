package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PedroEvaldt/job-searcher/internal/client"
)

// TestRemoteOK checks how Fetch handles the legal notice that opens the Remote OK feed:
// an array without it is an error, and the notice alone is an empty feed.
func TestRemoteOK(t *testing.T) {
	tests := []struct {
		name          string
		body          []byte
		itemsExpected int
		wantErr       bool
	}{
		{"empty feed", []byte(`[]`), 0, true},
		{"only the legal notice", []byte(`[{"legal": "API terms"}]`), 0, false},
		{"legal notice and 2 jobs", []byte(`[{"legal": "API terms"}, {"id": "1"}, {"id": "2"}]`), 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := w.Write(tt.body)
				if err != nil {
					t.Errorf("failed to write to server: %v", err)
				}
			}))
			t.Cleanup(srv.Close)
			remoteOK := RemoteOK{
				C: client.NewClient(3, time.Second, 100*time.Millisecond, 0),
			}
			jsonMsg, err := remoteOK.Fetch(context.Background(), srv.URL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected err got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("failed to fetch server: %v", err)
				}
				if len(jsonMsg) != tt.itemsExpected {
					t.Errorf("expected %d items; got %d", tt.itemsExpected, len(jsonMsg))
				}
			}
		})
	}
}
