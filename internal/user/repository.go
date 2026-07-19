package user

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

type Repository interface {
	FindByEmail(email string) (*UsuarioEntity, error)
	FindByID(id string) (*UsuarioPerfilConEstadisticas, error)
	Create(u *UsuarioEntity) (string, error)
	Update(u *UsuarioEntity) error
	UpdateLastAccess(userID string) error

	// Destinos
	SaveDestino(userID string, nombre string, lat, lon float64) error
	GetDestinos(userID string, limit int) ([]DestinoRecienteResponse, error)
	DeleteDestino(userID string, destinoID string) error

	// Zonas
	UpsertZonas(userID string, zonas []ZonaRequest) error
	GetZonas(userID string) ([]ZonaUsuario, error)

	// Suscripciones
	SubscribeRuta(userID string, rutaID string) error
	UnsubscribeRuta(userID string, rutaID string) error
	GetSubscriptions(userID string) ([]SuscripcionRuta, error)

	// Notificaciones
	GetNotifications(userID string, page, limit int, soloNoLeidas bool) ([]NotificacionHistorial, error)
	CountNotifications(userID string, soloNoLeidas bool) (int, error)
	CountUnreadNotifications(userID string) (int, error)
	MarkNotification(userID string, notifID string, leida bool) error
	MarkAllNotificationsRead(userID string) error
	SaveNotification(userID string, n *NotificacionEntity) error
	SyncNotifications(userID string, since time.Time) ([]NotificacionHistorial, error)

	// Conductores interno
	ListConductors() ([]map[string]interface{}, error)
}

type userRepository struct {
	db            *sql.DB
	encryptionKey []byte
}

func NewRepository(db *sql.DB, encryptionKey []byte) Repository {
	return &userRepository{db: db, encryptionKey: encryptionKey}
}

