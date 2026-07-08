package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"saferoute/database"
	"saferoute/middleware"
)

type NotificarConductorRequest struct {
	ConductorID   string  `json:"conductor_id"`
	ReporteID     string  `json:"reporte_id"`
	TipoIncidente string  `json:"tipo_incidente"`
	Latitud       float64 `json:"latitud"`
	Longitud      float64 `json:"longitud"`
	Mensaje       string  `json:"mensaje"`
	DistanciaKm   float64 `json:"distancia_km"`
}

func NotificarConductorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		if adminID == "" {
			responderJSON(w, http.StatusUnauthorized, map[string]string{"error": "no autenticado"})
			return
		}

		var req NotificarConductorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			responderJSON(w, http.StatusBadRequest, map[string]string{"error": "datos inválidos"})
			return
		}

		// Validaciones
		if req.ConductorID == "" {
			responderJSON(w, http.StatusBadRequest, map[string]string{"error": "conductor_id es requerido"})
			return
		}
		if req.ReporteID == "" {
			responderJSON(w, http.StatusBadRequest, map[string]string{"error": "reporte_id es requerido"})
			return
		}

		mensaje := req.Mensaje
		if mensaje == "" {
			mensaje = fmt.Sprintf("⚠️ Alerta: %s reportado a %.1f km de tu ubicación. Verifica la ruta.",
				formatearTipoIncidente(req.TipoIncidente), req.DistanciaKm)
		}

		alertaConductor := map[string]interface{}{
			"tipo":            "alerta_incidente_admin",
			"reporte_id":      req.ReporteID,
			"tipo_incidente":  req.TipoIncidente,
			"lat":             req.Latitud,
			"lon":             req.Longitud,
			"distancia_km":    req.DistanciaKm,
			"mensaje":         mensaje,
			"instrucciones":   req.Mensaje,
			"enviado_por":     "admin",
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
		}

		notificado := enviarAlertaConductor(req.ConductorID, alertaConductor)

		if database.DB != nil {
			go guardarNotificacionAdmin(req.ConductorID, req.ReporteID, req.Latitud, req.Longitud, mensaje)
		}

		log.Printf("[ADMIN-NOTIFICACION] Admin %s → Conductor %s | Incidente: %s | Distancia: %.1f km | Entregado: %v",
			adminID, req.ConductorID, req.TipoIncidente, req.DistanciaKm, notificado)

		if notificado {
			responderJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "enviado",
				"mensaje": "Conductor notificado exitosamente en tiempo real",
			})
		} else {
			responderJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "almacenado",
				"mensaje": "Conductor no conectado. La notificación se entregará cuando se conecte.",
			})
		}
	}
}


func enviarAlertaConductor(userID string, data map[string]interface{}) bool {
	subMu.RLock()
	defer subMu.RUnlock()

	msgBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("⚠️ [NotificarConductor] Error serializando: %v", err)
		return false
	}

	rutas, existe := suscriptoresPorUsuario[userID]
	if !existe {
		return false
	}

	for rutaID := range rutas {
		conns, ok := suscriptores[rutaID]
		if !ok {
			continue
		}
		for conn := range conns {
			if err := conn.WriteMessage(1, msgBytes); err == nil {
				log.Printf("✅ [NotificarConductor] Alerta enviada a %s en ruta %s", userID, rutaID)
				return true
			}
		}
	}

	return false
}

func guardarNotificacionAdmin(userID, reporteID string, lat, lon float64, mensaje string) {
	_, err := database.DB.Exec(`
		INSERT INTO notificaciones_historial 
		(user_id, tipo, reporte_id, latitud, longitud, ruta_id, mensaje, fecha_envio)
		VALUES ($1, 'alerta_incidente_admin', $2, $3, $4, 'admin-directo', $5, NOW())`,
		userID, reporteID, lat, lon, mensaje,
	)
	if err != nil {
		log.Printf("⚠️ [NotificarConductor] Error guardando en BD: %v", err)
	}
}

func formatearTipoIncidente(tipo string) string {
	nombres := map[string]string{
		"accidente":  "Accidente vehicular",
		"inundacion": "Inundación",
		"bache":      "Bache peligroso",
		"derrumbe":   "Derrumbe",
		"bloqueo":    "Bloqueo de carretera",
		"sin_luz":    "Falta de iluminación",
		"niebla":     "Niebla densa",
		"otro":       "Incidente",
	}
	if nombre, ok := nombres[tipo]; ok {
		return nombre
	}
	return tipo
}

func responderJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}