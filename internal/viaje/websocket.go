package viaje

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"saferoute/internal/middleware"
	"saferoute/internal/reporte"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

const (
	MsgTelemetria       = "telemetria"
	MsgAlertaProximidad = "alerta_proximidad"
	MsgConfirmacion     = "confirmacion"
	MsgEstadoConductor  = "estado_conductor"
	MsgSyncPendientes   = "sync_pendientes"
	MsgHistorialInicial = "historial_inicial"
	MsgNuevoReporte     = "nuevo_reporte"
	MsgAlertaDesvio     = "alerta_desvio"
	MsgAlertaTimeout    = "alerta_timeout"
)

type MensajeTelemetria struct {
	Tipo      string  `json:"tipo"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Velocidad float64 `json:"velocidad_kmh,omitempty"`
	RutaID    string  `json:"ruta_id"`
	Timestamp string  `json:"timestamp"`
}

type MensajeConfirmacion struct {
	Tipo       string `json:"tipo"`
	ReporteID  string `json:"reporte_id"`
	Vigente    bool   `json:"vigente"`
	UserID     string `json:"user_id"`
	Timestamp  string `json:"timestamp"`
}

type MensajeEntrante struct {
	Tipo      string  `json:"tipo"`
	Lat       float64 `json:"lat,omitempty"`
	Lon       float64 `json:"lon,omitempty"`
	Velocidad float64 `json:"velocidad_kmh,omitempty"`
	RutaID    string  `json:"ruta_id,omitempty"`
	Timestamp string  `json:"timestamp,omitempty"`
	ReporteID string  `json:"reporte_id,omitempty"`
	Vigente   *bool   `json:"vigente,omitempty"`
	Estado    string  `json:"estado,omitempty"`
	Reportes  []interface{} `json:"reportes_pendientes,omitempty"`
}

type MotorService interface {
	SyncInteraccion(tipo, userID, rutaID string, data map[string]interface{})
	SyncReporteValidado(reporteID string, vigente bool)
	SyncReporteCreado(reporte reporte.ReporteResponse)
}

type WebSocketManager struct {
	db                     *sql.DB
	viajeSvc               Service
	motorSvc               MotorService
	pubKey                 ed25519.PublicKey
	suscriptores           map[string]map[*websocket.Conn]bool
	suscriptoresPorUsuario map[string]map[string]bool
	mu                     sync.RWMutex
}

var (
	activeManager *WebSocketManager
	activeMu      sync.RWMutex
)

func NewWebSocketManager(db *sql.DB, viajeSvc Service, motorSvc MotorService, pubKey ed25519.PublicKey) *WebSocketManager {
	mgr := &WebSocketManager{
		db:                     db,
		viajeSvc:               viajeSvc,
		motorSvc:               motorSvc,
		pubKey:                 pubKey,
		suscriptores:           make(map[string]map[*websocket.Conn]bool),
		suscriptoresPorUsuario: make(map[string]map[string]bool),
	}
	activeMu.Lock()
	activeManager = mgr
	activeMu.Unlock()
	return mgr
}

func BroadcastAdminMonitor(data interface{}) {
	activeMu.RLock()
	defer activeMu.RUnlock()
	if activeManager != nil {
		activeManager.BroadcastAdminMonitor(data)
	}
}

func (m *WebSocketManager) WebSocketHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rutaID := mux.Vars(r)["ruta_id"]

		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && len(authHeader) > 7 {
				token := authHeader[7:]
				if m.pubKey != nil {
					if claims, err := middleware.ValidateToken(token, m.pubKey); err == nil {
						if uid, ok := claims["user_id"].(string); ok {
							userID = uid
						}
					}
				}
			}
		}

		if userID == "" {
			userID = "anonimo"
		}

		log.Printf("[WS] Nueva conexión: User=%s, Ruta=%s", userID, rutaID)

		if userID != "anonimo" && userID != "admin" && m.db != nil {
			_, err := m.db.Exec(
				`INSERT INTO suscripciones_rutas (user_id, ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion)
				 VALUES ($1, $2, true, NOW(), NOW())
				 ON CONFLICT (user_id, ruta_id) 
				 DO UPDATE SET suscrito = true, fecha_actualizacion = NOW()`,
				userID, rutaID,
			)
			if err != nil {
				log.Printf("⚠️ [WS] No se guardó suscripción (user_id no es UUID): %v", err)
			}
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("❌ [WS] Error upgrading: %v", err)
			return
		}
		defer func() {
			conn.Close()
			m.mu.Lock()
			delete(m.suscriptores[rutaID], conn)
			if len(m.suscriptores[rutaID]) == 0 {
				delete(m.suscriptores, rutaID)
			}
			if m.suscriptoresPorUsuario[userID] != nil {
				delete(m.suscriptoresPorUsuario[userID], rutaID)
				if len(m.suscriptoresPorUsuario[userID]) == 0 {
					delete(m.suscriptoresPorUsuario, userID)
				}
			}
			m.mu.Unlock()
			log.Printf("[WS] Usuario %s desconectado de ruta %s", userID, rutaID)
		}()

		m.mu.Lock()
		if m.suscriptores[rutaID] == nil {
			m.suscriptores[rutaID] = make(map[*websocket.Conn]bool)
		}
		m.suscriptores[rutaID][conn] = true

		if m.suscriptoresPorUsuario[userID] == nil {
			m.suscriptoresPorUsuario[userID] = make(map[string]bool)
		}
		m.suscriptoresPorUsuario[userID][rutaID] = true
		m.mu.Unlock()

		log.Printf("[WS] Usuario %s registrado. Total conexiones: %d", userID, len(m.suscriptores[rutaID]))

		if rutaID == "admin-monitor" {
			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				}
			}()
		}

		if userID != "anonimo" {
			go m.enviarHistorialReciente(userID, conn)
		}

		for {
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS] Error lectura (desconexión): %v", err)
				break
			}

			var msg MensajeEntrante
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				log.Printf("[WS] Mensaje inválido de %s: %v", userID, err)
				continue
			}

			switch msg.Tipo {
			case MsgTelemetria:
				log.Printf("[TELEMETRÍA] User=%s Lat=%.6f Lon=%.6f Vel=%.0f km/h Ruta=%s",
					userID, msg.Lat, msg.Lon, msg.Velocidad, msg.RutaID)
					
				if userID != "admin" && userID != "anonimo" && m.db != nil {
					_, err := m.db.Exec(
						`INSERT INTO zonas_usuario (user_id, zona_nombre, latitud, longitud, radio_km, activo, fecha_actualizacion)
						 VALUES ($1, 'ubicacion_actual', $2, $3, 15.0, true, NOW())
						 ON CONFLICT (user_id, zona_nombre)
						 DO UPDATE SET latitud = $2, longitud = $3, fecha_actualizacion = NOW()`,
						userID, msg.Lat, msg.Lon,
					)
					if err != nil {
						log.Printf("[TELEMETRÍA] No se actualizó ubicación (user_id no es UUID): %v", err)
					}
				}

				var nuevoEstado string
				var alertaDesvio bool
				if m.viajeSvc != nil && userID != "admin" && userID != "anonimo" {
					var err error
					nuevoEstado, alertaDesvio, err = m.viajeSvc.ActualizarUbicacionViaje(userID, msg.Lat, msg.Lon, msg.Velocidad)
					if err != nil {
						log.Printf("[TELEMETRÍA] Error actualizando viaje para %s: %v", userID, err)
					}
				}

				if m.db != nil {
					go m.verificarProximidadYNotificar(userID, msg.Lat, msg.Lon, msg.RutaID, conn)
				}

				resp := map[string]interface{} {
					"tipo":      "telemetria_ack",
					"status":    "ok",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}
				if nuevoEstado != "" {
					resp["estado_viaje"] = nuevoEstado
				}
				conn.WriteJSON(resp)

				if m.motorSvc != nil {
					go m.motorSvc.SyncInteraccion("telemetria", userID, msg.RutaID, map[string]interface{}{
						"lat": msg.Lat,
						"lon": msg.Lon,
						"velocidad_kmh": msg.Velocidad,
						"timestamp_cliente": msg.Timestamp,
					})
				}

				m.mu.RLock()
				if adminConns, ok := m.suscriptores["admin-monitor"]; ok {
					telemetriaMsg := map[string]interface{}{
						"tipo":            "telemetria",
						"user_id":         userID,
						"lat":             msg.Lat,
						"lon":             msg.Lon,
						"velocidad_kmh":   msg.Velocidad,
						"ruta_id":         msg.RutaID,
						"timestamp":       msg.Timestamp,
					}
					if nuevoEstado != "" {
						telemetriaMsg["estado_viaje"] = nuevoEstado
					}
					telemetriaBytes, _ := json.Marshal(telemetriaMsg)
					for adminConn := range adminConns {
						if adminConn != conn {
							adminConn.WriteMessage(websocket.TextMessage, telemetriaBytes)
						}
					}

					if alertaDesvio {
						alertaMsg := map[string]interface{}{
							"tipo":      MsgAlertaDesvio,
							"user_id":   userID,
							"ruta_id":   msg.RutaID,
							"lat":       msg.Lat,
							"lon":       msg.Lon,
							"mensaje":   fmt.Sprintf("Conductor %s se ha desviado de su ruta planificada", userID),
							"timestamp": time.Now().UTC().Format(time.RFC3339),
						}
						alertaBytes, _ := json.Marshal(alertaMsg)
						for adminConn := range adminConns {
							adminConn.WriteMessage(websocket.TextMessage, alertaBytes)
						}
					}
				}
				m.mu.RUnlock()

			case MsgConfirmacion:
				log.Printf("[CONFIRMACIÓN] User=%s Reporte=%s Vigente=%v",
					userID, msg.ReporteID, msg.Vigente != nil && *msg.Vigente)

				if msg.ReporteID != "" && m.db != nil {
					vigente := false
					if msg.Vigente != nil {
						vigente = *msg.Vigente
					}

					if vigente {
						m.db.Exec(
							"UPDATE reportes SET confirmaciones = confirmaciones + 1 WHERE id = $1",
							msg.ReporteID,
						)
					} else {
						m.db.Exec(
							"UPDATE reportes SET vigente = FALSE WHERE id = $1",
							msg.ReporteID,
						)
					}

					resp := map[string]interface{}{
						"tipo":       "confirmacion_ack",
						"status":     "ok",
						"reporte_id": msg.ReporteID,
						"vigente":    vigente,
					}
					conn.WriteJSON(resp)

					if m.motorSvc != nil {
						go m.motorSvc.SyncReporteValidado(msg.ReporteID, vigente)
						go m.motorSvc.SyncInteraccion("confirmacion_reporte", userID, msg.RutaID, map[string]interface{}{
							"reporte_id": msg.ReporteID,
							"vigente":    vigente,
						})
					}
				}

			case MsgEstadoConductor:
				if m.motorSvc != nil {
					go m.motorSvc.SyncInteraccion("estado_conductor", userID, msg.RutaID, map[string]interface{}{
						"estado": msg.Estado,
						"lat":    msg.Lat,
						"lon":    msg.Lon,
					})
				}
				log.Printf("[ESTADO] User=%s Estado=%s", userID, msg.Estado)

				if msg.Estado == "emergencia" {
					alerta := reporte.NotificacionAlerta{
						Tipo:      "emergencia_conductor",
						ReporteID: "",
						Latitud:   msg.Lat,
						Longitud:  msg.Lon,
						NotaVoz:   fmt.Sprintf("Conductor %s marcó emergencia", userID),
						RutaID:    msg.RutaID,
						Timestamp: time.Now().UTC(),
						Mensaje:   fmt.Sprintf("🚨 Conductor %s necesita ayuda", userID),
					}
					go m.BroadcastNotificacion(alerta)
				}

			case MsgSyncPendientes:
				log.Printf("[SYNC] User=%s enviando %d reportes pendientes", userID, len(msg.Reportes))
				subidos := 0
				for _, repRaw := range msg.Reportes {
					rep, ok := repRaw.(map[string]interface{})
					if !ok {
						continue
					}
					tipo, _ := rep["tipo"].(string)
					lat, _ := rep["latitud"].(float64)
					lon, _ := rep["longitud"].(float64)
					nota, _ := rep["nota_voz"].(string)
					ruta, _ := rep["ruta_id"].(string)

					if tipo != "" && lat != 0 && lon != 0 && m.db != nil {
						var rResponse reporte.ReporteResponse
						err := m.db.QueryRow(
							`INSERT INTO reportes (user_id, tipo, latitud, longitud, nota_voz, ruta_id)
							 VALUES ($1, $2, $3, $4, $5, $6)
							 RETURNING id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones`,
							userID, tipo, lat, lon, nota, ruta,
						).Scan(
							&rResponse.ID, &rResponse.Tipo, &rResponse.Latitud, &rResponse.Longitud,
							&rResponse.NotaVoz, &rResponse.RutaID, &rResponse.Timestamp,
							&rResponse.Vigente, &rResponse.Confirmaciones,
						)
						if err == nil {
							subidos++
							if m.motorSvc != nil {
								go m.motorSvc.SyncReporteCreado(rResponse)
							}
						}
					}
				}

				resp := map[string]interface{}{
					"tipo":    "sync_ack",
					"subidos": subidos,
					"total":   len(msg.Reportes),
				}
				conn.WriteJSON(resp)

			default:
				log.Printf("[WS] Tipo de mensaje desconocido: %s", msg.Tipo)
			}
		}
	}
}

func (m *WebSocketManager) verificarProximidadYNotificar(userID string, lat, lon float64, rutaID string, conn *websocket.Conn) {
	if m.db == nil {
		return
	}
	rows, err := m.db.Query(
		`SELECT id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, confirmaciones, timestamp
		 FROM reportes 
		 WHERE vigente = TRUE 
		 AND (6371 * acos(cos(radians($1)) * cos(radians(latitud)) * 
		      cos(radians(longitud) - radians($2)) + 
		      sin(radians($1)) * sin(radians(latitud)))) <= 5
		 ORDER BY timestamp DESC LIMIT 5`,
		lat, lon,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, tipo, notaVoz, rutaRep string
		var latRep, lonRep float64
		var confirmaciones int
		var timestamp time.Time

		rows.Scan(&id, &tipo, &latRep, &lonRep, &notaVoz, &rutaRep, &confirmaciones, &timestamp)

		alerta := map[string]interface{}{
			"tipo":           MsgAlertaProximidad,
			"reporte_id":     id,
			"tipo_incidente": tipo,
			"lat":            latRep,
			"lon":            lonRep,
			"nota_voz":       notaVoz,
			"confirmaciones": confirmaciones,
			"timestamp":      timestamp,
			"mensaje":        fmt.Sprintf(" %s reportado cerca de tu ubicación", formatearTipoWS(tipo)),
		}
		conn.WriteJSON(alerta)
		log.Print("Alerta de proximidad enviado:", alerta)
	}
}

func formatearTipoWS(tipo string) string {
	nombres := map[string]string{
		"accidente": "Accidente", "inundacion": "Inundación",
		"bache": "Bache", "derrumbe": "Derrumbe",
		"bloqueo": "Bloqueo", "sin_luz": "Falta de luz",
		"niebla": "Niebla",
	}
	if n, ok := nombres[tipo]; ok {
		return n
	}
	return tipo
}

func (m *WebSocketManager) BroadcastNotificacion(notificacion reporte.NotificacionAlerta) {
	log.Printf("[BROADCAST] Broadcast para reporte en (%.6f, %.6f)", notificacion.Latitud, notificacion.Longitud)

	if m.db == nil {
		return
	}

	rows, err := m.db.Query(
		`SELECT DISTINCT zu.user_id 
		 FROM zonas_usuario zu
		 WHERE zu.activo = true
		 AND (6371 * acos(cos(radians($1)) * cos(radians(zu.latitud)) * 
		      cos(radians(zu.longitud) - radians($2)) + 
		      sin(radians($1)) * sin(radians(zu.latitud)))) <= zu.radio_km`,
		notificacion.Latitud, notificacion.Longitud,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			log.Printf("[BROADCAST] Error escaneando user_id: %v", err)
			continue
		}
		userIDs = append(userIDs, uid)
	}

	for _, uid := range userIDs {
		m.guardarNotificacion(uid, notificacion)
	}

	m.mu.RLock()
	for uid, rutas := range m.suscriptoresPorUsuario {
		esDestinatario := false
		for _, id := range userIDs {
			if id == uid {
				esDestinatario = true
				break
			}
		}
		if !esDestinatario {
			continue
		}

		msg, _ := json.Marshal(notificacion)
		for rutaID := range rutas {
			if conns, ok := m.suscriptores[rutaID]; ok {
				for conn := range conns {
					conn.WriteMessage(websocket.TextMessage, msg)
				}
			}
		}
	}
	m.mu.RUnlock()
}

func (m *WebSocketManager) guardarNotificacion(userID string, notificacion reporte.NotificacionAlerta) error {
	if m.db == nil {
		return fmt.Errorf("base de datos no conectada")
	}

	_, err := m.db.Exec(
		`INSERT INTO notificaciones_historial 
		 (user_id, tipo, reporte_id, latitud, longitud, nota_voz, ruta_id, mensaje, fecha_envio)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`,
		userID, notificacion.Tipo, notificacion.ReporteID,
		notificacion.Latitud, notificacion.Longitud,
		notificacion.NotaVoz, notificacion.RutaID,
		notificacion.Mensaje,
	)
	return err
}

func (m *WebSocketManager) enviarHistorialReciente(userID string, conn *websocket.Conn) {
	if m.db == nil {
		return
	}

	rows, err := m.db.Query(
		`SELECT id, tipo, COALESCE(reporte_id::text,''), latitud, longitud, 
		        COALESCE(nota_voz,''), ruta_id, mensaje, fecha_envio
		 FROM notificaciones_historial 
		 WHERE user_id = $1 AND leida = false
		 ORDER BY fecha_envio DESC LIMIT 10`,
		userID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var notificaciones []map[string]interface{}
	for rows.Next() {
		var id, tipo, reporteID, notaVoz, rutaID, mensaje string
		var lat, lon float64
		var fecha time.Time

		rows.Scan(&id, &tipo, &reporteID, &lat, &lon, &notaVoz, &rutaID, &mensaje, &fecha)
		notificaciones = append(notificaciones, map[string]interface{}{
			"id": id, "tipo_alerta": tipo, "reporte_id": reporteID,
			"latitud": lat, "longitud": lon, "nota_voz": notaVoz,
			"ruta_id": rutaID, "mensaje": mensaje, "fecha_envio": fecha, "leida": false,
		})
	}

	if len(notificaciones) > 0 {
		conn.WriteJSON(map[string]interface{}{
			"tipo":            MsgHistorialInicial,
			"notificaciones":  notificaciones,
			"total_no_leidas": len(notificaciones),
		})
	}
}

func (m *WebSocketManager) GetEstadoSuscriptores() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	estado := make(map[string]interface{})
	estado["total_rutas"] = len(m.suscriptores)
	estado["total_usuarios"] = len(m.suscriptoresPorUsuario)

	rutas := make(map[string]int)
	for rutaID, conns := range m.suscriptores {
		rutas[rutaID] = len(conns)
	}
	estado["rutas"] = rutas
	return estado
}

func (m *WebSocketManager) BroadcastAdminMonitor(data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adminConns, ok := m.suscriptores["admin-monitor"]
	if !ok {
		return
	}

	msg, err := json.Marshal(data)
	if err != nil {
		log.Printf("⚠️ [BroadcastAdmin] Error serializando mensaje: %v", err)
		return
	}

	for conn := range adminConns {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("⚠️ [BroadcastAdmin] Error enviando a admin: %v", err)
			conn.Close()
			delete(adminConns, conn)
		}
	}
}

// NotifyRutasCercanas implements reporte.WSNotifier
func (m *WebSocketManager) NotifyRutasCercanas(reporte reporte.ReporteResponse) {
	log.Printf("[RUTAS CERCANAS] Buscando para reporte %s (Lat=%.6f, Lon=%.6f)",
		reporte.ID, reporte.Latitud, reporte.Longitud)

	if m.db == nil {
		return
	}

	rows, err := m.db.Query(
		`SELECT DISTINCT ruta_id FROM reportes
		 WHERE vigente = TRUE
		   AND ruta_id != $1
		   AND (6371 * acos(cos(radians($2)) * cos(radians(latitud)) *
		        cos(radians(longitud) - radians($3)) +
		        sin(radians($2)) * sin(radians(latitud)))) <= 15
		 LIMIT 10`,
		reporte.RutaID, 
		reporte.Latitud,  
		reporte.Longitud,
	)
	if err != nil {
		log.Printf("[RUTAS CERCANAS] Error SQL: %v", err)
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

	log.Printf("[RUTAS CERCANAS] Encontradas %d rutas: %v", len(rutasCercanas), rutasCercanas)
	if len(rutasCercanas) == 0 {
		return
	}

	notificacion := map[string]interface{}{
		"tipo":       "alerta_cercana",
		"reporte_id": reporte.ID,
		"latitud":    reporte.Latitud,
		"longitud":   reporte.Longitud,
		"nota_voz":   reporte.NotaVoz,
		"ruta_id":    reporte.RutaID,
		"timestamp":  reporte.Timestamp,
		"mensaje":    fmt.Sprintf("%s reportado cerca de tu ruta", formatearTipoWS(reporte.Tipo)),
	}

	msg, err := json.Marshal(notificacion)
	if err != nil {
		return
	}

	m.mu.RLock()
	for _, rutaID := range rutasCercanas {
		conns, ok := m.suscriptores[rutaID]
		if !ok || len(conns) == 0 {
			continue
		}
		count := 0
		for conn := range conns {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err == nil {
				count++
			}
		}
		log.Printf("  [RUTAS CERCANAS] Enviado a %d suscriptores de ruta %s", count, rutaID)
	}
	m.mu.RUnlock()
}

// NotificarConductorRequest is the payload for saving a recent destination
type NotificarConductorRequest struct {
	ConductorID   string  `json:"conductor_id"`
	ReporteID     string  `json:"reporte_id"`
	TipoIncidente string  `json:"tipo_incidente"`
	Latitud       float64 `json:"latitud"`
	Longitud      float64 `json:"longitud"`
	Mensaje       string  `json:"mensaje"`
	DistanciaKm   float64 `json:"distancia_km"`
}

func (m *WebSocketManager) NotificarConductorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminID := middleware.GetUserID(r)
		if adminID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "no autenticado"})
			return
		}

		var req NotificarConductorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "datos inválidos"})
			return
		}

		if req.ConductorID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "conductor_id es requerido"})
			return
		}
		if req.ReporteID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "reporte_id es requerido"})
			return
		}

		mensaje := req.Mensaje
		if mensaje == "" {
			mensaje = fmt.Sprintf("Alerta: %s reportado a %.1f km de tu ubicación. Verifica la ruta.",
				formatearTipoWS(req.TipoIncidente), req.DistanciaKm)
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

		notificado := m.enviarAlertaConductor(req.ConductorID, alertaConductor)

		if m.db != nil {
			go m.guardarNotificacionAdmin(req.ConductorID, req.ReporteID, req.Latitud, req.Longitud, mensaje)
		}

		log.Printf("[ADMIN-NOTIFICACION] Admin %s → Conductor %s | Incidente: %s | Distancia: %.1f km | Entregado: %v",
			adminID, req.ConductorID, req.TipoIncidente, req.DistanciaKm, notificado)

		w.Header().Set("Content-Type", "application/json")
		if notificado {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "enviado",
				"mensaje": "Conductor notificado exitosamente en tiempo real",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "almacenado",
				"mensaje": "Conductor no conectado. La notificación se entregará cuando se conecte.",
			})
		}
	}
}

func (m *WebSocketManager) enviarAlertaConductor(userID string, data map[string]interface{}) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msgBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("[NotificarConductor] Error serializando: %v", err)
		return false
	}

	rutas, existe := m.suscriptoresPorUsuario[userID]
	if !existe {
		return false
	}

	for rutaID := range rutas {
		conns, ok := m.suscriptores[rutaID]
		if !ok {
			continue
		}
		for conn := range conns {
			if err := conn.WriteMessage(1, msgBytes); err == nil {
				log.Printf("[NotificarConductor] Alerta enviada a %s en ruta %s", userID, rutaID)
				return true
			}
		}
	}

	return false
}

var uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

func esUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

func (m *WebSocketManager) guardarNotificacionAdmin(userID, reporteID string, lat, lon float64, mensaje string) {
	var reporteIDParam interface{}
	if esUUID(reporteID) {
		reporteIDParam = reporteID
	} else {
		log.Printf("[NotificarConductor] reporte_id '%s' no es UUID válido, se guardará como NULL", reporteID)
		reporteIDParam = nil
	}

	_, err := m.db.Exec(`
		INSERT INTO notificaciones_historial 
		(user_id, tipo, reporte_id, latitud, longitud, ruta_id, mensaje, fecha_envio)
		VALUES ($1, 'alerta_incidente_admin', $2, $3, $4, 'admin-directo', $5, NOW())`,
		userID, reporteIDParam, lat, lon, mensaje,
	)
	if err != nil {
		log.Printf("[NotificarConductor] Error guardando en BD: %v", err)
	}
}
