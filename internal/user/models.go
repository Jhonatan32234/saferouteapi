package user

import "time"

type UserProfile struct {
    ID                  string    `json:"id"`
    Email               string    `json:"email"`
    Nombre              string    `json:"nombre"`
    Tipo                string    `json:"tipo"`
    Telefono            string    `json:"telefono"`
    CreatedAt           time.Time `json:"created_at,omitempty"`
    UpdatedAt           time.Time `json:"updated_at,omitempty"`
    UltimoAcceso        time.Time `json:"ultimo_acceso,omitempty"`
    ReportesCreados     int       `json:"reportes_creados"`
    ReportesConfirmados int       `json:"reportes_confirmados"`
}

type UpdateProfileRequest struct {
	Nombre   string `json:"nombre,omitempty"`
	Telefono string `json:"telefono,omitempty"`
	Email    string `json:"email,omitempty"`
}

type NotificacionHistorial struct {
    ID           string     `json:"id"`
    UserID       string     `json:"user_id"`
    Tipo         string     `json:"tipo"`
    ReporteID    string     `json:"reporte_id,omitempty"`
    Latitud      float64    `json:"latitud,omitempty"`
    Longitud     float64    `json:"longitud,omitempty"`
    NotaVoz      string     `json:"nota_voz,omitempty"`
    RutaID       string     `json:"ruta_id"`
    Mensaje      string     `json:"mensaje"`
    Leida        bool       `json:"leida"`
    FechaEnvio   time.Time  `json:"fecha_envio"`
    FechaLectura *time.Time `json:"fecha_lectura,omitempty"`
}

type NotificacionHistorialResponse struct {
	Notificaciones []NotificacionHistorial `json:"notificaciones"`
	Total          int                     `json:"total"`
	NoLeidas       int                     `json:"no_leidas"`
	Pagina         int                     `json:"pagina"`
	TotalPaginas   int                     `json:"total_paginas"`
}

type MarcarNotificacionRequest struct {
	Leida bool `json:"leida"`
}

type SuscripcionRuta struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"user_id"`
	RutaID             string    `json:"ruta_id"`
	Suscrito           bool      `json:"suscrito"`
	FechaSuscripcion   time.Time `json:"fecha_suscripcion"`
	FechaActualizacion time.Time `json:"fecha_actualizacion"`
}

type DestinoRecienteRequest struct {
	Nombre string  `json:"nombre"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

type DestinoRecienteResponse struct {
	ID           string    `json:"id"`
	Nombre       string    `json:"nombre"`
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	FechaCreacion time.Time `json:"fecha_creacion"`
}

type ZonaUsuario struct {
    ID          string    `json:"id"`
    UserID      string    `json:"user_id"`
    ZonaNombre  string    `json:"zona_nombre"`
    Latitud     float64   `json:"latitud"`
    Longitud    float64   `json:"longitud"`
    RadioKm     float64   `json:"radio_km"`
    Activo      bool      `json:"activo"`
}

type ZonaRequest struct {
    ZonaNombre string  `json:"zona_nombre"`
    Latitud    float64 `json:"latitud"`
    Longitud   float64 `json:"longitud"`
    RadioKm    float64 `json:"radio_km,omitempty"`
}
