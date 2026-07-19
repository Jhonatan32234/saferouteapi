package user

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

func (h *Handler) GuardarDestinoRecenteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req DestinoRecienteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if req.Nombre == "" {
			common.WriteError(w, http.StatusBadRequest, "nombre del destino es requerido")
			return
		}

		if err := h.userSvc.SaveDestino(userID, req); err != nil {
			log.Printf("❌ [DESTINO-GUARDAR] Error guardando: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error guardando destino")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"mensaje": "Destino guardado correctamente",
		})
	}
}

func (h *Handler) GetDestinosRecientesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		limite := 10
		limiteStr := r.URL.Query().Get("limite")
		if l, err := strconv.Atoi(limiteStr); err == nil && l > 0 && l <= 50 {
			limite = l
		}

		destinos, err := h.userSvc.GetDestinos(userID, limite)
		if err != nil {
			log.Printf("❌ [DESTINO-LISTAR] Error obteniendo: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error obteniendo destinos")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(destinos)
	}
}

func (h *Handler) EliminarDestinoRecenteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		destinoID := r.URL.Query().Get("id")
		if destinoID == "" {
			var req struct {
				ID string `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				destinoID = req.ID
			}
		}

		if destinoID == "" {
			common.WriteError(w, http.StatusBadRequest, "id del destino es requerido")
			return
		}

		if err := h.userSvc.DeleteDestino(userID, destinoID); err != nil {
			log.Printf("❌ [DESTINO-ELIMINAR] Error eliminando: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error eliminando destino")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"mensaje": "Destino eliminado correctamente",
		})
	}
}
