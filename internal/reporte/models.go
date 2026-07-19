package reporte

import "time"

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

type AdminResumenRequest struct {
	SemanaInicio string `json:"semana_inicio"`
	SemanaFin    string `json:"semana_fin"`
}

type AdminResumenResponse struct {
	TotalReportes    int          `json:"total_reportes"`
	Topicos          []TopicoInfo `json:"topicos"`
	ResumenLLM       string       `json:"resumen_llm"`
	FechaGeneracion  time.Time    `json:"fecha_generacion"`
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

type ReporteEstadisticas struct {
	TotalReportes    int            `json:"total_reportes"`
	ReportesPorTipo  map[string]int `json:"reportes_por_tipo"`
	ReportesHoy      int            `json:"reportes_hoy"`
	ReportesSemana   int            `json:"reportes_semana"`
	TasaConfirmacion float64        `json:"tasa_confirmacion"`
}
