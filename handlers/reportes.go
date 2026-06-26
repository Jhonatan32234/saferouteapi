package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"saferoute/database"
	"saferoute/middleware"
	"saferoute/models"
	"saferoute/pipes"
	"saferoute/services"
)

// CreateReporteHandler delega la creación al Service previo paso por el Pipe de validación.
func CreateReporteHandler(reporteSvc *services.ReporteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Decodificar DTO de entrada
		var req models.ReporteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
			return
		}

		// 2. Pipe: Validar y sanitizar el DTO (modifica in-place)
		if err := pipes.ValidateReporte(&req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// 3. Obtener identidad del usuario desde el contexto (inyectada por AuthMiddleware)
		userID := middleware.GetUserID(r)
		log.Printf("📝 [REPORTE] User=%s Tipo=%s Lat=%.6f Lon=%.6f Ruta=%s",
			userID, req.Tipo, req.Latitud, req.Longitud, req.RutaID)

		// 4. Service: Ejecutar lógica de negocio y persistir
		reporte, err := reporteSvc.Create(req, userID)
		if err != nil {
			log.Printf("❌ [REPORTE] Error creando: %v", err)
			writeError(w, http.StatusInternalServerError, "error creando el reporte")
			return
		}

		log.Printf("✅ [REPORTE] Creado ID=%s", reporte.ID)

		// 5. Notificar en tiempo real a suscriptores de la ruta (background)
		go func() {
			BroadcastNotificacion(models.NotificacionAlerta{
				Tipo:      "nuevo_reporte",
				ReporteID: reporte.ID,
				Latitud:   reporte.Latitud,
				Longitud:  reporte.Longitud,
				NotaVoz:   reporte.NotaVoz,
				RutaID:    reporte.RutaID,
				Timestamp: reporte.Timestamp,
				Mensaje:   fmt.Sprintf("⚠️ %s reportado en tu zona", formatearTipo(reporte.Tipo)),
			})
		}()
		go notificarRutasCercanas(reporte)

		// 6. Responder con el DTO de salida
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(reporte)
	}
}

// GetReportesHandler delega la consulta de reportes al Service.
func GetReportesHandler(reporteSvc *services.ReporteService) http.HandlerFunc {
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

		reportes, err := reporteSvc.GetAll(tipo, vigente, limit, offset)
		if err != nil {
			log.Printf("❌ [REPORTES] Error consultando: %v", err)
			writeError(w, http.StatusInternalServerError, "error consultando reportes")
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

// GetReportesCercanosHandler delega la búsqueda geoespacial al Service.
func GetReportesCercanosHandler(reporteSvc *services.ReporteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "latitud inválida")
			return
		}
		lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "longitud inválida")
			return
		}
		radioKm := 30.0
		if rv, err := strconv.ParseFloat(r.URL.Query().Get("radio_km"), 64); err == nil && rv > 0 && rv <= 100 {
			radioKm = rv
		}

		reportes, err := reporteSvc.GetCercanos(lat, lon, radioKm, 50)
		if err != nil {
			log.Printf("❌ [CERCANOS] Error: %v", err)
			writeError(w, http.StatusInternalServerError, "error consultando reportes")
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

// GetReporteHandler obtiene un reporte específico por ID usando el Service.
func GetReporteHandler(reporteSvc *services.ReporteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reporteID := mux.Vars(r)["id"]
		rep, err := reporteSvc.GetByID(reporteID)
		if err != nil {
			writeError(w, http.StatusNotFound, "reporte no encontrado")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rep)
	}
}

