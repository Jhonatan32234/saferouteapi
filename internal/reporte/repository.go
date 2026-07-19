package reporte

import (
	"database/sql"
	"fmt"
)

type Repository interface {
	Create(e *ReporteEntity) (*ReporteEntity, error)
	FindAll(tipo string, vigente *bool, limit int, offset int) ([]ReporteEntity, error)
	FindByID(id string) (*ReporteEntity, error)
	FindCercanos(lat, lon, radioKm float64, limit int) ([]ReporteEntity, error)
	Validar(id string, vigente bool) error
	GetEstadisticas() (ReporteEstadisticas, error)
}

type reporteRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &reporteRepository{db: db}
}

func (r *reporteRepository) Create(e *ReporteEntity) (*ReporteEntity, error) {
	result := &ReporteEntity{}
	err := r.db.QueryRow(
		`INSERT INTO reportes (user_id, tipo, latitud, longitud, nota_voz, ruta_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, tipo, latitud, longitud, COALESCE(nota_voz,''),
		           ruta_id, timestamp, vigente, confirmaciones`,
		e.UserID, e.Tipo, e.Latitud, e.Longitud, e.NotaVoz, e.RutaID,
	).Scan(
		&result.ID, &result.UserID, &result.Tipo, &result.Latitud, &result.Longitud,
		&result.NotaVoz, &result.RutaID, &result.Timestamp, &result.Vigente, &result.Confirmaciones,
	)
	return result, err
}

func (r *reporteRepository) FindAll(tipo string, vigente *bool, limit int, offset int) ([]ReporteEntity, error) {
	query := `SELECT id, COALESCE(user_id::text,''), tipo, latitud, longitud,
	                 COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones
	          FROM reportes WHERE 1=1`
	args := []interface{}{}
	argCount := 0

	if tipo != "" {
		argCount++
		query += fmt.Sprintf(" AND tipo = $%d", argCount)
		args = append(args, tipo)
	}
	if vigente != nil {
		argCount++
		query += fmt.Sprintf(" AND vigente = $%d", argCount)
		args = append(args, *vigente)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM reportes WHERE 1=1"
	countArgs := []interface{}{}
	cArg := 0
	if tipo != "" {
		cArg++
		countQuery += fmt.Sprintf(" AND tipo = $%d", cArg)
		countArgs = append(countArgs, tipo)
	}
	if vigente != nil {
		cArg++
		countQuery += fmt.Sprintf(" AND vigente = $%d", cArg)
		countArgs = append(countArgs, *vigente)
	}
	r.db.QueryRow(countQuery, countArgs...).Scan(&total)

	if offset >= total {
		return []ReporteEntity{}, nil
	}

	argCount++
	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT $%d", argCount)
	args = append(args, limit)

	argCount++
	query += fmt.Sprintf(" OFFSET $%d", argCount)
	args = append(args, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reportes []ReporteEntity
	for rows.Next() {
		var e ReporteEntity
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.Tipo, &e.Latitud, &e.Longitud,
			&e.NotaVoz, &e.RutaID, &e.Timestamp, &e.Vigente, &e.Confirmaciones,
		); err != nil {
			continue
		}
		reportes = append(reportes, e)
	}
	return reportes, nil
}

func (r *reporteRepository) FindByID(id string) (*ReporteEntity, error) {
	e := &ReporteEntity{}
	err := r.db.QueryRow(
		`SELECT id, COALESCE(user_id::text,''), tipo, latitud, longitud,
		        COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones
		 FROM reportes WHERE id = $1`,
		id,
	).Scan(
		&e.ID, &e.UserID, &e.Tipo, &e.Latitud, &e.Longitud,
		&e.NotaVoz, &e.RutaID, &e.Timestamp, &e.Vigente, &e.Confirmaciones,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *reporteRepository) FindCercanos(lat, lon, radioKm float64, limit int) ([]ReporteEntity, error) {
	rows, err := r.db.Query(
		`SELECT id, COALESCE(user_id::text,''), tipo, latitud, longitud,
		        COALESCE(nota_voz,''), ruta_id, timestamp, vigente, confirmaciones,
		        (6371 * acos(
		            cos(radians($1)) * cos(radians(latitud)) *
		            cos(radians(longitud) - radians($2)) +
		            sin(radians($1)) * sin(radians(latitud))
		        )) AS distancia_km
		 FROM reportes
		 WHERE vigente = true
		   AND (6371 * acos(
		            cos(radians($1)) * cos(radians(latitud)) *
		            cos(radians(longitud) - radians($2)) +
		            sin(radians($1)) * sin(radians(latitud))
		        )) <= $3
		 ORDER BY distancia_km ASC
		 LIMIT $4`,
		lat, lon, radioKm, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reportes []ReporteEntity
	for rows.Next() {
		var e ReporteEntity
		var distancia float64
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.Tipo, &e.Latitud, &e.Longitud,
			&e.NotaVoz, &e.RutaID, &e.Timestamp, &e.Vigente, &e.Confirmaciones,
			&distancia,
		); err != nil {
			continue
		}
		reportes = append(reportes, e)
	}
	return reportes, nil
}

func (r *reporteRepository) Validar(id string, vigente bool) error {
	var err error
	if vigente {
		_, err = r.db.Exec(
			`UPDATE reportes SET confirmaciones = confirmaciones + 1 WHERE id = $1`,
			id,
		)
	} else {
		_, err = r.db.Exec(
			`UPDATE reportes SET vigente = false WHERE id = $1`,
			id,
		)
	}
	return err
}

func (r *reporteRepository) GetEstadisticas() (ReporteEstadisticas, error) {
	var stats ReporteEstadisticas
	err := r.db.QueryRow("SELECT COUNT(*) FROM reportes WHERE vigente = TRUE").Scan(&stats.TotalReportes)
	if err != nil {
		return stats, err
	}

	stats.ReportesPorTipo = make(map[string]int)
	rows, err := r.db.Query("SELECT tipo, COUNT(*) FROM reportes WHERE vigente = TRUE GROUP BY tipo")
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tipo string
			var count int
			if err := rows.Scan(&tipo, &count); err == nil {
				stats.ReportesPorTipo[tipo] = count
			}
		}
	}

	r.db.QueryRow("SELECT COUNT(*) FROM reportes WHERE timestamp::date = CURRENT_DATE").Scan(&stats.ReportesHoy)
	r.db.QueryRow("SELECT COUNT(*) FROM reportes WHERE timestamp >= date_trunc('week', CURRENT_DATE)").Scan(&stats.ReportesSemana)

	var totalConfirmaciones int
	r.db.QueryRow("SELECT COALESCE(SUM(confirmaciones), 0) FROM reportes").Scan(&totalConfirmaciones)
	if stats.TotalReportes > 0 {
		stats.TasaConfirmacion = float64(totalConfirmaciones) / float64(stats.TotalReportes)
	}

	return stats, nil
}
