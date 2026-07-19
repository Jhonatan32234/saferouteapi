package middleware

import (
    "net/http"
    "os"
)

func InternalAPIKeyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-Internal-API-Key")
        expectedKey := os.Getenv("INTERNAL_API_KEY")
        if expectedKey == "" {
            expectedKey = "my_api_key"
        }

        if apiKey != expectedKey {
            http.Error(w, `{"error":"acceso interno no autorizado"}`, http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
