package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type loggedResponse struct {
	http.ResponseWriter
	status int
	bytes  int
	err    error
}

func (w *loggedResponse) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggedResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *loggedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Log requests without query strings, headers or bodies, which can contain secrets.
func requestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var id [16]byte
		_, _ = rand.Read(id[:])
		requestID := hex.EncodeToString(id[:])
		w.Header().Set("X-Request-ID", requestID)
		out := &loggedResponse{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				// Do not log the panic value: it may contain request data.
				slog.Error("http panic", "request_id", requestID, "stack", string(debug.Stack()))
				if out.status == 0 {
					respond(out, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
				}
			}
			status := out.status
			if status == 0 {
				status = http.StatusOK
			}
			attrs := []any{"request_id", requestID, "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(start).Milliseconds(), "bytes", out.bytes}
			if out.err != nil {
				attrs = append(attrs, "error", out.err.Error())
			}
			if status >= 500 {
				slog.Error("http request", attrs...)
			} else if status >= 400 {
				slog.Warn("http request", attrs...)
			} else {
				slog.Info("http request", attrs...)
			}
		}()
		next.ServeHTTP(out, r)
	})
}
