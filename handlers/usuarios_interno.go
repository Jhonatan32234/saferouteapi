package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"saferoute/database"
)

func GetUsuariosInternoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := database.DB.Query(`
			SELECT 
				u.id, 
				u.nombre, 
				u.tipo,
				u.created_at,
				COALESCE(u.ultimo_acceso, u.created_at) as ultimo_acceso,
				COUNT(DISTINCT r.id) as total_reportes,
				COUNT(DISTINCT CASE WHEN r.confirmaciones > 0 THEN r.id END) as reportes_confirmados,
				COUNT(DISTINCT r.ruta_id) as total_rutas,
				COUNT(DISTINCT CASE WHEN r.vigente = false THEN r.id END) as reportes_resueltos,
				COUNT(DISTINCT v.id) as total_viajes,
				COUNT(DISTINCT s.ruta_id) as rutas_suscritas,
				COUNT(DISTINCT n.id) as total_notificaciones,
				COUNT(DISTINCT CASE WHEN n.leida = false THEN n.id END) as notificaciones_no_leidas
			FROM usuarios u
			LEFT JOIN reportes r ON u.id = r.user_id
			LEFT JOIN viajes v ON u.id = v.user_id
			LEFT JOIN suscripciones_rutas s ON u.id = s.user_id AND s.suscrito = true
			LEFT JOIN notificaciones_historial n ON u.id = n.user_id
			WHERE u.tipo = 'conductor'
			GROUP BY u.id, u.nombre, u.tipo, u.created_at, u.ultimo_acceso
			ORDER BY total_reportes DESC
		`)
		if err != nil {
			log.Printf("ERROR consultando usuarios: %v", err)
			writeError(w, http.StatusInternalServerError, "error consultando usuarios")
			return
		}
		defer rows.Close()

		var usuarios []map[string]interface{}
		for rows.Next() {
			var id, nombre, tipo string
			var createdAt, ultimoAcceso interface{}
			var totalReportes, reportesConfirmados, totalRutas, reportesResueltos, totalViajes, rutasSuscritas, totalNotificaciones, noLeidas int

			err := rows.Scan(
				&id, &nombre, &tipo, &createdAt, &ultimoAcceso,
				&totalReportes, &reportesConfirmados, &totalRutas,
				&reportesResueltos, &totalViajes,
				&rutasSuscritas, &totalNotificaciones, &noLeidas,
			)
			if err != nil {
				log.Printf("ERROR escaneando usuario: %v", err)
				continue
			}

			// Calcular métricas derivadas
			precision := 0.0
			if totalReportes > 0 {
				precision = float64(reportesConfirmados) / float64(totalReportes) * 100
			}

			alertasIgnoradas := noLeidas
			if alertasIgnoradas < 0 {
				alertasIgnoradas = 0
			}

			rutasPeligrosasPct := 30.0
			if totalReportes > 0 && reportesResueltos > 0 {
				rutasPeligrosasPct = float64(reportesResueltos) / float64(totalReportes) * 100
			}
			if rutasPeligrosasPct > 100 {
				rutasPeligrosasPct = 100
			}

			horarioPred := "diurno"
			if noLeidas > totalNotificaciones/2 && totalNotificaciones > 0 {
				horarioPred = "nocturno"
			} else if totalNotificaciones > 10 {
				horarioPred = "mixto"
			}

			usuarios = append(usuarios, map[string]interface{}{
				"conductor_id":          id,
				"nombre":                nombre,
				"tipo":                  tipo,
				"tipo_conductor":        mapearTipoConductor(nombre, tipo),
				"total_rutas":           totalRutas,
				"rutas_peligrosas_pct":  round(rutasPeligrosasPct, 1),
				"total_reportes":        totalReportes,
				"reportes_confirmados":  reportesConfirmados,
				"precision_reportes":    round(precision, 1),
				"alertas_ignoradas":     alertasIgnoradas,
				"horario_predominante":  horarioPred,
				"total_viajes":          totalViajes,
				"rutas_suscritas":       rutasSuscritas,
			})
		}

		if usuarios == nil {
			usuarios = []map[string]interface{}{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"usuarios": usuarios,
			"total":    len(usuarios),
		})
	}
}
func mapearTipoConductor(nombre, tipo string) string {
	nombreLower := nombre
	if len(nombre) > 15 {
		nombreLower = nombre[:15]
	}
	
	// Heurística simple basada en el nombre
	if contiene(nombreLower, "taxi") || contiene(nombreLower, "taxista") {
		return "taxista"
	}
	if contiene(nombreLower, "comer") || contiene(nombreLower, "carga") {
		return "comerciante"
	}
	if contiene(nombreLower, "proteccion") || contiene(nombreLower, "civil") || contiene(nombreLower, "emergencia") {
		return "proteccion_civil"
	}
	
	return "particular"
}

func contiene(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		(s == substr || 
		 (len(s) >= len(substr) && 
		  (s[:len(substr)] == substr || 
		   s[len(s)-len(substr):] == substr ||
		   containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func round(val float64, precision int) float64 {
	format := float64(1)
	for i := 0; i < precision; i++ {
		format *= 10
	}
	return float64(int(val*format)) / format
}