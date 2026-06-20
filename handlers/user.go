package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"saferoute/database"
	"saferoute/middleware"
	"saferoute/models"
)

// GetUserProfileHandler obtiene el perfil del usuario autenticado
func GetUserProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var profile models.UserProfile
		err := database.DB.QueryRow(
			`SELECT id, email, nombre, tipo, 
					COALESCE(telefono, '') as telefono,
					created_at, updated_at,
					COALESCE(ultimo_acceso, created_at) as ultimo_acceso
			 FROM usuarios 
			 WHERE id = $1`,
			userID,
		).Scan(
			&profile.ID, &profile.Email, &profile.Nombre,
			&profile.Tipo, &profile.Telefono,
			&profile.CreatedAt, &profile.UpdatedAt,
			&profile.UltimoAcceso,
		)

		if err != nil {
			log.Printf("ERROR obteniendo perfil: %v", err)
			writeError(w, http.StatusNotFound, "usuario no encontrado")
			return
		}

		// Obtener estadísticas del usuario
		database.DB.QueryRow(
			`SELECT COUNT(*) FROM reportes WHERE user_id = $1 AND vigente = TRUE`,
			userID,
		).Scan(&profile.ReportesCreados)

		database.DB.QueryRow(
			`SELECT COUNT(*) FROM reportes WHERE user_id = $1 AND confirmaciones > 0`,
			userID,
		).Scan(&profile.ReportesConfirmados)

		// Actualizar último acceso en background
		go func() {
			_, _ = database.DB.Exec(
				"UPDATE usuarios SET ultimo_acceso = $1 WHERE id = $2",
				time.Now(), userID,
			)
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

// UpdateUserProfileHandler actualiza el perfil del usuario
func UpdateUserProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req models.UpdateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// Construir consulta dinámica
		query := "UPDATE usuarios SET updated_at = NOW()"
		args := []interface{}{}
		argCount := 0

		if req.Nombre != "" {
			argCount++
			query += ", nombre = $" + strconv.Itoa(argCount)
			args = append(args, req.Nombre)
		}

		if req.Telefono != "" {
			argCount++
			query += ", telefono = $" + strconv.Itoa(argCount)
			args = append(args, req.Telefono)
		}

		if req.Email != "" {
			argCount++
			query += ", email = $" + strconv.Itoa(argCount)
			args = append(args, req.Email)
		}

		argCount++
		query += " WHERE id = $" + strconv.Itoa(argCount) + " RETURNING id"
		args = append(args, userID)

		var id string
		err := database.DB.QueryRow(query, args...).Scan(&id)
		if err != nil {
			log.Printf("ERROR actualizando perfil: %v", err)
			writeError(w, http.StatusInternalServerError, "error actualizando perfil")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "actualizado",
			"user_id": id,
		})
	}
}