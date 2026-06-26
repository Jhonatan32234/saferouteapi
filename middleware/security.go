package middleware

import (
	"net/http"
	"strings"
)

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OWASP Top 10 headers comunes
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Cache-Control", "no-store, max-age=0")

		// Configuración dinámica de Content-Security-Policy (CSP)
		// Cambia "/static/" o "/docs" según la ruta exacta que uses en tu router
		if strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/docs") {
			// Swagger UI necesita 'unsafe-inline' para sus estilos y scripts, 
			// y también fuentes/imágenes si se cargan desde CDNs externos.
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: https://fastapi.tiangolo.com;")
		} else {
			// Política estricta por defecto para el resto de la API
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
		}
		
		next.ServeHTTP(w, r)
	})
}