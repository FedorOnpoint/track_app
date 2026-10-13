package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckRejectsUnreadyAPI(t *testing.T) {
	for _, status := range []int{200, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/health/ready" {
					t.Error(r.URL.Path)
				}
				w.WriteHeader(status)
			}))
			defer s.Close()
			err := check(s.URL)
			if (err == nil) != (status == 200) {
				t.Fatalf("status=%d err=%v", status, err)
			}
		})
	}
}
