package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"saferoute/database"
	"saferoute/middleware"
	"saferoute/models"
)

// GetHistorialNotificacionesHandler obtiene el historial de notificaciones del usuario
func GetHistorialNotificacionesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Obtener user_id del contexto
		userID := middleware.GetUserID(r)
		
		if userID == "" {
			log.Printf("❌ [HISTORIAL] UserID vacío - no autenticado")
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		if err := database.EnsureConnection(); err != nil {
            log.Printf("❌ [HISTORIAL] Error de conexión: %v", err)
            writeError(w, http.StatusInternalServerError, "error de base de datos")
            return
        }

		db := database.GetDB()
        if db == nil {
            writeError(w, http.StatusInternalServerError, "base de datos no disponible")
            return
        }

		// Parámetros de paginación
		paginaStr := r.URL.Query().Get("pagina")
		limiteStr := r.URL.Query().Get("limite")
		soloNoLeidas := r.URL.Query().Get("no_leidas") == "true"

		pagina := 1
		limite := 20

		if p, err := strconv.Atoi(paginaStr); err == nil && p > 0 {
			pagina = p
		}
		if l, err := strconv.Atoi(limiteStr); err == nil && l > 0 && l <= 100 {
			limite = l
		}

		offset := (pagina - 1) * limite

		log.Printf("📊 [HISTORIAL] Consultando notificaciones para usuario %s (pagina=%d, limite=%d)", 
			userID, pagina, limite)

		// CORREGIDO: Usar NULL en lugar de '' para UUIDs
		query := `SELECT id, user_id, tipo, 
					COALESCE(reporte_id, NULL) as reporte_id,
					COALESCE(latitud, 0) as latitud, 
					COALESCE(longitud, 0) as longitud,
					COALESCE(nota_voz, '') as nota_voz, 
					ruta_id, mensaje,
					leida, fecha_envio, fecha_lectura
				  FROM notificaciones_historial 
				  WHERE user_id = $1`

		args := []interface{}{userID}
		argCount := 1

		if soloNoLeidas {
			argCount++
			query += " AND leida = $" + strconv.Itoa(argCount)
			args = append(args, false)
		}

		// Contar total
		countQuery := `SELECT COUNT(*) FROM notificaciones_historial WHERE user_id = $1`
		if soloNoLeidas {
			countQuery += " AND leida = false"
		}

		var total int
		err := database.DB.QueryRow(countQuery, userID).Scan(&total)
		if err != nil {
			log.Printf("❌ [HISTORIAL] Error contando notificaciones: %v", err)
			writeError(w, http.StatusInternalServerError, "error obteniendo notificaciones")
			return
		}

		// Obtener notificaciones no leídas
		var noLeidas int
		err = database.DB.QueryRow(
			"SELECT COUNT(*) FROM notificaciones_historial WHERE user_id = $1 AND leida = false",
			userID,
		).Scan(&noLeidas)
		if err != nil {
			noLeidas = 0
		}

		// Ordenar y paginar
		argCount++
		query += " ORDER BY fecha_envio DESC LIMIT $" + strconv.Itoa(argCount)
		args = append(args, limite)

		argCount++
		query += " OFFSET $" + strconv.Itoa(argCount)
		args = append(args, offset)

		rows, err := database.DB.Query(query, args...)
		if err != nil {
			log.Printf("❌ [HISTORIAL] Error consultando notificaciones: %v", err)
			writeError(w, http.StatusInternalServerError, "error obteniendo notificaciones")
			return
		}
		defer rows.Close()

		var notificaciones []models.NotificacionHistorial
		for rows.Next() {
			var n models.NotificacionHistorial
			var fechaLectura *time.Time
			var reporteID *string // Usar puntero para manejar NULL

			err := rows.Scan(
				&n.ID, &n.UserID, &n.Tipo, &reporteID,
				&n.Latitud, &n.Longitud, &n.NotaVoz,
				&n.RutaID, &n.Mensaje, &n.Leida,
				&n.FechaEnvio, &fechaLectura,
			)
			if err != nil {
				log.Printf("⚠️ [HISTORIAL] Error escaneando notificación: %v", err)
				continue
			}

			// Si reporteID es NULL, asignar string vacío
			if reporteID != nil {
				n.ReporteID = *reporteID
			} else {
				n.ReporteID = ""
			}

			if fechaLectura != nil {
				n.FechaLectura = fechaLectura
			}

			notificaciones = append(notificaciones, n)
		}

		totalPaginas := (total + limite - 1) / limite
		if totalPaginas == 0 {
			totalPaginas = 1
		}

		log.Printf("✅ [HISTORIAL] Encontradas %d notificaciones para usuario %s", 
			len(notificaciones), userID)

		response := models.NotificacionHistorialResponse{
			Notificaciones: notificaciones,
			Total:          total,
			NoLeidas:       noLeidas,
			Pagina:         pagina,
			TotalPaginas:   totalPaginas,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
// handlers/notificaciones.go - MarcarNotificacionHandler CORREGIDO

func MarcarNotificacionHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r)
        if userID == "" {
            log.Printf("❌ [MARCAR] UserID vacío - no autenticado")
            writeError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }

        // Obtener ID de la query string
        notificacionID := r.URL.Query().Get("id")
        
        // Si no viene en query, intentar del body
        if notificacionID == "" {
            var bodyReq struct {
                NotificacionID string `json:"notificacion_id"`
            }
            if err := json.NewDecoder(r.Body).Decode(&bodyReq); err == nil {
                notificacionID = bodyReq.NotificacionID
            }
        }

        if notificacionID == "" {
            log.Printf("❌ [MARCAR] ID de notificación no proporcionado")
            writeError(w, http.StatusBadRequest, "id de notificación requerido")
            return
        }

        log.Printf("📝 [MARCAR] Marcando notificación %s como leída para usuario %s", notificacionID, userID)

        // Decodificar body (leida true/false)
        var req models.MarcarNotificacionRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            // Si no se puede decodificar, usar true por defecto
            req.Leida = true
        }

        // Actualizar usando database.DB directamente
        query := `UPDATE notificaciones_historial 
                  SET leida = $1, fecha_lectura = CASE WHEN $1 THEN NOW() ELSE NULL END
                  WHERE id = $2 AND user_id = $3
                  RETURNING id`

        var id string
        err := database.DB.QueryRow(query, req.Leida, notificacionID, userID).Scan(&id)
        if err != nil {
            if err == sql.ErrNoRows {
                log.Printf("❌ [MARCAR] Notificación %s no encontrada para usuario %s", notificacionID, userID)
                writeError(w, http.StatusNotFound, "notificación no encontrada")
                return
            }
            log.Printf("❌ [MARCAR] Error actualizando notificación: %v", err)
            writeError(w, http.StatusInternalServerError, "error actualizando notificación")
            return
        }

        log.Printf("✅ [MARCAR] Notificación %s marcada como leida=%v", id, req.Leida)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":          "actualizado",
            "notificacion_id": id,
            "leida":           req.Leida,
        })
    }
}

