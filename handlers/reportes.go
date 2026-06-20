package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"saferoute/database"
	"saferoute/middleware"
	"saferoute/models"
)

// CreateReporteHandler - Versión corregida

func CreateReporteHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        log.Printf("🔵 [REPORTE] Iniciando creación de reporte")

        if err := database.EnsureConnection(); err != nil {
            log.Printf("❌ [REPORTE] Error de conexión a BD: %v", err)
            writeError(w, http.StatusInternalServerError, "error de conexión a base de datos")
            return
        }
        
        var req models.ReporteRequest
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            log.Printf("❌ [REPORTE] Error decodificando JSON: %v", err)
            writeError(w, http.StatusBadRequest, "datos de entrada inválidos")
            return
        }

        log.Printf("📝 [REPORTE] Datos recibidos: Tipo=%s, Lat=%.6f, Lon=%.6f, RutaID=%s", 
            req.Tipo, req.Latitud, req.Longitud, req.RutaID)

        // Validar tipo de incidente
        if !models.TiposValidos[req.Tipo] {
            log.Printf("❌ [REPORTE] Tipo inválido: %s", req.Tipo)
            writeError(w, http.StatusBadRequest, 
                fmt.Sprintf("tipo inválido. Usar: %s", tiposPermitidos()))
            return
        }

        // Validar campos requeridos
        if req.Latitud == 0 || req.Longitud == 0 {
            log.Printf("❌ [REPORTE] Coordenadas inválidas: Lat=%.6f, Lon=%.6f", req.Latitud, req.Longitud)
            writeError(w, http.StatusBadRequest, "latitud y longitud son requeridas")
            return
        }

        // Si no hay ruta_id, usar un valor por defecto
        if req.RutaID == "" {
            req.RutaID = "sin-ruta"
            log.Printf("⚠️ [REPORTE] RutaID vacío, usando 'sin-ruta'")
        }

        userID := middleware.GetUserID(r)
        log.Printf("👤 [REPORTE] UserID: %s", userID)

        if userID == "" {
            ctxUserID := r.Context().Value(middleware.UserIDKey)
            if ctxUserID != nil {
                userID = ctxUserID.(string)
                log.Printf("👤 [REPORTE] UserID obtenido del contexto: '%s'", userID)
            }
        }

        // CreateReporteHandler - Parte de la suscripción automática corregida

        if userID != "" && req.RutaID != "" && userID != "anonimo" {
            go func() {
                // USAR CONSULTA DIRECTA SIN PLACEHOLDERS
                query := fmt.Sprintf(
                    `INSERT INTO suscripciones_rutas (user_id, ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion)
                     VALUES ('%s', '%s', true, NOW(), NOW())
                     ON CONFLICT (user_id, ruta_id) 
                     DO UPDATE SET suscrito = true, fecha_actualizacion = NOW()`,
                    userID, req.RutaID,
                )
                
                _, err := database.DB.Exec(query)
                if err != nil {
                    log.Printf("⚠️ [REPORTE] No se pudo suscribir automáticamente a %s: %v", req.RutaID, err)
                } else {
                    log.Printf("✅ [REPORTE] Usuario %s suscrito automáticamente a %s", userID, req.RutaID)
                }
            }()
        }
        
        // Procesar nota de voz
        if req.NotaVoz == "" {
            req.NotaVoz = generarDescripcionAutomatica(req.Tipo)
            log.Printf("📢 [REPORTE] Nota de voz generada automáticamente: %s", req.NotaVoz)
        } else {
            req.NotaVoz = limpiarTexto(req.NotaVoz)
            log.Printf("📢 [REPORTE] Nota de voz procesada: %s", req.NotaVoz)
        }

        // ==========================================
        // INSERTAR REPORTE - CON CONSULTA DIRECTA
        // ==========================================
        log.Printf("💾 [REPORTE] Insertando en base de datos...")
        
        var reporte models.ReporteResponse
        
        // CONSULTA CORREGIDA - Usar consulta directa
        query := fmt.Sprintf(
            `INSERT INTO reportes (user_id, tipo, latitud, longitud, nota_voz, ruta_id) 
             VALUES ('%s', '%s', %f, %f, '%s', '%s') 
             RETURNING id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones`,
            userID, req.Tipo, req.Latitud, req.Longitud, req.NotaVoz, req.RutaID,
        )
        
        log.Printf("🔍 [REPORTE] Query: %s", query)
        
        err := database.DB.QueryRow(query).Scan(
            &reporte.ID, &reporte.Tipo, &reporte.Latitud, &reporte.Longitud,
            &reporte.NotaVoz, &reporte.RutaID, &reporte.Timestamp,
            &reporte.Vigente, &reporte.Confirmaciones,
        )

        if err != nil {
            log.Printf("❌ [REPORTE] Error insertando en BD: %v", err)
            
            // Si el error es de conexión, intentar reconectar y reintentar
            if err.Error() == "sql: database is closed" || err.Error() == "driver: bad connection" {
                log.Printf("🔄 [REPORTE] Error de conexión, reintentando...")
                if err := database.EnsureConnection(); err == nil {
                    // Reintentar una vez más
                    db2 := database.GetDB()
                    if db2 != nil {
                        err2 := db2.QueryRow(
                            `INSERT INTO reportes (user_id, tipo, latitud, longitud, nota_voz, ruta_id) 
                             VALUES ($1, $2, $3, $4, $5, $6) 
                             RETURNING id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones`,
                            userID, req.Tipo, req.Latitud, req.Longitud, req.NotaVoz, req.RutaID,
                        ).Scan(
                            &reporte.ID, &reporte.Tipo, &reporte.Latitud, &reporte.Longitud,
                            &reporte.NotaVoz, &reporte.RutaID, &reporte.Timestamp,
                            &reporte.Vigente, &reporte.Confirmaciones,
                        )
                        if err2 == nil {
                            log.Printf("✅ [REPORTE] Reporte creado en reintento")
                            goto reporteCreado
                        }
                        log.Printf("❌ [REPORTE] Reintento falló: %v", err2)
                    }
                }
            }
            
            writeError(w, http.StatusInternalServerError, "error creando reporte")
            return
        }

        reporteCreado:
        log.Printf("✅ [REPORTE] Reporte creado exitosamente: ID=%s, RutaID=%s", reporte.ID, reporte.RutaID)

        log.Printf("✅ [REPORTE] Reporte creado exitosamente: ID=%s, RutaID=%s", reporte.ID, reporte.RutaID)

        // ============================================
        // NOTIFICACIONES
        // ============================================
        
        log.Printf("📨 [NOTIFICACIÓN] Iniciando notificación para ruta: %s", reporte.RutaID)
        
        go func() {
            log.Printf("🔔 [NOTIFICACIÓN] Goroutine iniciada para reporte %s", reporte.ID)
            
            notificacion := models.NotificacionAlerta{
                Tipo:      "nuevo_reporte",
                ReporteID: reporte.ID,
                Latitud:   reporte.Latitud,
                Longitud:  reporte.Longitud,
                NotaVoz:   reporte.NotaVoz,
                RutaID:    reporte.RutaID,
                Timestamp: reporte.Timestamp,
                Mensaje:   fmt.Sprintf("⚠️ %s reportado en tu ruta", formatearTipo(reporte.Tipo)),
            }
            
            log.Printf("📤 [NOTIFICACIÓN] Notificación creada: %+v", notificacion)
            BroadcastNotificacion(notificacion)
            log.Printf("✅ [NOTIFICACIÓN] BroadcastNotificacion completado para ruta %s", reporte.RutaID)
        }()

        // 2. Notificar a rutas cercanas
        log.Printf("🗺️ [RUTAS CERCANAS] Iniciando búsqueda de rutas cercanas para reporte %s", reporte.ID)
        go notificarRutasCercanas(reporte)

        // Responder al cliente
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(reporte)
        
        log.Printf("✅ [REPORTE] Respuesta enviada al cliente para ID=%s", reporte.ID)
    }
}



