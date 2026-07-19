package reporte

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"saferoute/internal/common"
	"saferoute/internal/middleware"
)

type WSNotifier interface {
	BroadcastNotificacion(notificacion NotificacionAlerta)
	NotifyRutasCercanas(reporte ReporteResponse)
}

type Handler struct {
	reporteSvc Service
	notifier   WSNotifier
}

func NewHandler(reporteSvc Service, notifier WSNotifier) *Handler {
	return &Handler{
		reporteSvc: reporteSvc,
		notifier:   notifier,
	}
}

func (h *Handler) CreateReporteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ReporteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		if err := ValidateReporte(&req); err != nil {
			common.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		userID := middleware.GetUserID(r)
		log.Printf("[REPORTE] User=%s Tipo=%s Lat=%.6f Lon=%.6f Ruta=%s",
			userID, req.Tipo, req.Latitud, req.Longitud, req.RutaID)

		reporte, err := h.reporteSvc.Create(req, userID)
		if err != nil {
			log.Printf("[REPORTE] Error creando: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error creando el reporte")
			return
		}

		log.Printf("[REPORTE] Creado ID=%s", reporte.ID)

		if h.notifier != nil {
			go func() {
				h.notifier.BroadcastNotificacion(NotificacionAlerta{
					Tipo:      "nuevo_reporte",
					ReporteID: reporte.ID,
					Latitud:   reporte.Latitud,
					Longitud:  reporte.Longitud,
					NotaVoz:   reporte.NotaVoz,
					RutaID:    reporte.RutaID,
					Timestamp: reporte.Timestamp,
					Mensaje:   fmt.Sprintf("%s reportado en tu zona", formatearTipo(reporte.Tipo)),
				})
			}()
			go h.notifier.NotifyRutasCercanas(reporte)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(reporte)
	}
}

func (h *Handler) GetReportesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tipo := r.URL.Query().Get("tipo")
		vigente := r.URL.Query().Get("vigente")
		limit := 50
		if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 200 {
			limit = l
		}

		offset := 0
		if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
			offset = o
		}

		reportes, err := h.reporteSvc.GetAll(tipo, vigente, limit, offset)
		if err != nil {
			log.Printf("[REPORTES] Error consultando: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error consultando reportes")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"reportes": reportes,
			"total":    len(reportes),
			"filtros": map[string]string{
				"tipo":    tipo,
				"vigente": vigente,
			},
		})
	}
}

func (h *Handler) GetReportesCercanosHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
		if err != nil {
			common.WriteError(w, http.StatusBadRequest, "latitud inválida")
			return
		}
		lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
		if err != nil {
			common.WriteError(w, http.StatusBadRequest, "longitud inválida")
			return
		}
		radioKm := 30.0
		if rv, err := strconv.ParseFloat(r.URL.Query().Get("radio_km"), 64); err == nil && rv > 0 && rv <= 100 {
			radioKm = rv
		}

		reportes, err := h.reporteSvc.GetCercanos(lat, lon, radioKm, 50)
		if err != nil {
			log.Printf("[CERCANOS] Error: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error consultando reportes")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"incidentes": reportes,
			"total":      len(reportes),
			"ubicacion":  map[string]float64{"lat": lat, "lon": lon},
			"radio_km":   radioKm,
		})
	}
}

func (h *Handler) GetReporteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reporteID := mux.Vars(r)["id"]
		rep, err := h.reporteSvc.GetByID(reporteID)
		if err != nil {
			common.WriteError(w, http.StatusNotFound, "reporte no encontrado")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rep)
	}
}

func (h *Handler) ValidarReporteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reporteID := mux.Vars(r)["id"]
		var accion struct {
			Vigente bool `json:"vigente"`
		}
		if err := json.NewDecoder(r.Body).Decode(&accion); err != nil {
			common.WriteError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if err := h.reporteSvc.Validar(reporteID, accion.Vigente); err != nil {
			common.WriteError(w, http.StatusInternalServerError, "error actualizando reporte")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "actualizado",
			"reporte_id": reporteID,
			"vigente":    accion.Vigente,
		})
	}
}

func (h *Handler) GetEstadisticasHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := h.reporteSvc.GetEstadisticas()
		if err != nil {
			log.Printf("[ESTADISTICAS] Error obteniendo: %v", err)
			common.WriteError(w, http.StatusInternalServerError, "error obteniendo estadísticas")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func formatearTipo(tipo string) string {
	nombres := map[string]string{
		"accidente":  "Accidente",
		"inundacion": "Inundación",
		"bache":      "Bache",
		"derrumbe":   "Derrumbe",
		"sin_luz":    "Falta de iluminación",
		"niebla":     "Niebla densa",
		"bloqueo":    "Bloqueo vial",
		"otro":       "Incidente",
	}
	if nombre, ok := nombres[tipo]; ok {
		return nombre
	}
	return tipo
}
