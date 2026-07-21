package viaje

import (
	"database/sql"
	"fmt"
	"log"
)

type Repository interface {
	Create(viaje *Viaje, wktLineString string) (string, error)
	FindByID(id string) (*Viaje, error)
	FindActiveByUserID(userID string) (*Viaje, error)
	FindAllActive() ([]ViajeActivoAdmin, error)
	UpdateEstado(id string, estado string) error
	UpdateHeartbeat(viajeID string, lat, lon, vel float64) (distanciaDesvio float64, distanciaDestino float64, err error)
	GetLastCoordinate(viajeID string) (lat float64, lon float64, found bool, err error)
	FindActiveByEmpresa(empresaID string) ([]ViajeActivoAdmin, error)
}

type viajeRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &viajeRepository{db: db}
}


// FindActiveByEmpresa consulta viajes activos solo de conductores de una empresa
func (r *viajeRepository) FindActiveByEmpresa(empresaID string) ([]ViajeActivoAdmin, error) {
    query := `
        SELECT 
            v.id AS viaje_id,
            v.user_id,
            u.nombre AS nombre_conductor,
            v.ruta_id,
            v.origen_lat,
            v.origen_lon,
            v.destino_lat,
            v.destino_lon,
            v.polyline_ruta,
            v.estado,
            v.ultimo_heartbeat,
            COALESCE(h.latitud, v.origen_lat) AS ultima_latitud,
            COALESCE(h.longitud, v.origen_lon) AS ultima_longitud,
            COALESCE(h.velocidad_kmh, 0.0) AS ultima_velocidad_kmh
        FROM viajes v
        JOIN usuarios u ON u.id = v.user_id
        LEFT JOIN LATERAL (
            SELECT latitud, longitud, velocidad_kmh 
            FROM historial_viaje_coordenadas 
            WHERE viaje_id = v.id 
            ORDER BY timestamp DESC LIMIT 1
        ) h ON true
        WHERE v.estado IN ('activo', 'desviado', 'parada_tecnica', 'contacto_perdido')
          AND u.empresa_id = $1
        ORDER BY v.fecha_inicio DESC`

    rows, err := r.db.Query(query, empresaID)
    if err != nil {
        return nil, fmt.Errorf("error consultando viajes activos: %w", err)
    }
    defer rows.Close()

    var viajes []ViajeActivoAdmin
    for rows.Next() {
        var va ViajeActivoAdmin
        err := rows.Scan(
            &va.ViajeID, &va.UserID, &va.NombreConductor, &va.RutaID,
            &va.OrigenLat, &va.OrigenLon, &va.DestinoLat, &va.DestinoLon,
            &va.PolylineRuta, &va.Estado, &va.UltimoHeartbeat,
            &va.UltimaLatitud, &va.UltimaLongitud, &va.UltimaVelocidad,
        )
        if err != nil {
            return nil, fmt.Errorf("error escaneando viaje activo: %w", err)
        }
        viajes = append(viajes, va)
    }

    return viajes, nil
}

func (r *viajeRepository) Create(viaje *Viaje, wktLineString string) (string, error) {
	var id string
	query := `
		INSERT INTO viajes (user_id, ruta_id, origen_lat, origen_lon, destino_lat, destino_lon, polyline_ruta, geom_ruta, estado)
		VALUES ($1, $2, $3, $4, $5, $6, $7, ST_GeomFromText($8, 4326), $9)
		RETURNING id`

	err := r.db.QueryRow(query,
		viaje.UserID, viaje.RutaID,
		viaje.OrigenLat, viaje.OrigenLon,
		viaje.DestinoLat, viaje.DestinoLon,
		viaje.PolylineRuta, wktLineString,
		viaje.Estado,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("error insertando viaje: %w", err)
	}

	return id, nil
}

func (r *viajeRepository) FindByID(id string) (*Viaje, error) {
	v := &Viaje{}
	var fechaFin sql.NullTime

	query := `
		SELECT id, user_id, ruta_id, origen_lat, origen_lon, destino_lat, destino_lon, polyline_ruta, estado, fecha_inicio, fecha_fin, ultimo_heartbeat, creado_en
		FROM viajes WHERE id = $1`

	err := r.db.QueryRow(query, id).Scan(
		&v.ID, &v.UserID, &v.RutaID, &v.OrigenLat, &v.OrigenLon,
		&v.DestinoLat, &v.DestinoLon, &v.PolylineRuta, &v.Estado,
		&v.FechaInicio, &fechaFin, &v.UltimoHeartbeat, &v.CreadoEn,
	)
	if err != nil {
		return nil, err
	}

	if fechaFin.Valid {
		v.FechaFin = &fechaFin.Time
	}

	return v, nil
}