func (r *userRepository) FindByEmail(email string) (*UsuarioEntity, error) {
	u := &UsuarioEntity{}
	err := r.db.QueryRow(
		`SELECT id, email, password_hash, nombre, tipo, COALESCE(telefono, ''), created_at, updated_at
		 FROM usuarios WHERE email = $1`,
		email,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Nombre,
		&u.Tipo, &u.Telefono, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := u.AfterLoad(r.encryptionKey); err != nil {
		return nil, fmt.Errorf("AfterLoad error: %w", err)
	}
	return u, nil
}

func (r *userRepository) FindByID(id string) (*UsuarioPerfilConEstadisticas, error) {
    u := &UsuarioPerfilConEstadisticas{}
    var ultimoAcceso sql.NullTime

    err := r.db.QueryRow(
        `SELECT u.id, u.email, u.password_hash, u.nombre, u.tipo, COALESCE(u.telefono, ''),
                u.created_at, u.updated_at, u.ultimo_acceso,
                COALESCE(COUNT(r.id), 0) AS reportes_creados,
                COALESCE(SUM(CASE WHEN r.confirmaciones > 0 THEN 1 ELSE 0 END), 0) AS reportes_confirmados
         FROM usuarios u
         LEFT JOIN reportes r ON r.user_id = u.id
         WHERE u.id = $1
         GROUP BY u.id, u.email, u.password_hash, u.nombre, u.tipo, u.telefono,
                  u.created_at, u.updated_at, u.ultimo_acceso`,
        id,
    ).Scan(
        &u.ID, &u.Email, &u.PasswordHash, &u.Nombre,
        &u.Tipo, &u.Telefono, &u.CreatedAt, &u.UpdatedAt, &ultimoAcceso,
        &u.ReportesCreados, &u.ReportesConfirmados,
    )
    if err != nil {
        return nil, err
    }

    if ultimoAcceso.Valid {
        t := ultimoAcceso.Time
        u.UltimoAcceso = &t
    }

    if err := u.AfterLoad(r.encryptionKey); err != nil {
        return nil, fmt.Errorf("AfterLoad error: %w", err)
    }

    return u, nil
}

func (r *userRepository) Create(u *UsuarioEntity) (string, error) {
    log.Printf("[REPO] Creando usuario - Email: %s, Teléfono antes de BeforeSave: '%s'", 
        u.Email, u.Telefono)

    if err := u.BeforeSave(r.encryptionKey); err != nil {
        return "", fmt.Errorf("BeforeSave error: %w", err)
    }

    log.Printf("[REPO] Teléfono después de BeforeSave: '%s'", u.Telefono)

    var id string
    err := r.db.QueryRow(
        `INSERT INTO usuarios (email, password_hash, nombre, tipo, telefono)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING id`,
        u.Email, u.PasswordHash, u.Nombre, u.Tipo, u.Telefono,
    ).Scan(&id)
    
    if err != nil {
        log.Printf("[REPO] Error INSERT: %v", err)
        return "", err
    }

    log.Printf("[REPO] Usuario creado - ID: %s", id)
    return id, nil
}

func (r *userRepository) Update(u *UsuarioEntity) error {
    if err := u.BeforeSave(r.encryptionKey); err != nil {
        return fmt.Errorf("BeforeSave error: %w", err)
    }

    query := "UPDATE usuarios SET updated_at = NOW()"
    args := []interface{}{}
    argCount := 0

    if u.Nombre != "" {
        argCount++
        query += fmt.Sprintf(", nombre = $%d", argCount)
        args = append(args, u.Nombre)
    }
    
    argCount++
    query += fmt.Sprintf(", telefono = NULLIF($%d, '')", argCount)
    args = append(args, u.Telefono)
    
    if u.Email != "" {
        argCount++
        query += fmt.Sprintf(", email = $%d", argCount)
        args = append(args, u.Email)
    }

    argCount++
    query += fmt.Sprintf(" WHERE id = $%d", argCount)
    args = append(args, u.ID)

    log.Printf("[REPO] Update query: %s", query)
    log.Printf("[REPO] Update args: %v", args)

    result, err := r.db.Exec(query, args...)
    if err != nil {
        return err
    }

    rows, _ := result.RowsAffected()
    log.Printf("[REPO] Filas actualizadas: %d", rows)
    
    return nil
}

func (r *userRepository) UpdateLastAccess(userID string) error {
	_, err := r.db.Exec(
		"UPDATE usuarios SET ultimo_acceso = NOW() WHERE id = $1",
		userID,
	)
	return err
}

func (r *userRepository) SaveDestino(userID string, nombre string, lat, lon float64) error {
	var existingID string
	err := r.db.QueryRow(
		`SELECT id FROM historial_destinos WHERE user_id = $1 AND nombre = $2`,
		userID, nombre,
	).Scan(&existingID)

	if err == sql.ErrNoRows {
		_, err = r.db.Exec(
			`INSERT INTO historial_destinos (user_id, nombre, latitud, longitud, fecha_creacion)
			 VALUES ($1, $2, $3, $4, NOW())`,
			userID, nombre, lat, lon,
		)
		if err != nil {
			return err
		}
	} else if err == nil {
		_, err = r.db.Exec(
			`UPDATE historial_destinos SET latitud = $1, longitud = $2, fecha_creacion = NOW()
			 WHERE id = $3 AND user_id = $4`,
			lat, lon, existingID, userID,
		)
		if err != nil {
			return err
		}
	} else {
		return err
	}

	// Limpiar destinos excedentes de 10
	_, err = r.db.Exec(`
		DELETE FROM historial_destinos
		WHERE user_id = $1 AND id NOT IN (
			SELECT id FROM historial_destinos
			WHERE user_id = $1
			ORDER BY fecha_creacion DESC
			LIMIT 10
		)`,
		userID,
	)
	return err
}

func (r *userRepository) GetDestinos(userID string, limit int) ([]DestinoRecienteResponse, error) {
	rows, err := r.db.Query(
		`SELECT id, nombre, latitud, longitud, fecha_creacion
		 FROM historial_destinos
		 WHERE user_id = $1
		 ORDER BY fecha_creacion DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var destinos []DestinoRecienteResponse
	for rows.Next() {
		var d DestinoRecienteResponse
		if err := rows.Scan(&d.ID, &d.Nombre, &d.Lat, &d.Lon, &d.FechaCreacion); err != nil {
			return nil, err
		}
		destinos = append(destinos, d)
	}
	if destinos == nil {
		destinos = []DestinoRecienteResponse{}
	}
	return destinos, nil
}

func (r *userRepository) DeleteDestino(userID string, destinoID string) error {
	_, err := r.db.Exec(
		"DELETE FROM historial_destinos WHERE user_id = $1 AND id = $2",
		userID, destinoID,
	)
	return err
}

func (r *userRepository) UpsertZonas(userID string, zonas []ZonaRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, zona := range zonas {
		radio := zona.RadioKm
		if radio == 0 {
			radio = 15.0
		}

		_, err = tx.Exec(
			`INSERT INTO zonas_usuario (user_id, zona_nombre, latitud, longitud, radio_km, activo, fecha_actualizacion)
			 VALUES ($1, $2, $3, $4, $5, true, NOW())
			 ON CONFLICT (user_id, zona_nombre) 
			 DO UPDATE SET 
				latitud = EXCLUDED.latitud,
				longitud = EXCLUDED.longitud,
				radio_km = EXCLUDED.radio_km,
				activo = true,
				fecha_actualizacion = NOW()`,
			userID, zona.ZonaNombre, zona.Latitud, zona.Longitud, radio,
		)
		if err != nil {
			return err
		}
	}

	// Desactivar zonas que no vinieron en la petición
	var nombres []interface{}
	nombres = append(nombres, userID)
	query := "UPDATE zonas_usuario SET activo = false, fecha_actualizacion = NOW() WHERE user_id = $1"
	if len(zonas) > 0 {
		query += " AND zona_nombre NOT IN ("
		for idx, z := range zonas {
			if idx > 0 {
				query += ", "
			}
			query += fmt.Sprintf("$%d", idx+2)
			nombres = append(nombres, z.ZonaNombre)
		}
		query += ")"
	}
	_, err = tx.Exec(query, nombres...)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *userRepository) GetZonas(userID string) ([]ZonaUsuario, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, zona_nombre, latitud, longitud, radio_km, activo
		 FROM zonas_usuario
		 WHERE user_id = $1 AND activo = true`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zonas []ZonaUsuario
	for rows.Next() {
		var z ZonaUsuario
		err := rows.Scan(&z.ID, &z.UserID, &z.ZonaNombre, &z.Latitud, &z.Longitud, &z.RadioKm, &z.Activo)
		if err != nil {
			return nil, err
		}
		zonas = append(zonas, z)
	}
	if zonas == nil {
		zonas = []ZonaUsuario{}
	}
	return zonas, nil
}

func (r *userRepository) SubscribeRuta(userID string, rutaID string) error {
	_, err := r.db.Exec(
		`INSERT INTO suscripciones_rutas (user_id, ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion)
		 VALUES ($1, $2, true, NOW(), NOW())
		 ON CONFLICT (user_id, ruta_id) 
		 DO UPDATE SET suscrito = true, fecha_actualizacion = NOW()`,
		userID, rutaID,
	)
	return err
}

func (r *userRepository) UnsubscribeRuta(userID string, rutaID string) error {
	_, err := r.db.Exec(
		`UPDATE suscripciones_rutas 
		 SET suscrito = false, fecha_actualizacion = NOW()
		 WHERE user_id = $1 AND ruta_id = $2`,
		userID, rutaID,
	)
	return err
}

func (r *userRepository) GetSubscriptions(userID string) ([]SuscripcionRuta, error) {
	rows, err := r.db.Query(
		`SELECT user_id, ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion
		 FROM suscripciones_rutas
		 WHERE user_id = $1 AND suscrito = true`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []SuscripcionRuta
	for rows.Next() {
		var s SuscripcionRuta
		err := rows.Scan(&s.UserID, &s.RutaID, &s.Suscrito, &s.FechaSuscripcion, &s.FechaActualizacion)
		if err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	if subs == nil {
		subs = []SuscripcionRuta{}
	}
	return subs, nil
}

func (r *userRepository) GetNotifications(userID string, page, limit int, soloNoLeidas bool) ([]NotificacionHistorial, error) {
	offset := (page - 1) * limit
	query := `SELECT id, user_id, tipo, 
				COALESCE(reporte_id::text, '') as reporte_id,
				COALESCE(latitud, 0) as latitud, 
				COALESCE(longitud, 0) as longitud,
				COALESCE(nota_voz, '') as nota_voz, 
				ruta_id, mensaje,
				leida, fecha_envio, fecha_lectura
			  FROM notificaciones_historial 
			  WHERE user_id = $1`
	args := []interface{}{userID}
	argCount := 1

	if soloNoLeidas {
		argCount++
		query += fmt.Sprintf(" AND leida = $%d", argCount)
		args = append(args, false)
	}

	argCount++
	query += fmt.Sprintf(" ORDER BY fecha_envio DESC LIMIT $%d", argCount)
	args = append(args, limit)

	argCount++
	query += fmt.Sprintf(" OFFSET $%d", argCount)
	args = append(args, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []NotificacionHistorial
	for rows.Next() {
		var n NotificacionHistorial
		var repID sql.NullString
		var fechaLectura sql.NullTime
		err := rows.Scan(
			&n.ID, &n.UserID, &n.Tipo, &repID,
			&n.Latitud, &n.Longitud, &n.NotaVoz, &n.RutaID, &n.Mensaje,
			&n.Leida, &n.FechaEnvio, &fechaLectura,
		)
		if err != nil {
			return nil, err
		}
		if repID.Valid {
			n.ReporteID = repID.String
		}
		if fechaLectura.Valid {
			t := fechaLectura.Time
			n.FechaLectura = &t
		}
		list = append(list, n)
	}
	if list == nil {
		list = []NotificacionHistorial{}
	}
	return list, nil
}

func (r *userRepository) CountNotifications(userID string, soloNoLeidas bool) (int, error) {
	query := `SELECT COUNT(*) FROM notificaciones_historial WHERE user_id = $1`
	args := []interface{}{userID}
	if soloNoLeidas {
		query += " AND leida = false"
	}
	var total int
	err := r.db.QueryRow(query, args...).Scan(&total)
	return total, err
}

func (r *userRepository) CountUnreadNotifications(userID string) (int, error) {
	var noLeidas int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM notificaciones_historial WHERE user_id = $1 AND leida = false",
		userID,
	).Scan(&noLeidas)
	return noLeidas, err
}

func (r *userRepository) MarkNotification(userID string, notifID string, leida bool) error {
	var err error
	if leida {
		_, err = r.db.Exec(
			`UPDATE notificaciones_historial 
			 SET leida = true, fecha_lectura = NOW() 
			 WHERE id = $1 AND user_id = $2`,
			notifID, userID,
		)
	} else {
		_, err = r.db.Exec(
			`UPDATE notificaciones_historial 
			 SET leida = false, fecha_lectura = NULL 
			 WHERE id = $1 AND user_id = $2`,
			notifID, userID,
		)
	}
	return err
}

func (r *userRepository) MarkAllNotificationsRead(userID string) error {
	_, err := r.db.Exec(
		`UPDATE notificaciones_historial 
		 SET leida = true, fecha_lectura = NOW() 
		 WHERE user_id = $1 AND leida = false`,
		userID,
	)
	return err
}

func (r *userRepository) SaveNotification(userID string, n *NotificacionEntity) error {
	_, err := r.db.Exec(`
		INSERT INTO notificaciones_historial 
		(user_id, tipo, reporte_id, latitud, longitud, nota_voz, ruta_id, mensaje, leida, fecha_envio)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		userID, n.Tipo, n.ReporteID, n.Latitud, n.Longitud, n.NotaVoz, n.RutaID, n.Mensaje, n.Leida, n.FechaEnvio,
	)
	return err
}

func (r *userRepository) SyncNotifications(userID string, since time.Time) ([]NotificacionHistorial, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, tipo, 
				COALESCE(reporte_id::text, '') as reporte_id,
				COALESCE(latitud, 0) as latitud, 
				COALESCE(longitud, 0) as longitud,
				COALESCE(nota_voz, '') as nota_voz, 
				ruta_id, mensaje,
				leida, fecha_envio, fecha_lectura
		 FROM notificaciones_historial 
		 WHERE user_id = $1 AND fecha_envio > $2
		 ORDER BY fecha_envio DESC
		 LIMIT 100`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []NotificacionHistorial
	for rows.Next() {
		var n NotificacionHistorial
		var repID sql.NullString
		var fechaLectura sql.NullTime
		err := rows.Scan(
			&n.ID, &n.UserID, &n.Tipo, &repID,
			&n.Latitud, &n.Longitud, &n.NotaVoz, &n.RutaID, &n.Mensaje,
			&n.Leida, &n.FechaEnvio, &fechaLectura,
		)
		if err != nil {
			return nil, err
		}
		if repID.Valid {
			n.ReporteID = repID.String
		}
		if fechaLectura.Valid {
			t := fechaLectura.Time
			n.FechaLectura = &t
		}
		list = append(list, n)
	}
	if list == nil {
		list = []NotificacionHistorial{}
	}
	return list, nil
}

func (r *userRepository) ListConductors() ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`
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
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
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
			continue
		}

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

		result = append(result, map[string]interface{}{
			"conductor_id":          id,
			"nombre":                nombre,
			"tipo":                  tipo,
			"tipo_conductor":        mapearTipoConductor(nombre),
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
	if result == nil {
		result = []map[string]interface{}{}
	}
	return result, nil
}

func mapearTipoConductor(nombre string) string {
	nombreLower := strings.ToLower(nombre)
	if strings.Contains(nombreLower, "taxi") || strings.Contains(nombreLower, "taxista") {
		return "taxista"
	}
	if strings.Contains(nombreLower, "comer") || strings.Contains(nombreLower, "carga") {
		return "comerciante"
	}
	if strings.Contains(nombreLower, "proteccion") || strings.Contains(nombreLower, "civil") || strings.Contains(nombreLower, "emergencia") {
		return "proteccion_civil"
	}
	return "particular"
}

func round(val float64, precision int) float64 {
	format := float64(1)
	for i := 0; i < precision; i++ {
		format *= 10
	}
	return float64(int(val*format)) / format
}
