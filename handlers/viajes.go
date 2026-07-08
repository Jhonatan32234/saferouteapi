package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"saferoute/middleware"
	"saferoute/models"
	"saferoute/pipes"
	"saferoute/services"
)

type ViajesHandler struct {
	viajeSvc *services.ViajeService
}

func NewViajesHandler(viajeSvc *services.ViajeService) *ViajesHandler {
	return &ViajesHandler{viajeSvc: viajeSvc}
}

func (h *ViajesHandler) IniciarViajeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeErrorLocal(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req models.IniciarViajeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErrorLocal(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := pipes.ValidateIniciarViaje(&req); err != nil {
			writeErrorLocal(w, http.StatusBadRequest, err.Error())
			return
		}

		viajeID, err := h.viajeSvc.IniciarViaje(userID, req)
		if err != nil {
			log.Printf("❌ [VIAJES] Error iniciando viaje para %s: %v", userID, err)
			writeErrorLocal(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "activo",
			"viaje_id": viajeID,
		})
	}
}

func (h *ViajesHandler) FinalizarViajeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeErrorLocal(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req models.FinalizarViajeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErrorLocal(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := pipes.ValidateFinalizarViaje(&req); err != nil {
			writeErrorLocal(w, http.StatusBadRequest, err.Error())
			return
		}

		estadoFinal, err := h.viajeSvc.FinalizarViaje(userID, req)
		if err != nil {
			log.Printf("❌ [VIAJES] Error finalizando viaje %s para %s: %v", req.ViajeID, userID, err)
			writeErrorLocal(w, http.StatusBadRequest, err.Error())
			return
		}

		var mensaje string
		var tipoAlerta string

		if estadoFinal == "cancelado" {
			mensaje = fmt.Sprintf("El conductor %s canceló su viaje de forma anticipada", userID)
			tipoAlerta = "warning"
		} else {
			mensaje = fmt.Sprintf("El conductor %s llegó a su destino exitosamente", userID)
			tipoAlerta = "info"
		}

		eventoAdmin := map[string]interface{}{
			"tipo":       "viaje_finalizado",
			"user_id":    userID,
			"viaje_id":   req.ViajeID,
			"estado":     estadoFinal,
			"mensaje":    mensaje,
			"tipo_alerta": tipoAlerta,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		}

		go BroadcastAdminMonitor(eventoAdmin)
		log.Printf("[VIAJES] Evento de finalización (%s) emitido para viaje %s del conductor %s", estadoFinal, req.ViajeID, userID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  estadoFinal,
			"mensaje": mensaje,
		})
	}
}

func (h *ViajesHandler) GetActiveViajeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			writeErrorLocal(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		viaje, err := h.viajeSvc.GetActiveViaje(userID)
		if err != nil {
			log.Printf("[VIAJES] Error consultando viaje activo de %s: %v", userID, err)
			writeErrorLocal(w, http.StatusNotFound, "no se encontró un viaje activo")
			return
		}

		if viaje == nil {
			writeErrorLocal(w, http.StatusNotFound, "no tienes un viaje activo actualmente")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(viaje)
	}
}

func (h *ViajesHandler) GetActiveViajesAdminHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viajes, err := h.viajeSvc.GetActiveViajesAdmin()
		if err != nil {
			log.Printf("[ADMIN-VIAJES] Error consultando viajes activos: %v", err)
			writeErrorLocal(w, http.StatusInternalServerError, "error consultando viajes activos")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(viajes)
	}
}

func writeErrorLocal(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
		Code:  code,
	})
}