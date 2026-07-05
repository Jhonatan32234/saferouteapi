package services

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"saferoute/models"
)

type MotorSyncService struct {
	nlpURL          string
	prediccionesURL string
	apiKey          string
	client          *http.Client
}

func NewMotorSyncService(nlpURL, prediccionesURL, apiKey string) *MotorSyncService {
	return &MotorSyncService{
		nlpURL:           strings.TrimRight(nlpURL, "/"),
		prediccionesURL:  strings.TrimRight(prediccionesURL, "/"),
		apiKey:           apiKey,
		client:           &http.Client{Timeout: 3 * time.Second},
	}
}

func (s *MotorSyncService) SyncReporteCreado(reporte models.ReporteResponse) {
	payload := map[string]interface{}{
		"id":              reporte.ID,
		"texto":           textoReporte(reporte),
		"tipo":            reporte.Tipo,
		"ruta_id":         reporte.RutaID,
		"latitud":         reporte.Latitud,
		"longitud":        reporte.Longitud,
		"timestamp":       reporte.Timestamp,
		"vigente":         reporte.Vigente,
		"confirmaciones": reporte.Confirmaciones,
		"evento":         "reporte_creado",
	}

	s.postJSON(s.nlpURL+"/nlp/ingest/reporte", payload)
	s.postJSON(s.prediccionesURL+"/predicciones/ingest/reporte", payload)
}

func (s *MotorSyncService) SyncReporteValidado(reporteID string, vigente bool) {
	payload := map[string]interface{}{
		"id":        reporteID,
		"vigente":   vigente,
		"evento":    "reporte_validado",
		"timestamp": time.Now().UTC(),
	}

	s.postJSON(s.nlpURL+"/nlp/ingest/reporte/estado", payload)
	s.postJSON(s.prediccionesURL+"/predicciones/ingest/reporte/estado", payload)
}

func (s *MotorSyncService) SyncInteraccion(tipo, userID, rutaID string, data map[string]interface{}) {
	payload := map[string]interface{}{
		"tipo":      tipo,
		"user_id":   userID,
		"ruta_id":   rutaID,
		"timestamp": time.Now().UTC(),
		"data":      data,
	}

	s.postJSON(s.prediccionesURL+"/predicciones/ingest/interaccion", payload)
}

func (s *MotorSyncService) postJSON(url string, payload interface{}) {
	if s == nil || url == "" || strings.HasPrefix(url, "/") {
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[MOTOR_SYNC] Error serializando payload: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[MOTOR_SYNC] Error creando request %s: %v", url, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("X-Internal-API-Key", s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("[MOTOR_SYNC] Motor no disponible %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[MOTOR_SYNC] Motor respondio %d para %s", resp.StatusCode, url)
	}
}

func textoReporte(reporte models.ReporteResponse) string {
	if strings.TrimSpace(reporte.NotaVoz) != "" {
		return reporte.NotaVoz
	}
	return reporte.Tipo
}
