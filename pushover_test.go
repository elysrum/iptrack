package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotify(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "success", status: http.StatusOK, wantErr: false},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
		{name: "bad request", status: http.StatusBadRequest, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				for _, field := range []string{"token", "user", "title", "message"} {
					if r.FormValue(field) == "" {
						t.Errorf("missing required form field %q", field)
					}
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			orig := pushoverURL
			pushoverURL = srv.URL
			defer func() { pushoverURL = orig }()

			err := notify("tok", "usr", "Test Title", "Test message")
			if (err != nil) != tt.wantErr {
				t.Fatalf("notify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotifyNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	orig := pushoverURL
	pushoverURL = srv.URL
	defer func() { pushoverURL = orig }()

	if err := notify("tok", "usr", "title", "msg"); err == nil {
		t.Fatal("expected error for closed server, got nil")
	}
}
