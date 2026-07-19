package user

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

func (h *Handler) ActualizarZonasUsuarioHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			log.Printf("[ZONAS] UserID vacío")
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[ZONAS] Error leyendo body: %v", err)
			common.WriteError(w, http.StatusBadRequest, "error leyendo datos")
			return
		}
		defer r.Body.Close()
		
		log.Printf("[ZONAS] Body recibido: %s", string(body))

		var req struct {
			Zonas []ZonaRequest `json:"zonas"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			log.Printf("[ZONAS] Error decodificando JSON: %v", err)
			common.WriteError(w, http.StatusBadRequest, "datos inválidos: "+err.Error())
			return
		}

		if len(req.Zonas) == 0 {
			log.Printf("[ZONAS] No se enviaron zonas")
			common.WriteError(w, http.StatusBadRequest, "se requiere al menos una zona")
			return
		}

		log.Printf("[ZONAS] Usuario %s envió %d zonas", userID, len(req.Zonas))

		if err := h.userSvc.UpsertZonas(userID, req.Zonas); err != nil {
			log.Printf("[ZONAS] Error guardando zonas: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error actualizando zonas")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"mensaje": "Zonas actualizadas correctamente",
		})
	}
}

func (h *Handler) ObtenerZonasUsuarioHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		zonas, err := h.userSvc.GetZonas(userID)
		if err != nil {
			log.Printf("[ZONAS] Error obteniendo zonas: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error obteniendo zonas")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"zonas": zonas,
			"total": len(zonas),
		})
	}
}
