package middleware

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		if r.Method == "OPTIONS" {
            next.ServeHTTP(w, r)
            return
        }

		wrapped := &responseWriterWrapper{
			w:      w,
			status: http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		userID := extractUserID(r)

		switch {
		case wrapped.status >= 500:
			log.Printf("AUDIT | user:%-36s | %-6s %-40s | %d | %v | ip:%s",
				userID, r.Method, r.URL.Path, wrapped.status, duration, r.RemoteAddr)
		case wrapped.status >= 400:
			log.Printf("AUDIT | user:%-36s | %-6s %-40s | %d | %v | ip:%s",
				userID, r.Method, r.URL.Path, wrapped.status, duration, r.RemoteAddr)
		default:
			log.Printf("AUDIT | user:%-36s | %-6s %-40s | %d | %v | ip:%s",
				userID, r.Method, r.URL.Path, wrapped.status, duration, r.RemoteAddr)
		}
	})
}

type responseWriterWrapper struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriterWrapper) Header() http.Header {
	return rw.w.Header()
}

func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.w.Write(b)
	rw.size += n
	return n, err
}

func (rw *responseWriterWrapper) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.w.WriteHeader(statusCode)
}

func (rw *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.w.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (rw *responseWriterWrapper) Flush() {
	if flusher, ok := rw.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func extractUserID(r *http.Request) string {
	if userID, ok := r.Context().Value(UserIDKey).(string); ok && userID != "" {
		return userID
	}
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		return userID
	}
	return "anónimo"
}
