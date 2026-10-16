package httpapi

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogging(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	h := requestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failure(w, errors.New("database unavailable"))
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test?token=secret", nil))
	if w.Code != 500 || w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("unexpected response: %v", w)
	}
	for _, want := range []string{`"status":500`, `"error":"database unavailable"`, w.Header().Get("X-Request-ID")} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("missing %s in %s", want, logs.String())
		}
	}
	if strings.Contains(logs.String(), "secret") || strings.Contains(w.Body.String(), "database unavailable") {
		t.Fatal("sensitive detail exposed")
	}
}

func TestRequestLoggingRecoversPanic(t *testing.T) {
	h := requestLogging(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("private") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "private") {
		t.Fatalf("unexpected response: %v", w)
	}
}
