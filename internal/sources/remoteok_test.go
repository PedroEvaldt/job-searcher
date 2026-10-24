package sources

import (
	"context"
	"encoding/json"
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

// TestRemoteOKSalary checks that ToJob maps salary into a nullable, decimal-capable
// value: the 0 sentinel (and an absent field) become nil (stored as NULL), a real
// amount is kept, and a fractional amount is not truncated.
func TestRemoteOKSalary(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantNil bool
		wantVal float64
	}{
		{"a real salary is kept", `{"id":"1","salary_min":50000}`, false, 50000},
		{"a fractional salary is not truncated", `{"id":"2","salary_min":0.59}`, false, 0.59},
		{"the 0 sentinel becomes NULL", `{"id":"3","salary_min":0}`, true, 0},
		{"an absent salary is NULL", `{"id":"4"}`, true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := RemoteOK{}.ToJob(json.RawMessage(tt.raw))
			if err != nil {
				t.Fatalf("ToJob failed: %v", err)
			}
			if tt.wantNil {
				if job.SalaryMin != nil {
					t.Errorf("expected nil salary_min; got %v", *job.SalaryMin)
				}
				return
			}
			if job.SalaryMin == nil {
				t.Fatalf("expected salary_min %v; got nil", tt.wantVal)
			}
			if *job.SalaryMin != tt.wantVal {
				t.Errorf("expected salary_min %v; got %v", tt.wantVal, *job.SalaryMin)
			}
		})
	}
}