// GetReportesHandler obtiene reportes con filtros opcionales
func GetReportesHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Parámetros de consulta
        tipo := r.URL.Query().Get("tipo")
        vigente := r.URL.Query().Get("vigente")
        limitStr := r.URL.Query().Get("limit")
        limit := 50
        if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
            limit = l
        }

        // Construir consulta dinámica
        query := `SELECT id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, 
                         timestamp, vigente, confirmaciones 
                  FROM reportes WHERE 1=1`
        args := []interface{}{}
        argCount := 0

        if tipo != "" && models.TiposValidos[tipo] {
            argCount++
            query += fmt.Sprintf(" AND tipo = $%d", argCount)
            args = append(args, tipo)
        }

        if vigente == "true" {
            argCount++
            query += fmt.Sprintf(" AND vigente = $%d", argCount)
            args = append(args, true)
        } else if vigente == "false" {
            argCount++
            query += fmt.Sprintf(" AND vigente = $%d", argCount)
            args = append(args, false)
        }

        argCount++
        query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d", argCount)
        args = append(args, limit)

        rows, err := database.DB.Query(query, args...)
        if err != nil {
            log.Printf("ERROR consultando reportes: %v", err)
            writeError(w, http.StatusInternalServerError, "error consultando reportes")
            return
        }
        defer rows.Close()

        var reportes []models.ReporteResponse
        for rows.Next() {
            var rep models.ReporteResponse
            if err := rows.Scan(&rep.ID, &rep.Tipo, &rep.Latitud, &rep.Longitud,
                &rep.NotaVoz, &rep.RutaID, &rep.Timestamp,
                &rep.Vigente, &rep.Confirmaciones); err != nil {
                continue
            }
            reportes = append(reportes, rep)
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

// GetReportesCercanosHandler obtiene reportes cercanos a una ubicación
func GetReportesCercanosHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        latStr := r.URL.Query().Get("lat")
        lonStr := r.URL.Query().Get("lon")
        radioStr := r.URL.Query().Get("radio_km")

        lat, err := strconv.ParseFloat(latStr, 64)
        if err != nil {
            writeError(w, http.StatusBadRequest, "latitud inválida")
            return
        }
        lon, err := strconv.ParseFloat(lonStr, 64)
        if err != nil {
            writeError(w, http.StatusBadRequest, "longitud inválida")
            return
        }
        radioKm := 30.0
        if radioStr != "" {
            if r, err := strconv.ParseFloat(radioStr, 64); err == nil && r > 0 && r <= 100 {
                radioKm = r
            }
        }

        // Consulta con fórmula de Haversine en PostgreSQL
        rows, err := database.DB.Query(
            `SELECT id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, 
                    timestamp, vigente, confirmaciones,
                    (6371 * acos(cos(radians($1)) * cos(radians(latitud)) * 
                     cos(radians(longitud) - radians($2)) + 
                     sin(radians($1)) * sin(radians(latitud)))) AS distancia_km
             FROM reportes 
             WHERE vigente = TRUE 
             AND (6371 * acos(cos(radians($1)) * cos(radians(latitud)) * 
                  cos(radians(longitud) - radians($2)) + 
                  sin(radians($1)) * sin(radians(latitud)))) <= $3
             ORDER BY distancia_km ASC
             LIMIT 50`,
            lat, lon, radioKm,
        )
        if err != nil {
            log.Printf("ERROR consultando reportes cercanos: %v", err)
            writeError(w, http.StatusInternalServerError, "error consultando reportes")
            return
        }
        defer rows.Close()

        incidentes := make([]models.IncidenteCercano, 0)
        for rows.Next() {
            var inc models.IncidenteCercano
            print(inc.NotaVoz)
            if err := rows.Scan(&inc.ID, &inc.Tipo, &inc.Latitud, &inc.Longitud,
                &inc.NotaVoz, &inc.RutaID, &inc.Timestamp,
                &inc.Confirmaciones, &inc.DistanciaKm,
            ); err != nil {
                continue
            }
            incidentes = append(incidentes, inc)
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "incidentes":      incidentes,
            "total":           len(incidentes),
            "ubicacion":       map[string]float64{"lat": lat, "lon": lon},
            "radio_km":        radioKm,
        })
    }
}

