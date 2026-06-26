package handlers

import (
	"encoding/json"
	"net/http"
	"saferoute/database"
	"time"
)

// HealthHandler retorna el estado de salud del sistema
func HealthHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        // Si es HEAD, solo responde con 200 y headers
        if r.Method == http.MethodHead {
            w.WriteHeader(http.StatusOK)
            return
        }

        dbStatus := "connected"
        if database.DB == nil {
            dbStatus = "disconnected"
        } else if err := database.DB.Ping(); err != nil {
            dbStatus = "error: " + err.Error()
        }

        response := map[string]interface{}{
            "status":    "ok",
            "version":   "1.0.0",
            "timestamp": time.Now().Format(time.RFC3339),
            "database":  dbStatus,
        }

        json.NewEncoder(w).Encode(response)
    }
}

