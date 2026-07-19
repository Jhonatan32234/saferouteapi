package viaje

import "time"

type Viaje struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id"`
	RutaID          string     `json:"ruta_id"`
	OrigenLat       float64    `json:"origen_lat"`
	OrigenLon       float64    `json:"origen_lon"`
	DestinoLat      float64    `json:"destino_lat"`
	DestinoLon      float64    `json:"destino_lon"`
	PolylineRuta    string     `json:"polyline_ruta"`
	Estado          string     `json:"estado"`
	FechaInicio     time.Time  `json:"fecha_inicio"`
	FechaFin        *time.Time `json:"fecha_fin,omitempty"`
	UltimoHeartbeat time.Time  `json:"ultimo_heartbeat"`
	CreadoEn        time.Time  `json:"creado_en"`
}

type IniciarViajeRequest struct {
	OrigenLat    float64 `json:"origen_lat"`
	OrigenLon    float64 `json:"origen_lon"`
	DestinoLat   float64 `json:"destino_lat"`
	DestinoLon   float64 `json:"destino_lon"`
	PolylineRuta string  `json:"polyline_ruta"`
	RutaID       string  `json:"ruta_id"`
}

type FinalizarViajeRequest struct {
	ViajeID  string `json:"viaje_id"`
	Password string `json:"password,omitempty"`
}

type ViajeActivoAdmin struct {
	ViajeID         string    `json:"viaje_id"`
	UserID          string    `json:"user_id"`
	NombreConductor string    `json:"nombre_conductor"`
	RutaID          string    `json:"ruta_id"`
	OrigenLat       float64   `json:"origen_lat"`
	OrigenLon       float64   `json:"origen_lon"`
	DestinoLat      float64   `json:"destino_lat"`
	DestinoLon      float64   `json:"destino_lon"`
	PolylineRuta    string    `json:"polyline_ruta"`
	Estado          string    `json:"estado"`
	UltimoHeartbeat time.Time `json:"ultimo_heartbeat"`
	UltimaLatitud   float64   `json:"ultima_latitud"`
	UltimaLongitud  float64   `json:"ultima_longitud"`
	UltimaVelocidad float64   `json:"ultima_velocidad_kmh"`
}
