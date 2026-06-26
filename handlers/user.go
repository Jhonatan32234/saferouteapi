package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"saferoute/middleware"
	"saferoute/models"
	"saferoute/services"
)

// GetUserProfileHandler recupera el perfil del usuario delegando al servicio.
func GetUserProfileHandler(userSvc *services.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		profile, err := userSvc.GetProfile(userID)
		if err != nil {
			log.Printf("❌ [PROFILE] Error obteniendo usuario %s: %v", userID, err)
			writeError(w, http.StatusNotFound, "usuario no encontrado")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

// UpdateUserProfileHandler actualiza campos del perfil delegando al servicio.
func UpdateUserProfileHandler(userSvc *services.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		// 1. Decodificar DTO de entrada
		var req models.UpdateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// 2. Service: aplicar actualización y persistir
		if err := userSvc.UpdateProfile(userID, req); err != nil {
			log.Printf("❌ [PROFILE] Error actualizando usuario %s: %v", userID, err)
			writeError(w, http.StatusInternalServerError, "error actualizando perfil")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "actualizado",
			"user_id": userID,
		})
	}
}