// GetReporteHandler obtiene un reporte específico por ID
func GetReporteHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        reporteID := mux.Vars(r)["id"]

        var rep models.ReporteResponse
        err := database.DB.QueryRow(
            `SELECT id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, 
                    timestamp, vigente, confirmaciones 
             FROM reportes WHERE id = $1`,
            reporteID,
        ).Scan(&rep.ID, &rep.Tipo, &rep.Latitud, &rep.Longitud,
            &rep.NotaVoz, &rep.RutaID, &rep.Timestamp,
            &rep.Vigente, &rep.Confirmaciones)

        if err != nil {
            writeError(w, http.StatusNotFound, "reporte no encontrado")
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(rep)
    }
}

// ValidarReporteHandler confirma o desmiente un reporte
func ValidarReporteHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        reporteID := mux.Vars(r)["id"]
        
        var accion struct {
            Vigente bool `json:"vigente"`
        }
        if err := json.NewDecoder(r.Body).Decode(&accion); err != nil {
            writeError(w, http.StatusBadRequest, "datos inválidos")
            return
        }

        if accion.Vigente {
            _, err := database.DB.Exec(
                "UPDATE reportes SET confirmaciones = confirmaciones + 1 WHERE id = $1",
                reporteID,
            )
            if err != nil {
                writeError(w, http.StatusInternalServerError, "error actualizando")
                return
            }
        } else {
            _, err := database.DB.Exec(
                "UPDATE reportes SET vigente = FALSE WHERE id = $1",
                reporteID,
            )
            if err != nil {
                writeError(w, http.StatusInternalServerError, "error actualizando")
                return
            }
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status":      "actualizado",
            "reporte_id":  reporteID,
            "vigente":     fmt.Sprintf("%v", accion.Vigente),
        })
    }
}

