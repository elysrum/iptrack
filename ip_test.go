package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchIP(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		status  int
		wantIP  string
		wantErr bool
	}{
		{name: "success", body: "203.0.113.1\n", status: http.StatusOK, wantIP: "203.0.113.1"},
		{name: "trims whitespace", body: "  203.0.113.1  \n", status: http.StatusOK, wantIP: "203.0.113.1"},
		{name: "non-200 status", status: http.StatusServiceUnavailable, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			orig := ipconfigURL
			ipconfigURL = srv.URL
			defer func() { ipconfigURL = orig }()

			ip, err := fetchIP()
			if (err != nil) != tt.wantErr {
				t.Fatalf("fetchIP() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && ip != tt.wantIP {
				t.Errorf("got %q, want %q", ip, tt.wantIP)
			}
		})
	}
}

func TestFetchIPNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	orig := ipconfigURL
	ipconfigURL = srv.URL
	defer func() { ipconfigURL = orig }()

	if _, err := fetchIP(); err == nil {
		t.Fatal("expected error for closed server, got nil")
	}
}
