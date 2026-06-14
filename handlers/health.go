package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler retorna el estado de salud del sistema
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Por ahora retornamos un estado simple.
		// En una fase más avanzada, podrías verificar la conexión a la DB aquí.
		response := map[string]interface{}{
			"status":    "ok",
			"version":   "1.0.0",
			"timestamp": time.Now().Format(time.RFC3339),
			"database":  "connected",
		}

		json.NewEncoder(w).Encode(response)
	}
}