// MarcarTodasNotificacionesHandler marca todas las notificaciones como leídas
func MarcarTodasNotificacionesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		result, err := database.DB.Exec(
			`UPDATE notificaciones_historial 
			 SET leida = true, fecha_lectura = NOW()
			 WHERE user_id = $1 AND leida = false`,
			userID,
		)
		if err != nil {
			log.Printf("❌ [MARCAR_TODAS] Error marcando todas las notificaciones: %v", err)
			writeError(w, http.StatusInternalServerError, "error actualizando notificaciones")
			return
		}

		rowsAffected, _ := result.RowsAffected()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "actualizado",
			"marcadas": rowsAffected,
			"mensaje":  "Todas las notificaciones marcadas como leídas",
		})
	}
}

// SincronizarNotificacionesHandler sincroniza notificaciones desde última fecha
func SincronizarNotificacionesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req struct {
			UltimaSincronizacion time.Time `json:"ultima_sincronizacion"`
		}

		body := json.NewDecoder(r.Body)
		if err := body.Decode(&req); err != nil {
			// Si no hay body, usar timestamp de hace 24 horas
			req.UltimaSincronizacion = time.Now().Add(-24 * time.Hour)
		}

		// CORREGIDO: Usar NULL en lugar de '' para UUIDs
		rows, err := database.DB.Query(
			`SELECT id, user_id, tipo, 
					COALESCE(reporte_id, NULL) as reporte_id,
					COALESCE(latitud, 0) as latitud, 
					COALESCE(longitud, 0) as longitud,
					COALESCE(nota_voz, '') as nota_voz, 
					ruta_id, mensaje,
					leida, fecha_envio, fecha_lectura
			 FROM notificaciones_historial 
			 WHERE user_id = $1 AND fecha_envio > $2
			 ORDER BY fecha_envio DESC
			 LIMIT 100`,
			userID, req.UltimaSincronizacion,
		)
		if err != nil {
			log.Printf("❌ [SINCRONIZAR] Error sincronizando notificaciones: %v", err)
			writeError(w, http.StatusInternalServerError, "error sincronizando")
			return
		}
		defer rows.Close()

		var notificaciones []models.NotificacionHistorial
		for rows.Next() {
			var n models.NotificacionHistorial
			var fechaLectura *time.Time
			var reporteID *string

			err := rows.Scan(
				&n.ID, &n.UserID, &n.Tipo, &reporteID,
				&n.Latitud, &n.Longitud, &n.NotaVoz,
				&n.RutaID, &n.Mensaje, &n.Leida,
				&n.FechaEnvio, &fechaLectura,
			)
			if err != nil {
				continue
			}

			if reporteID != nil {
				n.ReporteID = *reporteID
			} else {
				n.ReporteID = ""
			}

			if fechaLectura != nil {
				n.FechaLectura = fechaLectura
			}
			notificaciones = append(notificaciones, n)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"notificaciones": notificaciones,
			"total":          len(notificaciones),
			"timestamp":      time.Now(),
		})
	}
}