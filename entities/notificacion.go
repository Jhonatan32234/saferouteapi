package entities

import "time"

// NotificacionEntity representa 1:1 la tabla `notificaciones_historial` en la base de datos.
type NotificacionEntity struct {
	ID           string
	UserID       string
	Tipo         string
	ReporteID    *string // Nullable UUID
	Latitud      float64
	Longitud     float64
	NotaVoz      string
	RutaID       string
	Mensaje      string
	Leida        bool
	FechaEnvio   time.Time
	FechaLectura *time.Time // Nullable
}
