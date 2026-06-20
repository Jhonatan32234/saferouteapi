
package handlers

import (
    "encoding/json"
    "io"
    "log"
    "net/http"
    "time"
    
    "saferoute/database"
    "saferoute/middleware"
)

type ZonaUsuario struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    ZonaNombre  string    `json:"zona_nombre"`
    Latitud     float64   `json:"latitud"`
    Longitud    float64   `json:"longitud"`
    RadioKm     float64   `json:"radio_km"`
    Activo      bool      `json:"activo"`
}

type ZonaRequest struct {
    ZonaNombre string  `json:"zona_nombre"`
    Latitud    float64 `json:"latitud"`
    Longitud   float64 `json:"longitud"`
    RadioKm    float64 `json:"radio_km,omitempty"`
}

// handlers/zonas.go - ActualizarZonasUsuarioHandler corregido

func ActualizarZonasUsuarioHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r)
        if userID == "" {
            log.Printf("❌ [ZONAS] UserID vacío")
            writeError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }

        // Leer el body
        body, err := io.ReadAll(r.Body)
        if err != nil {
            log.Printf("❌ [ZONAS] Error leyendo body: %v", err)
            writeError(w, http.StatusBadRequest, "error leyendo datos")
            return
        }
        defer r.Body.Close()
        
        log.Printf("📝 [ZONAS] Body recibido: %s", string(body))

        // Decodificar el body
        var req struct {
            Zonas []ZonaRequest `json:"zonas"`
        }

        if err := json.Unmarshal(body, &req); err != nil {
            log.Printf("❌ [ZONAS] Error decodificando JSON: %v", err)
            writeError(w, http.StatusBadRequest, "datos inválidos: "+err.Error())
            return
        }

        if len(req.Zonas) == 0 {
            log.Printf("⚠️ [ZONAS] No se enviaron zonas")
            writeError(w, http.StatusBadRequest, "se requiere al menos una zona")
            return
        }

        log.Printf("📊 [ZONAS] Usuario %s envió %d zonas", userID, len(req.Zonas))

        db := database.GetDB()
        if db == nil {
            log.Printf("❌ [ZONAS] Base de datos no conectada")
            writeError(w, http.StatusInternalServerError, "error de base de datos")
            return
        }

        // Iniciar transacción
        tx, err := db.Begin()
        if err != nil {
            log.Printf("❌ [ZONAS] Error iniciando transacción: %v", err)
            writeError(w, http.StatusInternalServerError, "error actualizando zonas")
            return
        }
        defer tx.Rollback()

        // IMPORTANTE: Usar UPSERT (INSERT ... ON CONFLICT DO UPDATE)
        // En lugar de DELETE + INSERT para evitar problemas de concurrencia
        for i, zona := range req.Zonas {
            radio := zona.RadioKm
            if radio == 0 {
                radio = 15.0
            }

            log.Printf("  📍 [ZONAS] [%d/%d] Upsert zona: %s (%.6f, %.6f) radio=%.1fkm", 
                i+1, len(req.Zonas), zona.ZonaNombre, zona.Latitud, zona.Longitud, radio)

            // UPSERT: Si existe, actualiza; si no, inserta
            _, err = tx.Exec(
                `INSERT INTO zonas_usuario (user_id, zona_nombre, latitud, longitud, radio_km, activo, fecha_actualizacion)
                 VALUES ($1, $2, $3, $4, $5, true, NOW())
                 ON CONFLICT (user_id, zona_nombre) 
                 DO UPDATE SET 
                    latitud = EXCLUDED.latitud,
                    longitud = EXCLUDED.longitud,
                    radio_km = EXCLUDED.radio_km,
                    activo = true,
                    fecha_actualizacion = NOW()`,
                userID, zona.ZonaNombre, zona.Latitud, zona.Longitud, radio,
            )
            if err != nil {
                log.Printf("❌ [ZONAS] Error en UPSERT para %s: %v", zona.ZonaNombre, err)
                writeError(w, http.StatusInternalServerError, "error actualizando zonas")
                return
            }
        }

        // Commit de la transacción
        if err = tx.Commit(); err != nil {
            log.Printf("❌ [ZONAS] Error commit transacción: %v", err)
            writeError(w, http.StatusInternalServerError, "error actualizando zonas")
            return
        }

        log.Printf("✅ [ZONAS] Usuario %s actualizó %d zonas", userID, len(req.Zonas))

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "actualizado",
            "total":  len(req.Zonas),
        })
    }
}

// ObtenerZonasUsuarioHandler obtiene las zonas del usuario
func ObtenerZonasUsuarioHandler() http.HandlerFunc {
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
            `SELECT id, zona_nombre, latitud, longitud, radio_km, activo, fecha_creacion
             FROM zonas_usuario
             WHERE user_id = $1 AND activo = true
             ORDER BY zona_nombre`,
            userID,
        )
        if err != nil {
            log.Printf("❌ [ZONAS] Error obteniendo zonas: %v", err)
            writeError(w, http.StatusInternalServerError, "error obteniendo zonas")
            return
        }
        defer rows.Close()

        var zonas []ZonaUsuario
        for rows.Next() {
            var z ZonaUsuario
            var fechaCreacion time.Time
            err := rows.Scan(&z.ID, &z.ZonaNombre, &z.Latitud, &z.Longitud, 
                &z.RadioKm, &z.Activo, &fechaCreacion)
            if err != nil {
                continue
            }
            z.UserID = userID
            zonas = append(zonas, z)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "zonas": zonas,
            "total": len(zonas),
        })
    }
}