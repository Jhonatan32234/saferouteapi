package user

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

func (h *Handler) GetHistorialNotificacionesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			log.Printf("❌ [HISTORIAL] UserID vacío - no autenticado")
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

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

		log.Printf("[HISTORIAL] Consultando notificaciones para usuario %s (pagina=%d, limite=%d)", 
			userID, pagina, limite)

		resp, err := h.userSvc.GetNotifications(userID, pagina, limite, soloNoLeidas)
		if err != nil {
			log.Printf("[HISTORIAL] Error consultando notificaciones: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error obteniendo notificaciones")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func (h *Handler) MarcarNotificacionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			log.Printf("❌ [MARCAR] UserID vacío - no autenticado")
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		notificacionID := r.URL.Query().Get("id")
		if notificacionID == "" {
			var bodyReq struct {
				NotificacionID string `json:"notificacion_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&bodyReq); err == nil {
				notificacionID = bodyReq.NotificacionID
			}
		}

		if notificacionID == "" {
			log.Printf("[MARCAR] ID de notificación no proporcionado")
			common.WriteError(w, http.StatusBadRequest, "id de notificación requerido")
			return
		}

		var req MarcarNotificacionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.Leida = true
		}

		log.Printf("[MARCAR] Marcando notificación %s como leída=%v para usuario %s", notificacionID, req.Leida, userID)

		if err := h.userSvc.MarkNotification(userID, notificacionID, req.Leida); err != nil {
			log.Printf("❌ [MARCAR] Error actualizando notificación: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error actualizando notificación")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "actualizado",
			"notificacion_id": notificacionID,
			"leida":           req.Leida,
		})
	}
}

func (h *Handler) MarcarTodasNotificacionesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		if err := h.userSvc.MarkAllNotificationsRead(userID); err != nil {
			log.Printf("❌ [MARCAR_TODAS] Error marcando todas las notificaciones: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error actualizando notificaciones")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "actualizado",
			"mensaje":  "Todas las notificaciones marcadas como leídas",
		})
	}
}

func (h *Handler) SincronizarNotificacionesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req struct {
			UltimaSincronizacion time.Time `json:"ultima_sincronizacion"`
		}

		body := json.NewDecoder(r.Body)
		if err := body.Decode(&req); err != nil {
			req.UltimaSincronizacion = time.Now().Add(-24 * time.Hour)
		}

		notificaciones, err := h.userSvc.SyncNotifications(userID, req.UltimaSincronizacion)
		if err != nil {
			log.Printf("❌ [SINCRONIZAR] Error sincronizando notificaciones: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error sincronizando")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"notificaciones": notificaciones,
			"total":          len(notificaciones),
			"timestamp":      time.Now(),
		})
	}
}
