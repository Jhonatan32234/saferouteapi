// middleware/logging.go - Asegúrate de que no esté rompiendo Hijacker
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
        
        // Usar un ResponseWriter que NO rompa Hijacker
        // Este wrapper debe implementar http.Hijacker también
        wrapped := &responseWriterWrapper{w: w}
        
        next.ServeHTTP(wrapped, r)
        
        log.Printf("AUDIT | user:%s | %s %s | %d | %v | ip:%s",
            getUserID(r), r.Method, r.URL.Path, wrapped.status, time.Since(start), r.RemoteAddr)
    })
}

// responseWriterWrapper debe implementar http.Hijacker para no romper WebSocket
type responseWriterWrapper struct {
    w      http.ResponseWriter
    status int
}

func (rw *responseWriterWrapper) Header() http.Header {
    return rw.w.Header()
}

func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
    if rw.status == 0 {
        rw.status = http.StatusOK
    }
    return rw.w.Write(b)
}

func (rw *responseWriterWrapper) WriteHeader(statusCode int) {
    rw.status = statusCode
    rw.w.WriteHeader(statusCode)
}

// Implementar http.Hijacker para soportar WebSocket
func (rw *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
    if hijacker, ok := rw.w.(http.Hijacker); ok {
        return hijacker.Hijack()
    }
    return nil, nil, http.ErrNotSupported
}

// Implementar http.Flusher para soporte adicional
func (rw *responseWriterWrapper) Flush() {
    if flusher, ok := rw.w.(http.Flusher); ok {
        flusher.Flush()
    }
}

func getUserID(r *http.Request) string {
    if userID := r.Header.Get("X-User-ID"); userID != "" {
        return userID
    }
    if userID := r.URL.Query().Get("user_id"); userID != "" {
        return userID
    }
    return "anonimo"
}