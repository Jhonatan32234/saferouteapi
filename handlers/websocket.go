package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"saferoute/database"
	"saferoute/middleware"
	"saferoute/models"
	"saferoute/services"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

var (
	suscriptores           = make(map[string]map[*websocket.Conn]bool)
	suscriptoresPorUsuario = make(map[string]map[string]bool)
	subMu                  sync.RWMutex
	jwtSecret              string
	viajeSvc               *services.ViajeService
)

func SetViajeService(svc *services.ViajeService) {
	viajeSvc = svc
}

// ============================================================
// NUEVO: Tipos de mensaje WebSocket
// ============================================================
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

// MensajeTelemetria representa datos GPS en tiempo real
type MensajeTelemetria struct {
	Tipo      string  `json:"tipo"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Velocidad float64 `json:"velocidad_kmh,omitempty"`
	RutaID    string  `json:"ruta_id"`
	Timestamp string  `json:"timestamp"`
}

// MensajeConfirmacion representa validación de un reporte
type MensajeConfirmacion struct {
	Tipo       string `json:"tipo"`
	ReporteID  string `json:"reporte_id"`
	Vigente    bool   `json:"vigente"`
	UserID     string `json:"user_id"`
	Timestamp  string `json:"timestamp"`
}

// MensajeEntrante es cualquier mensaje que llega del cliente
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

func SetJWTSecret(secret string) {
	jwtSecret = secret
}

// ============================================================
// WebSocketHandler - Multimensaje
// ============================================================

func WebSocketHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rutaID := mux.Vars(r)["ruta_id"]

		// Autenticación
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && len(authHeader) > 7 {
				token := authHeader[7:]
				if jwtSecret != "" {
					if claims, err := middleware.ValidateToken(token, jwtSecret); err == nil {
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

		log.Printf("🔌 [WS] Nueva conexión: User=%s, Ruta=%s", userID, rutaID)

		// Guardar suscripción en BD
		if userID != "anonimo" && userID != "admin" && database.DB != nil {
		    _, err := database.DB.Exec(
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

		// Upgrade
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("❌ [WS] Error upgrading: %v", err)
			return
		}
		defer func() {
			conn.Close()
			// Limpiar al desconectar
			subMu.Lock()
			delete(suscriptores[rutaID], conn)
			if len(suscriptores[rutaID]) == 0 {
				delete(suscriptores, rutaID)
			}
			if suscriptoresPorUsuario[userID] != nil {
				delete(suscriptoresPorUsuario[userID], rutaID)
				if len(suscriptoresPorUsuario[userID]) == 0 {
					delete(suscriptoresPorUsuario, userID)
				}
			}
			subMu.Unlock()
			log.Printf("🔌 [WS] Usuario %s desconectado de ruta %s", userID, rutaID)
		}()

		// Registrar en memoria
		subMu.Lock()
		if suscriptores[rutaID] == nil {
			suscriptores[rutaID] = make(map[*websocket.Conn]bool)
		}
		suscriptores[rutaID][conn] = true

		if suscriptoresPorUsuario[userID] == nil {
			suscriptoresPorUsuario[userID] = make(map[string]bool)
		}
		suscriptoresPorUsuario[userID][rutaID] = true
		subMu.Unlock()

		log.Printf("✅ [WS] Usuario %s registrado. Total conexiones: %d", userID, len(suscriptores[rutaID]))

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

		// Enviar historial inicial
		if userID != "anonimo" {
			go enviarHistorialReciente(userID, conn)
		}

		// ============================================================
		// BUCLE PRINbraCIPAL: LEER MENSAJES DEL CLIENTE
		// ============================================================
		for {
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS] Error lectura (desconexión): %v", err)
				break
			}

			// Parsear mensaje
			var msg MensajeEntrante
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				log.Printf("⚠️ [WS] Mensaje inválido de %s: %v", userID, err)
				continue
			}
			fmt.Print(msg)

			switch msg.Tipo {
			// ============================================================
			// TELEMETRÍA: El conductor envía su ubicación periódicamente
			// ============================================================
			case MsgTelemetria:
		    log.Printf("📍 [TELEMETRÍA] User=%s Lat=%.6f Lon=%.6f Vel=%.0f km/h Ruta=%s",
		        userID, msg.Lat, msg.Lon, msg.Velocidad, msg.RutaID)
					
		    // 1. Actualizar ubicación (omitir si user_id no es UUID)
		    if userID != "admin" && userID != "anonimo" && database.DB != nil {
		        _, err := database.DB.Exec(
		            `INSERT INTO zonas_usuario (user_id, zona_nombre, latitud, longitud, radio_km, activo, fecha_actualizacion)
		             VALUES ($1, 'ubicacion_actual', $2, $3, 15.0, true, NOW())
		             ON CONFLICT (user_id, zona_nombre)
		             DO UPDATE SET latitud = $2, longitud = $3, fecha_actualizacion = NOW()`,
		            userID, msg.Lat, msg.Lon,
		        )
		        if err != nil {
		            log.Printf("⚠️ [TELEMETRÍA] No se actualizó ubicación (user_id no es UUID): %v", err)
		        }
		    }

		    // NUEVO: Registrar en el viaje y verificar desvíos
		    var nuevoEstado string
		    var alertaDesvio bool
		    if viajeSvc != nil && userID != "admin" && userID != "anonimo" {
		        var err error
		        nuevoEstado, alertaDesvio, err = viajeSvc.ActualizarUbicacionViaje(userID, msg.Lat, msg.Lon, msg.Velocidad)
		        if err != nil {
		            log.Printf("⚠️ [TELEMETRÍA] Error actualizando viaje para %s: %v", userID, err)
		        }
		    }
		
		    // 2. Buscar reportes cercanos y notificar al conductor
		    if database.DB != nil {
		        go verificarProximidadYNotificar(userID, msg.Lat, msg.Lon, msg.RutaID, conn)
		    }
		
		    // 3. Confirmar recepción al emisor
		    resp := map[string]interface{}{
		        "tipo":      "telemetria_ack",
		        "status":    "ok",
		        "timestamp": time.Now().UTC().Format(time.RFC3339),
		    }
		    if nuevoEstado != "" {
		        resp["estado_viaje"] = nuevoEstado
		    }
		    conn.WriteJSON(resp)
		    syncInteraccionMotor("telemetria", userID, msg.RutaID, map[string]interface{}{
		        "lat": msg.Lat,
		        "lon": msg.Lon,
		        "velocidad_kmh": msg.Velocidad,
		        "timestamp_cliente": msg.Timestamp,
		    })
		
		    // ============================================================
		    // NUEVO: Retransmitir telemetría y desvíos a todos los admin-monitor
		    // ============================================================
		    subMu.RLock()
		    if adminConns, ok := suscriptores["admin-monitor"]; ok {
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
		            if adminConn != conn { // No reenviar al mismo que envió
		                adminConn.WriteMessage(websocket.TextMessage, telemetriaBytes)
		            }
		        }

		        // Si hay alerta de desvío, emitir un mensaje de alerta explícito
		        if alertaDesvio {
		            alertaMsg := map[string]interface{}{
		                "tipo":      MsgAlertaDesvio,
		                "user_id":   userID,
		                "ruta_id":   msg.RutaID,
		                "lat":       msg.Lat,
		                "lon":       msg.Lon,
		                "mensaje":   fmt.Sprintf("🚨 Conductor %s se ha desviado de su ruta planificada", userID),
		                "timestamp": time.Now().UTC().Format(time.RFC3339),
		            }
		            alertaBytes, _ := json.Marshal(alertaMsg)
		            for adminConn := range adminConns {
		                adminConn.WriteMessage(websocket.TextMessage, alertaBytes)
		            }
		        }
		    }
		    subMu.RUnlock()

			// ============================================================
			// CONFIRMACIÓN: El conductor valida/descarta un reporte
			// ============================================================
			case MsgConfirmacion:
				log.Printf("✅ [CONFIRMACIÓN] User=%s Reporte=%s Vigente=%v",
					userID, msg.ReporteID, msg.Vigente != nil && *msg.Vigente)

				if msg.ReporteID != "" && database.DB != nil {
					vigente := false
					if msg.Vigente != nil {
						vigente = *msg.Vigente
					}

					if vigente {
						database.DB.Exec(
							"UPDATE reportes SET confirmaciones = confirmaciones + 1 WHERE id = $1",
							msg.ReporteID,
						)
					} else {
						database.DB.Exec(
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
					syncReporteValidado(msg.ReporteID, vigente)
					syncInteraccionMotor("confirmacion_reporte", userID, msg.RutaID, map[string]interface{}{
						"reporte_id": msg.ReporteID,
						"vigente":    vigente,
					})
				}

			// ============================================================
			// ESTADO DEL CONDUCTOR
			// ============================================================
			case MsgEstadoConductor:
				syncInteraccionMotor("estado_conductor", userID, msg.RutaID, map[string]interface{}{
					"estado": msg.Estado,
					"lat":    msg.Lat,
					"lon":    msg.Lon,
				})
				log.Printf("🚦 [ESTADO] User=%s Estado=%s", userID, msg.Estado)

				// Broadcast a administradores u otros conductores
				if msg.Estado == "emergencia" {
					alerta := models.NotificacionAlerta{
						Tipo:      "emergencia_conductor",
						ReporteID: "",
						Latitud:   msg.Lat,
						Longitud:  msg.Lon,
						NotaVoz:   fmt.Sprintf("Conductor %s marcó emergencia", userID),
						RutaID:    msg.RutaID,
						Timestamp: time.Now().UTC(),
						Mensaje:   fmt.Sprintf("🚨 Conductor %s necesita ayuda", userID),
					}
					go BroadcastNotificacion(alerta)
				}

			// ============================================================
			// SYNC: Reportes pendientes guardados offline
			// ============================================================
			case MsgSyncPendientes:
				log.Printf("📤 [SYNC] User=%s enviando %d reportes pendientes", userID, len(msg.Reportes))
				subidos := 0
				for _, repRaw := range msg.Reportes {
					rep, ok := repRaw.(map[string]interface{})
					if !ok {
						continue
					}
					// Insertar reporte
					tipo, _ := rep["tipo"].(string)
					lat, _ := rep["latitud"].(float64)
					lon, _ := rep["longitud"].(float64)
					nota, _ := rep["nota_voz"].(string)
					ruta, _ := rep["ruta_id"].(string)

					if tipo != "" && lat != 0 && lon != 0 {
						var reporte models.ReporteResponse
						err := database.DB.QueryRow(
							`INSERT INTO reportes (user_id, tipo, latitud, longitud, nota_voz, ruta_id)
							 VALUES ($1, $2, $3, $4, $5, $6)
							 RETURNING id, tipo, latitud, longitud, COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones`,
							userID, tipo, lat, lon, nota, ruta,
						).Scan(
							&reporte.ID, &reporte.Tipo, &reporte.Latitud, &reporte.Longitud,
							&reporte.NotaVoz, &reporte.RutaID, &reporte.Timestamp,
							&reporte.Vigente, &reporte.Confirmaciones,
						)
						if err == nil {
							subidos++
							syncReporteCreado(reporte)
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
				log.Printf("⚠️ [WS] Tipo de mensaje desconocido: %s", msg.Tipo)
			}
		}
	}
}

// ============================================================
// VERIFICAR PROXIMIDAD Y NOTIFICAR
// ============================================================

func verificarProximidadYNotificar(userID string, lat, lon float64, rutaID string, conn *websocket.Conn) {
	rows, err := database.DB.Query(
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
			"mensaje":        fmt.Sprintf("⚠️ %s reportado cerca de tu ubicación", formatearTipoWS(tipo)),
		}
		conn.WriteJSON(alerta)
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

// ============================================================
// FUNCIONES EXISTENTES (mantener)
// ============================================================

func NotificarSuscriptores(rutaID string, data interface{}) {
	subMu.RLock()
	defer subMu.RUnlock()

	if suscriptores[rutaID] == nil {
		return
	}

	msg, err := json.Marshal(data)
	if err != nil {
		return
	}

	for conn := range suscriptores[rutaID] {
		conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func BroadcastNotificacion(notificacion models.NotificacionAlerta) {
	log.Printf("📡 [BROADCAST] Broadcast para reporte en (%.6f, %.6f)", notificacion.Latitud, notificacion.Longitud)

	db := database.GetDB()
	if db == nil {
		return
	}

	// Buscar usuarios por zona geográfica
	rows, err := db.Query(
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
		rows.Scan(&uid)
		userIDs = append(userIDs, uid)
	}

	// Guardar en historial
	for _, uid := range userIDs {
		GuardarNotificacion(uid, notificacion)
	}

	// Notificar en tiempo real
	subMu.RLock()
	for uid, rutas := range suscriptoresPorUsuario {
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
			if conns, ok := suscriptores[rutaID]; ok {
				for conn := range conns {
					conn.WriteMessage(websocket.TextMessage, msg)
				}
			}
		}
	}
	subMu.RUnlock()
}

func GuardarNotificacion(userID string, notificacion models.NotificacionAlerta) error {
	if database.DB == nil {
		return fmt.Errorf("base de datos no conectada")
	}

	_, err := database.DB.Exec(
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

func enviarHistorialReciente(userID string, conn *websocket.Conn) {
	db := database.GetDB()
	if db == nil {
		return
	}

	rows, err := db.Query(
		`SELECT id, tipo, COALESCE(reporte_id,''), latitud, longitud, 
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

func GetEstadoSuscriptores() map[string]interface{} {
	subMu.RLock()
	defer subMu.RUnlock()

	estado := make(map[string]interface{})
	estado["total_rutas"] = len(suscriptores)
	estado["total_usuarios"] = len(suscriptoresPorUsuario)

	rutas := make(map[string]int)
	for rutaID, conns := range suscriptores {
		rutas[rutaID] = len(conns)
	}
	estado["rutas"] = rutas
	return estado
}

// BroadcastAdminMonitor transmite datos a todos los clientes del canal admin-monitor
func BroadcastAdminMonitor(data interface{}) {
	subMu.RLock()
	defer subMu.RUnlock()

	adminConns, ok := suscriptores["admin-monitor"]
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
