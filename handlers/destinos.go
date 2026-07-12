package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"saferoute/database"
	"saferoute/middleware"
)

// DestinoRecienteRequest is the payload for saving a recent destination
type DestinoRecienteRequest struct {
	Nombre string  `json:"nombre"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// DestinoRecienteResponse is the response for a recent destination
type DestinoRecienteResponse struct {
	ID           string    `json:"id"`
	Nombre       string    `json:"nombre"`
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	FechaCreacion time.Time `json:"fecha_creacion"`
}

// GuardarDestinoRecenteHandler saves or updates a recent destination for the authenticated user
func GuardarDestinoRecenteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req DestinoRecienteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if req.Nombre == "" {
			writeError(w, http.StatusBadRequest, "nombre del destino es requerido")
			return
		}

		if err := database.EnsureConnection(); err != nil {
			log.Printf("❌ [DESTINO-GUARDAR] Error de conexión: %v", err)
			writeError(w, http.StatusInternalServerError, "error de base de datos")
			return
		}

		// Avoid duplicates: if same nombre exists, update it (refresh fecha + coordinates)
		var existingID string
		err := database.DB.QueryRow(
			`SELECT id FROM historial_destinos WHERE user_id = $1 AND nombre = $2`,
			userID, req.Nombre,
		).Scan(&existingID)

		if err == sql.ErrNoRows {
			// Insert new
			_, err = database.DB.Exec(
				`INSERT INTO historial_destinos (user_id, nombre, latitud, longitud, fecha_creacion)
				 VALUES ($1, $2, $3, $4, NOW())`,
				userID, req.Nombre, req.Lat, req.Lon,
			)
			if err != nil {
				log.Printf("❌ [DESTINO-GUARDAR] Error insertando destino: %v", err)
				writeError(w, http.StatusInternalServerError, "error guardando destino")
				return
			}
			log.Printf("[DESTINO-GUARDAR] Destino '%s' guardado para usuario %s", req.Nombre, userID)
		} else if err == nil {
			// Update existing: refresh coordinates and fecha
			_, err = database.DB.Exec(
				`UPDATE historial_destinos SET latitud = $1, longitud = $2, fecha_creacion = NOW()
				 WHERE id = $3 AND user_id = $4`,
				req.Lat, req.Lon, existingID, userID,
			)
			if err != nil {
				log.Printf("❌ [DESTINO-GUARDAR] Error actualizando destino: %v", err)
				writeError(w, http.StatusInternalServerError, "error actualizando destino")
				return
			}
			log.Printf("[DESTINO-GUARDAR] Destino '%s' actualizado para usuario %s", req.Nombre, userID)
		} else {
			log.Printf("❌ [DESTINO-GUARDAR] Error consultando destino existente: %v", err)
			writeError(w, http.StatusInternalServerError, "error de base de datos")
			return
		}

		// Keep only the latest 10 destinations per user
		_, err = database.DB.Exec(`
			DELETE FROM historial_destinos
			WHERE user_id = $1 AND id NOT IN (
				SELECT id FROM historial_destinos
				WHERE user_id = $1
				ORDER BY fecha_creacion DESC
				LIMIT 10
			)`,
			userID,
		)
		if err != nil {
			log.Printf("⚠️ [DESTINO-GUARDAR] Error limpiando destinos antiguos: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"mensaje": "Destino guardado correctamente",
		})
	}
}

// GetDestinosRecientesHandler returns the recent destinations for the authenticated user
func GetDestinosRecientesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		if err := database.EnsureConnection(); err != nil {
			log.Printf("❌ [DESTINO-LISTAR] Error de conexión: %v", err)
			writeError(w, http.StatusInternalServerError, "error de base de datos")
			return
		}

		limite := 10
		limiteStr := r.URL.Query().Get("limite")
		if l, err := parseInt(limiteStr); err == nil && l > 0 && l <= 50 {
			limite = l
		}

		rows, err := database.DB.Query(
			`SELECT id, user_id, nombre, latitud, longitud, fecha_creacion
			 FROM historial_destinos
			 WHERE user_id = $1
			 ORDER BY fecha_creacion DESC
			 LIMIT $2`,
			userID, limite,
		)
		if err != nil {
			log.Printf("❌ [DESTINO-LISTAR] Error consultando destinos: %v", err)
			writeError(w, http.StatusInternalServerError, "error obteniendo destinos")
			return
		}
		defer rows.Close()

		var destinos []DestinoRecienteResponse
		for rows.Next() {
			var d DestinoRecienteResponse
			var userIDStr string
			if err := rows.Scan(&d.ID, &userIDStr, &d.Nombre, &d.Lat, &d.Lon, &d.FechaCreacion); err != nil {
				log.Printf("⚠️ [DESTINO-LISTAR] Error escaneando destino: %v", err)
				continue
			}
			destinos = append(destinos, d)
		}

		if destinos == nil {
			destinos = []DestinoRecienteResponse{}
		}

		log.Printf("[DESTINO-LISTAR] Encontrados %d destinos recientes para usuario %s", len(destinos), userID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"destinos": destinos,
			"total":    len(destinos),
		})
	}
}

// EliminarDestinoRecenteHandler removes a specific destination from history
func EliminarDestinoRecenteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		destinoID := r.URL.Query().Get("id")
		if destinoID == "" {
			var body struct {
				DestinoID string `json:"destino_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				destinoID = body.DestinoID
			}
		}

		if destinoID == "" {
			writeError(w, http.StatusBadRequest, "id del destino es requerido")
			return
		}

		if err := database.EnsureConnection(); err != nil {
			log.Printf("❌ [DESTINO-ELIMINAR] Error de conexión: %v", err)
			writeError(w, http.StatusInternalServerError, "error de base de datos")
			return
		}

		result, err := database.DB.Exec(
			`DELETE FROM historial_destinos WHERE id = $1 AND user_id = $2`,
			destinoID, userID,
		)
		if err != nil {
			log.Printf("❌ [DESTINO-ELIMINAR] Error eliminando destino: %v", err)
			writeError(w, http.StatusInternalServerError, "error eliminando destino")
			return
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			writeError(w, http.StatusNotFound, "destino no encontrado")
			return
		}

		log.Printf("[DESTINO-ELIMINAR] Destino %s eliminado para usuario %s", destinoID, userID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "eliminado",
			"mensaje": "Destino eliminado del historial",
		})
	}
}

// parseInt helper to avoid importing strconv in some contexts
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid digit: %c", c)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}