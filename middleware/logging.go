package middleware

import (
    "log"
    "net/http"
    "time"
)

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        next.ServeHTTP(rw, r)
        
        userID := GetUserID(r)
        if userID == "" {
            userID = "anonimo"
        }
        
        log.Printf("AUDIT | user:%s | %s %s | %d | %v | ip:%s",
            userID,
            r.Method,
            r.URL.Path,
            rw.statusCode,
            time.Since(start),
            r.RemoteAddr,
        )
    })
}