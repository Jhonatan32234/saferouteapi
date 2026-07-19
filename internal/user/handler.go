package user

import (
	"encoding/json"
	"log"
	"net/http"

	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

type Handler struct {
	userSvc Service
}

func NewHandler(userSvc Service) *Handler {
	return &Handler{userSvc: userSvc}
}

func (h *Handler) GetUserProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		profile, err := h.userSvc.GetProfile(userID)
		if err != nil {
			log.Printf("❌ [PROFILE] Error obteniendo usuario %s: %v", userID, err)
			common.WriteError(w, http.StatusNotFound, "usuario no encontrado")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

func (h *Handler) UpdateUserProfileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req UpdateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := h.userSvc.UpdateProfile(userID, req); err != nil {
			log.Printf("[PROFILE] Error actualizando usuario %s: %v", userID, err)
			common.WriteError(w, http.StatusInternalServerError, "error actualizando perfil")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "actualizado",
			"user_id": userID,
		})
	}
}

func (h *Handler) GetUsuariosInternoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conductores, err := h.userSvc.ListConductors()
		if err != nil {
			log.Printf("ERROR consultando usuarios: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error consultando usuarios")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"usuarios": conductores,
			"total":    len(conductores),
		})
	}
}
