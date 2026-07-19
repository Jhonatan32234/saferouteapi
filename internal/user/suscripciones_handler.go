package user

import (
	"encoding/json"
	"log"
	"net/http"

	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

func (h *Handler) SuscribirRutaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req struct {
			RutaID string `json:"ruta_id"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if req.RutaID == "" {
			common.WriteError(w, http.StatusBadRequest, "ruta_id es requerido")
			return
		}

		if err := h.userSvc.SubscribeRuta(userID, req.RutaID); err != nil {
			log.Printf("Error suscribiendo usuario %s a ruta %s: %v", userID, req.RutaID, err)
			common.WriteError(w, http.StatusInternalServerError, "error suscribiendo a la ruta")
			return
		}

		log.Printf("Usuario %s suscrito a ruta %s", userID, req.RutaID)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "suscrito",
			"ruta_id": req.RutaID,
		})
	}
}

func (h *Handler) DesuscribirRutaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		rutaID := r.URL.Query().Get("ruta_id")
		if rutaID == "" {
			common.WriteError(w, http.StatusBadRequest, "ruta_id es requerido")
			return
		}

		if err := h.userSvc.UnsubscribeRuta(userID, rutaID); err != nil {
			log.Printf("Error desuscribiendo usuario %s de ruta %s: %v", userID, rutaID, err)
			common.WriteError(w, http.StatusInternalServerError, "error desuscribiendo de la ruta")
			return
		}

		log.Printf("Usuario %s desuscrito de ruta %s", userID, rutaID)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "desuscrito",
			"ruta_id": rutaID,
		})
	}
}

func (h *Handler) GetSuscripcionesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		subs, err := h.userSvc.GetSubscriptions(userID)
		if err != nil {
			log.Printf("Error obteniendo suscripciones de usuario %s: %v", userID, err)
			common.WriteError(w, http.StatusInternalServerError, "error obteniendo suscripciones")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subs)
	}
}
