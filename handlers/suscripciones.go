package handlers

import (
    "encoding/json"
    "log"
    "net/http"
    "time"

    "saferoute/database"
    "saferoute/middleware"
)

// SuscribirRutaHandler suscribe al usuario a una ruta específica
func SuscribirRutaHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r)
        if userID == "" {
            writeError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }

        var req struct {
            RutaID string `json:"ruta_id"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "datos inválidos")
            return
        }

        if req.RutaID == "" {
            writeError(w, http.StatusBadRequest, "ruta_id es requerido")
            return
        }

        db := database.GetDB()
        if db == nil {
            writeError(w, http.StatusInternalServerError, "error de base de datos")
            return
        }

        _, err := db.Exec(
            `INSERT INTO suscripciones_rutas (user_id, ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion)
             VALUES ($1, $2, true, NOW(), NOW())
             ON CONFLICT (user_id, ruta_id) 
             DO UPDATE SET suscrito = true, fecha_actualizacion = NOW()`,
            userID, req.RutaID,
        )
        if err != nil {
            log.Printf("❌ Error suscribiendo usuario %s a ruta %s: %v", userID, req.RutaID, err)
            writeError(w, http.StatusInternalServerError, "error suscribiendo a la ruta")
            return
        }

        log.Printf("✅ Usuario %s suscrito a ruta %s", userID, req.RutaID)
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status":  "suscrito",
            "ruta_id": req.RutaID,
        })
    }
}

// DesuscribirRutaHandler desuscribe al usuario de una ruta
func DesuscribirRutaHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r)
        if userID == "" {
            writeError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }

        rutaID := r.URL.Query().Get("ruta_id")
        if rutaID == "" {
            writeError(w, http.StatusBadRequest, "ruta_id es requerido")
            return
        }

        db := database.GetDB()
        if db == nil {
            writeError(w, http.StatusInternalServerError, "error de base de datos")
            return
        }

        _, err := db.Exec(
            `UPDATE suscripciones_rutas 
             SET suscrito = false, fecha_actualizacion = NOW()
             WHERE user_id = $1 AND ruta_id = $2`,
            userID, rutaID,
        )
        if err != nil {
            log.Printf("❌ Error desuscribiendo usuario %s de ruta %s: %v", userID, rutaID, err)
            writeError(w, http.StatusInternalServerError, "error desuscribiendo de la ruta")
            return
        }

        log.Printf("✅ Usuario %s desuscrito de ruta %s", userID, rutaID)
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status":  "desuscrito",
            "ruta_id": rutaID,
        })
    }
}

// GetSuscripcionesHandler obtiene todas las suscripciones del usuario
func GetSuscripcionesHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r)
        if userID == "" {
            writeError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }

        db := database.GetDB()
        if db == nil {
            writeError(w, http.StatusInternalServerError, "error de base de datos")
            return
        }

        rows, err := db.Query(
            `SELECT ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion
             FROM suscripciones_rutas
             WHERE user_id = $1`,
            userID,
        )
        if err != nil {
            log.Printf("❌ Error obteniendo suscripciones: %v", err)
            writeError(w, http.StatusInternalServerError, "error obteniendo suscripciones")
            return
        }
        defer rows.Close()

        var suscripciones []map[string]interface{}
        for rows.Next() {
            var rutaID string
            var suscrito bool
            var fechaSuscripcion, fechaActualizacion time.Time
            
            if err := rows.Scan(&rutaID, &suscrito, &fechaSuscripcion, &fechaActualizacion); err != nil {
                continue
            }
            
            suscripciones = append(suscripciones, map[string]interface{}{
                "ruta_id":             rutaID,
                "suscrito":            suscrito,
                "fecha_suscripcion":   fechaSuscripcion,
                "fecha_actualizacion": fechaActualizacion,
            })
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "suscripciones": suscripciones,
            "total":         len(suscripciones),
        })
    }
}