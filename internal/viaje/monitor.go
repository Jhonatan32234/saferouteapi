package viaje

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

func StartSignalTimeoutMonitor(db *sql.DB, checkInterval, timeout time.Duration) {
	ticker := time.NewTicker(checkInterval)
	go func() {
		for range ticker.C {
			rows, err := db.Query(`
				SELECT v.id, v.user_id, u.nombre, v.ultimo_heartbeat 
				FROM viajes v
				JOIN usuarios u ON u.id = v.user_id
				WHERE v.estado IN ('activo', 'desviado', 'parada_tecnica')
				  AND v.ultimo_heartbeat < NOW() - INTERVAL '1 second' * $1`,
				timeout.Seconds(),
			)
			if err != nil {
				log.Printf("[HEARTBEAT] Error consultando timeouts: %v", err)
				continue
			}

			type viajeAfectado struct {
				id       string
				userID   string
				nombre   string
				ultimoHB time.Time
			}
			var viajesAfectados []viajeAfectado

			for rows.Next() {
				var v viajeAfectado
				if err := rows.Scan(&v.id, &v.userID, &v.nombre, &v.ultimoHB); err == nil {
					viajesAfectados = append(viajesAfectados, v)
				}
			}
			rows.Close()

			for _, v := range viajesAfectados {
				log.Printf("[HEARTBEAT] Señal perdida con conductor %s (Viaje: %s). Último contacto: %v", v.nombre, v.id, v.ultimoHB)
				
				_, err := db.Exec("UPDATE viajes SET estado = 'contacto_perdido' WHERE id = $1", v.id)
				if err != nil {
					log.Printf("[HEARTBEAT] Error actualizando estado de viaje %s: %v", v.id, err)
					continue
				}

				var lastLat, lastLon float64
				err = db.QueryRow(`
					SELECT latitud, longitud 
					FROM historial_viaje_coordenadas 
					WHERE viaje_id = $1 
					ORDER BY timestamp DESC LIMIT 1`, 
					v.id,
				).Scan(&lastLat, &lastLon)

				alerta := map[string]interface{}{
					"tipo":                 "alerta_timeout",
					"viaje_id":             v.id,
					"user_id":              v.userID,
					"nombre_conductor":     v.nombre,
					"mensaje":              fmt.Sprintf("Se perdió la señal del conductor %s. Último contacto: %s", v.nombre, v.ultimoHB.Local().Format("15:04:05")),
					"ultimo_contacto_time": v.ultimoHB.Format(time.RFC3339),
					"lat":                  lastLat,
					"lon":                  lastLon,
					"timestamp":            time.Now().UTC().Format(time.RFC3339),
				}
				BroadcastAdminMonitor(alerta)
			}
		}
	}()
}
