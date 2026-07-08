package models

import "time"


type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
    Nombre   string `json:"nombre"`
    Tipo     string `json:"tipo"`
    Telefono string `json:"telefono"`
}


type AuthResponse struct {
	Token  string `json:"token"`
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"`
	Email  string `json:"email,omitempty"`
	UserID string `json:"user_id,omitempty"`
}



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

type NotificacionAlerta struct {
	Tipo      string    `json:"tipo"`
	ReporteID string    `json:"reporte_id"`
	Latitud   float64   `json:"latitud"`
	Longitud  float64   `json:"longitud"`
	NotaVoz   string    `json:"nota_voz,omitempty"`
	RutaID    string    `json:"ruta_id"`
	Timestamp time.Time `json:"timestamp"`
	Mensaje   string    `json:"mensaje"`
}


type SuscripcionRuta struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"user_id"`
	RutaID             string    `json:"ruta_id"`
	Suscrito           bool      `json:"suscrito"`
	FechaSuscripcion   time.Time `json:"fecha_suscripcion"`
	FechaActualizacion time.Time `json:"fecha_actualizacion"`
}


type ReporteRequest struct {
	Tipo     string  `json:"tipo"`
	Latitud  float64 `json:"latitud"`
	Longitud float64 `json:"longitud"`
	NotaVoz  string  `json:"nota_voz,omitempty"`
	RutaID   string  `json:"ruta_id"`
}

type ReporteResponse struct {
	ID             string    `json:"id"`
	Tipo           string    `json:"tipo"`
	Latitud        float64   `json:"latitud"`
	Longitud       float64   `json:"longitud"`
	NotaVoz        string    `json:"nota_voz,omitempty"`
	RutaID         string    `json:"ruta_id"`
	Timestamp      time.Time `json:"timestamp"`
	Vigente        bool      `json:"vigente"`
	Confirmaciones int       `json:"confirmaciones"`
}

type BusquedaRequest struct {
	Query string `json:"query"`
}

type BusquedaResponse struct {
	Resultados []ReporteResultado `json:"resultados"`
	Total      int                `json:"total"`
}

type ReporteResultado struct {
	Reporte ReporteResponse `json:"reporte"`
	Score   float64         `json:"score"`
}


type RutasRequest struct {
	OrigenLat  float64 `json:"origen_lat"`
	OrigenLon  float64 `json:"origen_lon"`
	DestinoLat float64 `json:"destino_lat"`
	DestinoLon float64 `json:"destino_lon"`
}

type RutaResponse struct {
	ID                 string    `json:"id"`
	Nombre             string    `json:"nombre"`
	DistanciaKM        float64   `json:"distancia_km"`
	TiempoMinutos      int       `json:"tiempo_minutos"`
	Seguridad          string    `json:"seguridad"`
	RiesgoCombinado    float64   `json:"riesgo_combinado"`
	ClustersAtravesados []int    `json:"clusters_atravesados"`
	Polyline           string    `json:"polyline"`
}

type RutasResponse struct {
	Rutas       []RutaResponse `json:"rutas"`
	Recomendada string         `json:"recomendada"`
}


type AdminResumenRequest struct {
	SemanaInicio string `json:"semana_inicio"`
	SemanaFin    string `json:"semana_fin"`
}

type AdminResumenResponse struct {
	TotalReportes    int           `json:"total_reportes"`
	Topicos          []TopicoInfo  `json:"topicos"`
	ResumenLLM       string        `json:"resumen_llm"`
	FechaGeneracion  time.Time     `json:"fecha_generacion"`
}

type TopicoInfo struct {
	ID             int      `json:"id"`
	Nombre         string   `json:"nombre"`
	Frecuencia     int      `json:"frecuencia"`
	Porcentaje     float64  `json:"porcentaje"`
	PalabrasClave  []string `json:"palabras_clave"`
	Tendencia      string   `json:"tendencia"`
	AccionSugerida string   `json:"accion_sugerida"`
}


type ReportesCercanosRequest struct {
	Latitud  float64 `json:"lat"`
	Longitud float64 `json:"lon"`
	RadioKm  float64 `json:"radio_km"`
}

type IncidenteCercano struct {
	ID             string    `json:"id"`
	Tipo           string    `json:"tipo"`
	Latitud        float64   `json:"latitud"`
	Longitud       float64   `json:"longitud"`
	NotaVoz        string    `json:"nota_voz,omitempty"`
	RutaID         string    `json:"ruta_id"`
	Timestamp      time.Time `json:"timestamp"`
	Confirmaciones int       `json:"confirmaciones"`
	DistanciaKm    float64   `json:"distancia_km"`
}


type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Detalle string `json:"detalle,omitempty"`
}


type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Database  string `json:"database"`
}


var TiposValidos = map[string]bool{
	"accidente":  true,
	"inundacion": true,
	"bache":      true,
	"derrumbe":   true,
	"sin_luz":    true,
	"niebla":     true,
	"bloqueo":    true,
	"otro":       true,
}


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