package repository

import (
	"database/sql"
	"fmt"

	"saferoute/entities"
)


type ReporteRepository struct {
	db *sql.DB
}

func NewReporteRepository(db *sql.DB) *ReporteRepository {
	return &ReporteRepository{db: db}
}

func (r *ReporteRepository) Create(e *entities.ReporteEntity) (*entities.ReporteEntity, error) {
	result := &entities.ReporteEntity{}
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

func (r *ReporteRepository) FindAll(tipo string, vigente *bool, limit int, offset int) ([]entities.ReporteEntity, error) {
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
		return []entities.ReporteEntity{}, nil
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

	var reportes []entities.ReporteEntity
	for rows.Next() {
		var e entities.ReporteEntity
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

func (r *ReporteRepository) FindByID(id string) (*entities.ReporteEntity, error) {
	e := &entities.ReporteEntity{}
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

func (r *ReporteRepository) FindCercanos(lat, lon, radioKm float64, limit int) ([]entities.ReporteEntity, error) {
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

	var reportes []entities.ReporteEntity
	for rows.Next() {
		var e entities.ReporteEntity
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

func (r *ReporteRepository) Validar(id string, vigente bool) error {
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

func (r *ReporteRepository) SuscribirRuta(userID, rutaID string) error {
	_, err := r.db.Exec(
		`INSERT INTO suscripciones_rutas (user_id, ruta_id, suscrito, fecha_suscripcion, fecha_actualizacion)
		 VALUES ($1, $2, true, NOW(), NOW())
		 ON CONFLICT (user_id, ruta_id)
		 DO UPDATE SET suscrito = true, fecha_actualizacion = NOW()`,
		userID, rutaID,
	)
	return err
}
