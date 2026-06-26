package entities

import "time"

// ReporteEntity representa 1:1 la tabla `reportes` en la base de datos.
type ReporteEntity struct {
	ID             string
	UserID         string
	Tipo           string
	Latitud        float64
	Longitud       float64
	NotaVoz        string
	RutaID         string
	Timestamp      time.Time
	Vigente        bool
	Confirmaciones int
}
