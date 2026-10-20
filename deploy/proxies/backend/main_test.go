package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func do(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestFailAndRecover(t *testing.T) {
	h := newHandler("b1")
	if rec := do(h, "GET", "/healthz"); rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "b1" {
		t.Fatalf("healthy = %d %q", rec.Code, rec.Body.String())
	}
	do(h, "POST", "/fail")
	if rec := do(h, "GET", "/healthz"); rec.Code != 503 {
		t.Fatalf("after fail = %d, want 503", rec.Code)
	}
	do(h, "POST", "/recover")
	if rec := do(h, "GET", "/"); rec.Code != 200 {
		t.Fatalf("after recover = %d, want 200", rec.Code)
	}
}
