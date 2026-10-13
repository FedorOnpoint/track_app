package seedcontent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLinksDistinguishBrokenAndUnavailable(t *testing.T) {
	cases := []struct {
		code int
		want LinkStatus
	}{{200, LinkOK}, {404, LinkBroken}, {410, LinkBroken}, {429, LinkUnavailable}, {503, LinkUnavailable}, {403, LinkUnavailable}}
	for _, tc := range cases {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code) }))
		result := NewLinkChecker(server.Client()).Check(context.Background(), server.URL)
		server.Close()
		if result.Status != tc.want || result.HTTPStatus != tc.code {
			t.Fatalf("code=%d result=%+v", tc.code, result)
		}
	}
}

func TestLinkCheckerFallsBackToBoundedGet(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(405)
			return
		}
		if r.Method != http.MethodGet || r.Header.Get("Range") != "bytes=0-1023" {
			t.Errorf("unexpected request: %s range=%s", r.Method, r.Header.Get("Range"))
		}
		w.WriteHeader(206)
	}))
	defer server.Close()
	if result := NewLinkChecker(server.Client()).Check(context.Background(), server.URL); result.Status != LinkOK {
		t.Fatal(result)
	}
}

func TestLinkCheckerRejectsUnsafeURLsAndRedirects(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, "http://example.com/", http.StatusFound)
	}))
	defer server.Close()
	checker := NewLinkChecker(server.Client())
	for _, url := range []string{"http://example.com/", "javascript:alert(1)", "https://user:password@example.com/"} {
		if result := checker.Check(context.Background(), url); result.Status != LinkInvalid {
			t.Fatal(result)
		}
	}
	if requests != 0 {
		t.Fatalf("unsafe URL caused %d requests", requests)
	}
	if result := checker.Check(context.Background(), server.URL); result.Status != LinkInvalid {
		t.Fatal(result)
	}
}

func TestLinkCheckerReportsTimeoutAsUnavailable(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if result := NewLinkChecker(server.Client()).Check(ctx, server.URL); result.Status != LinkUnavailable {
		t.Fatal(result)
	}
}
