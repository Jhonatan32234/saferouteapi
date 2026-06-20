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
    jwtSecret              string // Guardar el secret para usar en el handler

)


func SetJWTSecret(secret string) {
    jwtSecret = secret
}


// handlers/websocket.go - WebSocketHandler mejorado

func WebSocketHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        rutaID := mux.Vars(r)["ruta_id"]
        
        // Obtener user_id
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

        // === IMPORTANTE: Guardar suscripción en BD ===
        if userID != "anonimo" {
            log.Printf("💾 [WS] Guardando suscripción para usuario %s en ruta %s", userID, rutaID)
            
            if database.DB != nil {
                _, err := database.DB.Exec(
                    `INSERT INTO suscripciones_rutas (user_id, ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion)
                     VALUES ($1, $2, true, NOW(), NOW())
                     ON CONFLICT (user_id, ruta_id) 
                     DO UPDATE SET 
                        suscrito = true, 
                        fecha_actualizacion = NOW()`,
                    userID, rutaID,
                )
                if err != nil {
                    log.Printf("❌ [WS] Error guardando suscripción: %v", err)
                } else {
                    log.Printf("✅ [WS] Suscripción guardada para usuario %s en ruta %s", userID, rutaID)
                }
            }
        }

        // Upgrade a WebSocket
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Printf("❌ [WS] Error upgrading: %v", err)
            return
        }
        defer conn.Close()

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

        log.Printf("✅ [WS] Usuario %s registrado en ruta %s. Total conexiones: %d",
            userID, rutaID, len(suscriptores[rutaID]))

        // Enviar historial reciente
        if userID != "anonimo" {
            go enviarHistorialReciente(userID, conn)
        }

        // Mantener conexión
        for {
            _, _, err := conn.ReadMessage()
            if err != nil {
                subMu.Lock()
                delete(suscriptores[rutaID], conn)
                if len(suscriptores[rutaID]) == 0 {
                    delete(suscriptores, rutaID)
                }
                delete(suscriptoresPorUsuario[userID], rutaID)
                if len(suscriptoresPorUsuario[userID]) == 0 {
                    delete(suscriptoresPorUsuario, userID)
                }
                subMu.Unlock()
                log.Printf("🔌 [WS] Usuario %s desconectado de ruta %s", userID, rutaID)
                break
            }
        }
    }
}


// NotificarSuscriptores envía notificación a todos los suscriptores de una ruta
func NotificarSuscriptores(rutaID string, data interface{}) {
    subMu.RLock()
    defer subMu.RUnlock()

    if suscriptores[rutaID] == nil {
        return
    }

    msg, err := json.Marshal(data)
    if err != nil {
        log.Printf("❌ [NOTIFICAR] Error marshaling: %v", err)
        return
    }

    count := 0
    for conn := range suscriptores[rutaID] {
        if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
            log.Printf("❌ [NOTIFICAR] Error enviando a conexión: %v", err)
        } else {
            count++
        }
    }
    log.Printf("📨 [NOTIFICAR] Notificación enviada a %d/%d conexiones en ruta %s", 
        count, len(suscriptores[rutaID]), rutaID)
}



// handlers/websocket.go - BroadcastNotificacion mejorado con zonas

func BroadcastNotificacion(notificacion models.NotificacionAlerta) {
    log.Printf("📡 [BROADCAST] Iniciando broadcast para reporte en (%.6f, %.6f)", 
        notificacion.Latitud, notificacion.Longitud)
    
    db := database.GetDB()
    if db == nil {
        log.Printf("❌ [BROADCAST] Base de datos no conectada")
        return
    }
    
    // ==========================================
    // NUEVO: Buscar usuarios por zona geográfica
    // ==========================================
    query := `
        SELECT DISTINCT zu.user_id 
        FROM zonas_usuario zu
        WHERE zu.activo = true
        AND (6371 * acos(
            cos(radians($1)) * cos(radians(zu.latitud)) * 
            cos(radians(zu.longitud) - radians($2)) + 
            sin(radians($1)) * sin(radians(zu.latitud))
        )) <= zu.radio_km
    `
    
    log.Printf("📝 [BROADCAST] Buscando usuarios en zona cercana a (%.6f, %.6f) radio 15km", 
        notificacion.Latitud, notificacion.Longitud)
    
    rows, err := db.Query(query, notificacion.Latitud, notificacion.Longitud)
    if err != nil {
        log.Printf("❌ [BROADCAST] Error obteniendo usuarios por zona: %v", err)
        return
    }
    defer rows.Close()

    var userIDs []string
    for rows.Next() {
        var userID string
        if err := rows.Scan(&userID); err != nil {
            log.Printf("⚠️ [BROADCAST] Error escaneando: %v", err)
            continue
        }
        userIDs = append(userIDs, userID)
        log.Printf("  👤 [BROADCAST] Usuario en zona: %s", userID)
    }

    // ==========================================
    // TAMBIÉN: Buscar suscriptores por ruta exacta (fallback)
    // ==========================================
    if notificacion.RutaID != "" {
        rows2, err := db.Query(
            "SELECT user_id FROM suscripciones_rutas WHERE ruta_id = $1 AND suscrito = true",
            notificacion.RutaID,
        )
        if err == nil {
            for rows2.Next() {
                var userID string
                if err := rows2.Scan(&userID); err == nil {
                    // Evitar duplicados
                    encontrado := false
                    for _, id := range userIDs {
                        if id == userID {
                            encontrado = true
                            break
                        }
                    }
                    if !encontrado {
                        userIDs = append(userIDs, userID)
                        log.Printf("  👤 [BROADCAST] Usuario suscrito a ruta: %s", userID)
                    }
                }
            }
            rows2.Close()
        }
    }

    log.Printf("👥 [BROADCAST] Total usuarios a notificar: %d", len(userIDs))

    // Guardar en historial de cada usuario
    for i, userID := range userIDs {
        log.Printf("  💾 [BROADCAST] [%d/%d] Guardando para usuario %s", i+1, len(userIDs), userID)
        if err := GuardarNotificacion(userID, notificacion); err != nil {
            log.Printf("  ❌ [BROADCAST] Error guardando para %s: %v", userID, err)
        } else {
            log.Printf("  ✅ [BROADCAST] Guardado para %s", userID)
        }
    }

    // Notificar en tiempo real
    log.Printf("📨 [BROADCAST] Enviando en tiempo real...")
    
    // Notificar a usuarios conectados (en cualquier ruta)
    subMu.RLock()
    for userID, rutas := range suscriptoresPorUsuario {
        // Verificar si el usuario está en la lista de destinatarios
        destinatario := false
        for _, id := range userIDs {
            if id == userID {
                destinatario = true
                break
            }
        }
        
        if !destinatario {
            continue
        }
        
        // Enviar a todas las conexiones activas del usuario
        for rutaID := range rutas {
            if conns, ok := suscriptores[rutaID]; ok {
                msg, _ := json.Marshal(notificacion)
                for conn := range conns {
                    if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
                        log.Printf("❌ [BROADCAST] Error enviando a %s: %v", userID, err)
                    } else {
                        log.Printf("✅ [BROADCAST] Enviado a %s", userID)
                    }
                }
            }
        }
    }
    subMu.RUnlock()
    
    log.Printf("✅ [BROADCAST] Broadcast completado para %d usuarios", len(userIDs))
}
// handlers/websocket.go - GuardarNotificacion CORREGIDO

