// middleware/logging.go - Interceptor de Respuesta completo (Auditoría)
package middleware

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"time"
)

// LoggingMiddleware actúa como Interceptor de Respuesta:
// captura el código de estado y el tiempo de procesamiento de CADA petición.
// Registra el par [Método Ruta → StatusCode Tiempo UserID] para auditoría.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		if r.Method == "OPTIONS" {
            next.ServeHTTP(w, r)
            return
        }

		// Envolver el ResponseWriter para capturar el status code de salida
		wrapped := &responseWriterWrapper{
			w:      w,
			status: http.StatusOK, // Default si no se llama a WriteHeader
		}

		// Continuar la cadena de middlewares/handlers
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		userID := extractUserID(r)

		// Clasificar el nivel de log según el status code de la respuesta
		switch {
		case wrapped.status >= 500:
			log.Printf("🔴 AUDIT | user:%-36s | %-6s %-40s | %d | %v | ip:%s",
				userID, r.Method, r.URL.Path, wrapped.status, duration, r.RemoteAddr)
		case wrapped.status >= 400:
			log.Printf("🟡 AUDIT | user:%-36s | %-6s %-40s | %d | %v | ip:%s",
				userID, r.Method, r.URL.Path, wrapped.status, duration, r.RemoteAddr)
		default:
			log.Printf("🟢 AUDIT | user:%-36s | %-6s %-40s | %d | %v | ip:%s",
				userID, r.Method, r.URL.Path, wrapped.status, duration, r.RemoteAddr)
		}
	})
}

// responseWriterWrapper intercepta la escritura de la respuesta para capturar
// el código de estado sin modificar el comportamiento del ResponseWriter original.
// Implementa http.Hijacker (para WebSocket) y http.Flusher (para streaming).
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

// Hijack implementa http.Hijacker para mantener compatibilidad con WebSocket.
func (rw *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.w.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Flush implementa http.Flusher para compatibilidad con streaming / SSE.
func (rw *responseWriterWrapper) Flush() {
	if flusher, ok := rw.w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// extractUserID intenta obtener el UserID del contexto JWT, del header o del query param.
func extractUserID(r *http.Request) string {
	// Primero intentar desde el contexto (inyectado por AuthMiddleware)
	if userID, ok := r.Context().Value(UserIDKey).(string); ok && userID != "" {
		return userID
	}
	// Fallback a headers personalizados
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}
	// Fallback a query param (WebSocket)
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		return userID
	}
	return "anónimo"
}