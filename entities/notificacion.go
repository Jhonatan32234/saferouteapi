package entities

import "time"

type NotificacionEntity struct {
	ID           string
	UserID       string
	Tipo         string
	ReporteID    *string
	Latitud      float64
	Longitud     float64
	NotaVoz      string
	RutaID       string
	Mensaje      string
	Leida        bool
	FechaEnvio   time.Time
	FechaLectura *time.Time
}