// GetEstadisticasHandler obtiene estadísticas de reportes
func GetEstadisticasHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var stats struct {
            TotalReportes    int            `json:"total_reportes"`
            ReportesPorTipo  map[string]int `json:"reportes_por_tipo"`
            ReportesHoy      int            `json:"reportes_hoy"`
            ReportesSemana   int            `json:"reportes_semana"`
            TasaConfirmacion float64        `json:"tasa_confirmacion"`
        }

        // Total
        database.DB.QueryRow("SELECT COUNT(*) FROM reportes WHERE vigente = TRUE").Scan(&stats.TotalReportes)
        
        // Por tipo
        stats.ReportesPorTipo = make(map[string]int)
        rows, _ := database.DB.Query(
            "SELECT tipo, COUNT(*) FROM reportes WHERE vigente = TRUE GROUP BY tipo")
        if rows != nil {
            defer rows.Close()
            for rows.Next() {
                var tipo string
                var count int
                rows.Scan(&tipo, &count)
                stats.ReportesPorTipo[tipo] = count
            }
        }

        // Hoy
        database.DB.QueryRow(
            "SELECT COUNT(*) FROM reportes WHERE timestamp::date = CURRENT_DATE").Scan(&stats.ReportesHoy)
        
        // Esta semana
        database.DB.QueryRow(
            "SELECT COUNT(*) FROM reportes WHERE timestamp >= date_trunc('week', CURRENT_DATE)").Scan(&stats.ReportesSemana)

        // Tasa de confirmación
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
// FUNCIONES AUXILIARES
// ============================================================