// ValidarReporteHandler confirma o desmiente un reporte usando el Service.
func ValidarReporteHandler(reporteSvc *services.ReporteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reporteID := mux.Vars(r)["id"]
		var accion struct {
			Vigente bool `json:"vigente"`
		}
		if err := json.NewDecoder(r.Body).Decode(&accion); err != nil {
			writeError(w, http.StatusBadRequest, "datos inválidos")
			return
		}

		if err := reporteSvc.Validar(reporteID, accion.Vigente); err != nil {
			writeError(w, http.StatusInternalServerError, "error actualizando reporte")
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

// GetEstadisticasHandler obtiene estadísticas de reportes (acceso directo a BD, sin lógica de negocio compleja).
func GetEstadisticasHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var stats struct {
			TotalReportes   int            `json:"total_reportes"`
			ReportesPorTipo map[string]int `json:"reportes_por_tipo"`
			ReportesHoy     int            `json:"reportes_hoy"`
			ReportesSemana  int            `json:"reportes_semana"`
			TasaConfirmacion float64       `json:"tasa_confirmacion"`
		}

		database.DB.QueryRow("SELECT COUNT(*) FROM reportes WHERE vigente = TRUE").Scan(&stats.TotalReportes)

		stats.ReportesPorTipo = make(map[string]int)
		rows, _ := database.DB.Query("SELECT tipo, COUNT(*) FROM reportes WHERE vigente = TRUE GROUP BY tipo")
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var tipo string
				var count int
				rows.Scan(&tipo, &count)
				stats.ReportesPorTipo[tipo] = count
			}
		}

		database.DB.QueryRow("SELECT COUNT(*) FROM reportes WHERE timestamp::date = CURRENT_DATE").Scan(&stats.ReportesHoy)
		database.DB.QueryRow("SELECT COUNT(*) FROM reportes WHERE timestamp >= date_trunc('week', CURRENT_DATE)").Scan(&stats.ReportesSemana)

		var totalConfirmaciones int
		database.DB.QueryRow("SELECT COALESCE(SUM(confirmaciones), 0) FROM reportes").Scan(&totalConfirmaciones)
		if stats.TotalReportes > 0 {
			stats.TasaConfirmacion = float64(totalConfirmaciones) / float64(stats.TotalReportes)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// ============================================================
// FUNCIONES AUXILIARES INTERNAS
// ============================================================

// formatearTipo devuelve un nombre legible para el tipo de incidente.
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

// notificarRutasCercanas notifica en WebSocket a rutas cercanas usando placeholders seguros.
func notificarRutasCercanas(reporte models.ReporteResponse) {
	log.Printf("🔍 [RUTAS CERCANAS] Buscando para reporte %s (Lat=%.6f, Lon=%.6f)",
		reporte.ID, reporte.Latitud, reporte.Longitud)

	if database.DB == nil {
		log.Printf("❌ [RUTAS CERCANAS] Base de datos no conectada")
		return
	}

	// Consulta segura con placeholders parametrizados (sin fmt.Sprintf)
	// Consulta corregida y ordenada de forma secuencial
	rows, err := database.DB.Query(
	    `SELECT DISTINCT ruta_id FROM reportes
	     WHERE vigente = TRUE
	       AND ruta_id != $1
	       AND (6371 * acos(cos(radians($2)) * cos(radians(latitud)) *
	            cos(radians(longitud) - radians($4)) +
	            sin(radians($3)) * sin(radians(latitud)))) <= 15
	     LIMIT 10`,
	    reporte.RutaID, 
	    reporte.Latitud,  // Asignado a $2
	    reporte.Latitud,  // Asignado a $3
	    reporte.Longitud, // Asignado a $4
	)
	if err != nil {
		log.Printf("❌ [RUTAS CERCANAS] Error SQL: %v", err)
		return
	}
	defer rows.Close()

	var rutasCercanas []string
	for rows.Next() {
		var rutaID string
		if err := rows.Scan(&rutaID); err == nil {
			rutasCercanas = append(rutasCercanas, rutaID)
		}
	}

	log.Printf("✅ [RUTAS CERCANAS] Encontradas %d rutas: %v", len(rutasCercanas), rutasCercanas)
	if len(rutasCercanas) == 0 {
		return
	}

	notificacion := models.NotificacionAlerta{
		Tipo:      "alerta_cercana",
		ReporteID: reporte.ID,
		Latitud:   reporte.Latitud,
		Longitud:  reporte.Longitud,
		NotaVoz:   reporte.NotaVoz,
		RutaID:    reporte.RutaID,
		Timestamp: reporte.Timestamp,
		Mensaje:   fmt.Sprintf("⚠️ %s reportado cerca de tu ruta", formatearTipo(reporte.Tipo)),
	}

	msg, err := json.Marshal(notificacion)
	if err != nil {
		log.Printf("❌ [RUTAS CERCANAS] Error marshaling: %v", err)
		return
	}

	for _, rutaID := range rutasCercanas {
		subMu.RLock()
		conns, ok := suscriptores[rutaID]
		if !ok || len(conns) == 0 {
			subMu.RUnlock()
			continue
		}
		count := 0
		for conn := range conns {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err == nil {
				count++
			}
		}
		subMu.RUnlock()
		log.Printf("  ✅ [RUTAS CERCANAS] Enviado a %d suscriptores de ruta %s", count, rutaID)
	}
}

// haversine calcula distancia en km entre dos puntos geográficos.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}