func GuardarNotificacion(userID string, notificacion models.NotificacionAlerta) error {
    if err := database.EnsureConnection(); err != nil {
        return fmt.Errorf("error de conexión: %v", err)
    }
    
    db := database.GetDB()
    if db == nil {
        return fmt.Errorf("base de datos no conectada")
    }

    log.Printf("  💾 [GUARDAR] Insertando notificación para usuario %s", userID)

    var id string
    err := database.DB.QueryRow(
        `INSERT INTO notificaciones_historial 
         (user_id, tipo, reporte_id, latitud, longitud, nota_voz, ruta_id, mensaje, fecha_envio)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
         RETURNING id`,
        userID, notificacion.Tipo, notificacion.ReporteID,
        notificacion.Latitud, notificacion.Longitud,
        notificacion.NotaVoz, notificacion.RutaID,
        notificacion.Mensaje,
    ).Scan(&id)

    if err != nil {
        log.Printf("  ❌ [GUARDAR] Error guardando para usuario %s: %v", userID, err)
        return err
    }

    log.Printf("  ✅ [GUARDAR] Notificación guardada con ID %s para usuario %s", id, userID)
    return nil
}

// enviarHistorialReciente envía notificaciones no leídas
func enviarHistorialReciente(userID string, conn *websocket.Conn) {
    db := database.GetDB()
    if db == nil {
        log.Printf("❌ [HISTORIAL] Base de datos no conectada")
        return
    }

    rows, err := db.Query(
        `SELECT id, tipo, reporte_id, latitud, longitud, nota_voz, 
                ruta_id, mensaje, fecha_envio
         FROM notificaciones_historial 
         WHERE user_id = $1 AND leida = false
         ORDER BY fecha_envio DESC LIMIT 10`,
        userID,
    )
    if err != nil {
        log.Printf("❌ [HISTORIAL] Error consultando historial: %v", err)
        return
    }
    defer rows.Close()

    var notificaciones []map[string]interface{}
    for rows.Next() {
        var n struct {
            ID         string
            Tipo       string
            ReporteID  string
            Latitud    float64
            Longitud   float64
            NotaVoz    string
            RutaID     string
            Mensaje    string
            FechaEnvio time.Time
        }
        err := rows.Scan(&n.ID, &n.Tipo, &n.ReporteID, &n.Latitud,
            &n.Longitud, &n.NotaVoz, &n.RutaID, &n.Mensaje, &n.FechaEnvio)
        if err != nil {
            continue
        }

        notificacion := map[string]interface{}{
            "id":          n.ID,
            "tipo_alerta": n.Tipo,
            "reporte_id":  n.ReporteID,
            "latitud":     n.Latitud,
            "longitud":    n.Longitud,
            "nota_voz":    n.NotaVoz,
            "ruta_id":     n.RutaID,
            "mensaje":     n.Mensaje,
            "fecha_envio": n.FechaEnvio,
            "leida":       false,
        }
        notificaciones = append(notificaciones, notificacion)
    }

    if len(notificaciones) > 0 {
        response := map[string]interface{}{
            "tipo":            "historial_inicial",
            "notificaciones":  notificaciones,
            "total_no_leidas": len(notificaciones),
        }
        if err := conn.WriteJSON(response); err != nil {
            log.Printf("❌ [HISTORIAL] Error enviando historial: %v", err)
        } else {
            log.Printf("✅ [HISTORIAL] Historial enviado a usuario %s", userID)
        }
    }
}

// GetEstadoSuscriptores - Función de debug
func GetEstadoSuscriptores() map[string]interface{} {
    subMu.RLock()
    defer subMu.RUnlock()
    
    estado := make(map[string]interface{})
    estado["total_rutas"] = len(suscriptores)
    
    rutas := make(map[string]int)
    for rutaID, conns := range suscriptores {
        rutas[rutaID] = len(conns)
    }
    estado["rutas"] = rutas
    
    estado["total_usuarios"] = len(suscriptoresPorUsuario)
    return estado
}