// generarDescripcionAutomatica genera texto descriptivo según el tipo
func generarDescripcionAutomatica(tipo string) string {
    descripciones := map[string]string{
        "accidente":  "Accidente reportado en la vía",
        "inundacion": "Zona inundada detectada",
        "bache":      "Bache peligroso en el camino",
        "derrumbe":   "Derrumbe bloqueando la vía",
        "sin_luz":    "Zona sin iluminación",
        "niebla":     "Niebla densa reduciendo visibilidad",
        "bloqueo":    "Vía bloqueada",
        "otro":       "Incidente reportado",
    }
    if desc, ok := descripciones[tipo]; ok {
        return desc
    }
    return "Incidente vial reportado"
}

// limpiarTexto limpia y trunca texto
func limpiarTexto(texto string) string {
    texto = strings.TrimSpace(texto)
    if len(texto) > 300 {
        texto = texto[:297] + "..."
    }
    return texto
}

// formatearTipo devuelve un nombre legible para el tipo
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

// tiposPermitidos devuelve lista de tipos válidos
func tiposPermitidos() string {
    var tipos []string
    for t := range models.TiposValidos {
        tipos = append(tipos, t)
    }
    return strings.Join(tipos, ", ")
}

// rutaCercanaALaGeometria verifica proximidad simple (placeholder)
func rutaCercanaALaGeometria(rutaID string, lat, lon float64) bool {
    // Simplificación: asumimos que rutas con nombres similares están cerca
    return strings.Contains(strings.ToLower(rutaID), strings.ToLower(extraerZona(lat, lon)))
}

// extraerZona estima una zona geográfica simple
func extraerZona(lat, lon float64) string {
    zonas := []struct {
        nombre string
        latMin float64
        latMax float64
        lonMin float64
        lonMax float64
    }{
        {"tuxtla", 16.70, 16.80, -93.20, -93.05},
        {"suchiapa", 16.70, 16.75, -93.05, -93.00},
        {"berriozabal", 16.78, 16.85, -93.30, -93.22},
        {"chiapa", 16.70, 16.75, -93.05, -92.95},
        {"comitan", 16.20, 16.30, -92.20, -92.05},
        {"teopisca", 16.50, 16.60, -92.50, -92.40},
        {"san-cristobal", 16.70, 16.80, -92.65, -92.55},
        {"tapachula", 14.85, 14.95, -92.30, -92.20},
        {"palenque", 17.45, 17.55, -92.00, -91.90},
    }
    for _, z := range zonas {
        if lat >= z.latMin && lat <= z.latMax && lon >= z.lonMin && lon <= z.lonMax {
            return z.nombre
        }
    }
    return ""
}

// haversine calcula distancia en km entre dos puntos
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

