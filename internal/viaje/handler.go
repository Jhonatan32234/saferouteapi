package viaje

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"saferoute/internal/billing"
	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

type Handler struct {
	viajeSvc Service
}

func NewHandler(viajeSvc Service) *Handler {
	return &Handler{viajeSvc: viajeSvc}
}

func (h *Handler) IniciarViajeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req IniciarViajeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := ValidateIniciarViaje(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		viajeID, err := h.viajeSvc.IniciarViaje(userID, req)
		if err != nil {
			log.Printf("❌ [VIAJES] Error iniciando viaje para %s: %v", userID, err)
			common.WriteError(w, http.StatusInternalServerError, err.Error())
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

func (h *Handler) FinalizarViajeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		var req FinalizarViajeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := ValidateFinalizarViaje(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		estadoFinal, err := h.viajeSvc.FinalizarViaje(userID, req)
		if err != nil {
			log.Printf("❌ [VIAJES] Error finalizando viaje %s para %s: %v", req.ViajeID, userID, err)
			common.WriteError(w, http.StatusBadRequest, err.Error())
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
			"tipo":        "viaje_finalizado",
			"user_id":     userID,
			"viaje_id":    req.ViajeID,
			"estado":      estadoFinal,
			"mensaje":     mensaje,
			"tipo_alerta": tipoAlerta,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		}

		// Emitir evento a los administradores
		go BroadcastAdminMonitor(eventoAdmin)
		log.Printf("[VIAJES] Evento de finalización (%s) emitido para viaje %s del conductor %s", estadoFinal, req.ViajeID, userID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  estadoFinal,
			"mensaje": mensaje,
		})
	}
}

func (h *Handler) GetActiveViajeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)
		if userID == "" {
			common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
			return
		}

		viaje, err := h.viajeSvc.GetActiveViaje(userID)
		if err != nil {
			log.Printf("[VIAJES] Error consultando viaje activo de %s: %v", userID, err)
			common.WriteError(w, http.StatusNotFound, "no se encontró un viaje activo")
			return
		}

		if viaje == nil {
			common.WriteError(w, http.StatusNotFound, "no tienes un viaje activo actualmente")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(viaje)
	}
}



// GetActiveViajesAdminHandler ahora filtra por empresa del admin
func (h *Handler) GetActiveViajesAdminHandler(billingSvc *billing.Service) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        adminID := middleware.GetUserID(r)
        if adminID == "" {
            common.WriteError(w, http.StatusUnauthorized, "usuario no autenticado")
            return
        }

        // Obtener empresa del admin
        empresa, err := billingSvc.GetEmpresaByAdminID(adminID)
        if err != nil {
            common.WriteError(w, http.StatusNotFound, "empresa no encontrada")
            return
        }

        // Solo ver viajes de conductores de SU empresa
        viajes, err := h.viajeSvc.GetActiveViajesByEmpresa(empresa.ID)
        if err != nil {
            log.Printf("[ADMIN-VIAJES] Error consultando viajes: %v", err)
            common.WriteError(w, http.StatusInternalServerError, "error consultando viajes activos")
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(viajes)
    }
}