func (r *viajeRepository) FindActiveByUserID(userID string) (*Viaje, error) {
	v := &Viaje{}
	var fechaFin sql.NullTime

	query := `
		SELECT id, user_id, ruta_id, origen_lat, origen_lon, destino_lat, destino_lon, polyline_ruta, estado, fecha_inicio, fecha_fin, ultimo_heartbeat, creado_en
		FROM viajes 
		WHERE user_id = $1 AND estado IN ('activo', 'desviado', 'parada_tecnica')
		ORDER BY fecha_inicio DESC LIMIT 1`

	err := r.db.QueryRow(query, userID).Scan(
		&v.ID, &v.UserID, &v.RutaID, &v.OrigenLat, &v.OrigenLon,
		&v.DestinoLat, &v.DestinoLon, &v.PolylineRuta, &v.Estado,
		&v.FechaInicio, &fechaFin, &v.UltimoHeartbeat, &v.CreadoEn,
	)
	if err != nil {
		return nil, err
	}

	if fechaFin.Valid {
		v.FechaFin = &fechaFin.Time
	}

	return v, nil
}

func (r *viajeRepository) FindAllActive() ([]ViajeActivoAdmin, error) {
	query := `
		SELECT 
			v.id AS viaje_id,
			v.user_id,
			u.nombre AS nombre_conductor,
			v.ruta_id,
			v.origen_lat,
			v.origen_lon,
			v.destino_lat,
			v.destino_lon,
			v.polyline_ruta,
			v.estado,
			v.ultimo_heartbeat,
			COALESCE(h.latitud, v.origen_lat) AS ultima_latitud,
			COALESCE(h.longitud, v.origen_lon) AS ultima_longitud,
			COALESCE(h.velocidad_kmh, 0.0) AS ultima_velocidad_kmh
		FROM viajes v
		JOIN usuarios u ON u.id = v.user_id
		LEFT JOIN LATERAL (
			SELECT latitud, longitud, velocidad_kmh 
			FROM historial_viaje_coordenadas 
			WHERE viaje_id = v.id 
			ORDER BY timestamp DESC LIMIT 1
		) h ON true
		WHERE v.estado IN ('activo', 'desviado', 'parada_tecnica', 'contacto_perdido')
		ORDER BY v.fecha_inicio DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error consultando viajes activos: %w", err)
	}
	defer rows.Close()

	var viajes []ViajeActivoAdmin
	for rows.Next() {
		var va ViajeActivoAdmin
		err := rows.Scan(
			&va.ViajeID, &va.UserID, &va.NombreConductor, &va.RutaID,
			&va.OrigenLat, &va.OrigenLon, &va.DestinoLat, &va.DestinoLon,
			&va.PolylineRuta, &va.Estado, &va.UltimoHeartbeat,
			&va.UltimaLatitud, &va.UltimaLongitud, &va.UltimaVelocidad,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando viaje activo: %w", err)
		}
		viajes = append(viajes, va)
	}

	return viajes, nil
}

func (r *viajeRepository) UpdateEstado(id string, estado string) error {
	var err error
	if estado == "finalizado" || estado == "cancelado" {
		_, err = r.db.Exec(`UPDATE viajes SET estado = $1, fecha_fin = NOW(), ultimo_heartbeat = NOW() WHERE id = $2`, estado, id)
	} else {
		_, err = r.db.Exec(`UPDATE viajes SET estado = $1, ultimo_heartbeat = NOW() WHERE id = $2`, estado, id)
	}
	return err
}

func (r *viajeRepository) UpdateHeartbeat(viajeID string, lat, lon, vel float64) (distanciaDesvio float64, distanciaDestino float64, err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO historial_viaje_coordenadas (viaje_id, latitud, longitud, velocidad_kmh)
		VALUES ($1, $2, $3, $4)`,
		viajeID, lat, lon, vel,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("error insertando coordenada de historial: %w", err)
	}

	_, err = tx.Exec(`
		UPDATE viajes 
		SET ultimo_heartbeat = NOW() 
		WHERE id = $1`,
		viajeID,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("error actualizando heartbeat: %w", err)
	}

	queryDistancias := `
		SELECT 
			COALESCE(ST_Distance(ST_SetSRID(ST_Point($1, $2), 4326)::geography, geom_ruta::geography), 0.0) AS dist_desvio,
			COALESCE(ST_Distance(ST_SetSRID(ST_Point($1, $2), 4326)::geography, ST_SetSRID(ST_Point(destino_lon, destino_lat), 4326)::geography), 0.0) AS dist_destino
		FROM viajes 
		WHERE id = $3`

	err = tx.QueryRow(queryDistancias, lon, lat, viajeID).Scan(&distanciaDesvio, &distanciaDestino)
	if err != nil {
		log.Printf("Fallback: Error calculando distancias con PostGIS: %v. Usando 0.0 por defecto", err)
		distanciaDesvio = 0.0
		distanciaDestino = 99999.0
		err = nil
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}

	return distanciaDesvio, distanciaDestino, nil
}

func (r *viajeRepository) GetLastCoordinate(viajeID string) (lat float64, lon float64, found bool, err error) {
	err = r.db.QueryRow(`
		SELECT latitud, longitud 
		FROM historial_viaje_coordenadas 
		WHERE viaje_id = $1 
		ORDER BY timestamp DESC LIMIT 1`, 
		viajeID,
	).Scan(&lat, &lon)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return lat, lon, true, nil
}