// notificarRutasCercanasConHistorial notifica a rutas cercanas guardando en historial
// notificarRutasCercanasConHistorial notifica a rutas cercanas guardando en historial
func notificarRutasCercanasConHistorial(reporte models.ReporteResponse) {
    // Buscar rutas suscritas que estén cerca del reporte (radio ~15 km)
    rows, err := database.DB.Query(
        `SELECT DISTINCT ruta_id FROM reportes 
         WHERE vigente = TRUE 
         AND ruta_id != $1
         AND (6371 * acos(cos(radians($2)) * cos(radians(latitud)) * 
              cos(radians(longitud) - radians($3)) + 
              sin(radians($2)) * sin(radians(latitud)))) <= 15`,
        reporte.RutaID, reporte.Latitud, reporte.Longitud, // 3 parámetros
    )
    if err != nil {
        log.Printf("ERROR buscando rutas cercanas: %v", err)
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

    // Notificar a cada ruta cercana
    for _, rutaID := range rutasCercanas {
        notificacion := models.NotificacionAlerta{
            Tipo:      "alerta_cercana",
            ReporteID: reporte.ID,
            Latitud:   reporte.Latitud,
            Longitud:  reporte.Longitud,
            NotaVoz:   reporte.NotaVoz,
            RutaID:    rutaID,
            Timestamp: reporte.Timestamp,
            Mensaje:   fmt.Sprintf("⚠️ %s reportado cerca de tu ruta", formatearTipo(reporte.Tipo)),
        }
        
        // Broadcast a esta ruta específica
        go BroadcastNotificacion(notificacion)
    }
}

// handlers/reportes.go - notificarRutasCercanas DEFINITIVO

func notificarRutasCercanas(reporte models.ReporteResponse) {
    log.Printf("🔍 [RUTAS CERCANAS] Iniciando búsqueda para reporte %s (Lat=%.6f, Lon=%.6f)", 
        reporte.ID, reporte.Latitud, reporte.Longitud)
    
    if database.DB == nil {
        log.Printf("❌ [RUTAS CERCANAS] Base de datos no conectada")
        return
    }
    
    // CONSULTA CON PARÁMETROS INCORPORADOS DIRECTAMENTE
    query := fmt.Sprintf(`SELECT DISTINCT ruta_id FROM reportes 
              WHERE vigente = TRUE 
              AND ruta_id != '%s'
              AND (6371 * acos(cos(radians(%f)) * cos(radians(latitud)) * 
                   cos(radians(longitud) - radians(%f)) + 
                   sin(radians(%f)) * sin(radians(latitud)))) <= 15
              LIMIT 10`,
        reporte.RutaID, reporte.Latitud, reporte.Longitud, reporte.Latitud)
    
    log.Printf("📝 [RUTAS CERCANAS] Ejecutando consulta SQL: %s", query)
    
    rows, err := database.DB.Query(query)
    if err != nil {
        log.Printf("❌ [RUTAS CERCANAS] Error en consulta SQL: %v", err)
        return
    }
    defer rows.Close()

    var rutasCercanas []string
    for rows.Next() {
        var rutaID string
        if err := rows.Scan(&rutaID); err != nil {
            log.Printf("⚠️ [RUTAS CERCANAS] Error escaneando ruta: %v", err)
            continue
        }
        rutasCercanas = append(rutasCercanas, rutaID)
    }

    log.Printf("✅ [RUTAS CERCANAS] Encontradas %d rutas cercanas: %v", len(rutasCercanas), rutasCercanas)

    if len(rutasCercanas) == 0 {
        log.Printf("ℹ️ [RUTAS CERCANAS] No se encontraron rutas cercanas")
        return
    }

    // Notificar a cada ruta cercana
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

    log.Printf("📨 [RUTAS CERCANAS] Notificando a %d rutas cercanas", len(rutasCercanas))
    
    for i, rutaID := range rutasCercanas {
        log.Printf("  📍 [RUTAS CERCANAS] [%d/%d] Notificando a ruta: %s", i+1, len(rutasCercanas), rutaID)
        
        // Verificar suscriptores en memoria
        subMu.RLock()
        conns, ok := suscriptores[rutaID]
        if !ok || len(conns) == 0 {
            log.Printf("  ⚠️ [RUTAS CERCANAS] Ruta %s no tiene suscriptores activos", rutaID)
            subMu.RUnlock()
            continue
        }
        log.Printf("  👥 [RUTAS CERCANAS] Ruta %s tiene %d suscriptores activos", rutaID, len(conns))
        
        msg, err := json.Marshal(notificacion)
        if err != nil {
            log.Printf("  ❌ [RUTAS CERCANAS] Error marshaling notificación: %v", err)
            subMu.RUnlock()
            continue
        }
        
        // Enviar a cada suscriptor
        count := 0
        for conn := range conns {
            if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
                log.Printf("  ❌ [RUTAS CERCANAS] Error enviando a un suscriptor: %v", err)
            } else {
                count++
            }
        }
        subMu.RUnlock()
        
        log.Printf("  ✅ [RUTAS CERCANAS] Notificación enviada a %d/%d suscriptores de ruta %s", 
            count, len(conns), rutaID)
    }
    
    log.Printf("✅ [RUTAS CERCANAS] Proceso completado para reporte %s", reporte.ID)
}