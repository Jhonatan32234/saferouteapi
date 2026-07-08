package services

import (
	"fmt"

	"saferoute/entities"
	"saferoute/models"
	"saferoute/repository"
)

type ReporteService struct {
	reporteRepo *repository.ReporteRepository
}

func NewReporteService(repo *repository.ReporteRepository) *ReporteService {
	return &ReporteService{reporteRepo: repo}
}

func (s *ReporteService) Create(req models.ReporteRequest, userID string) (models.ReporteResponse, error) {
	entity := &entities.ReporteEntity{
		UserID:   userID,
		Tipo:     req.Tipo,
		Latitud:  req.Latitud,
		Longitud: req.Longitud,
		NotaVoz:  req.NotaVoz,
		RutaID:   req.RutaID,
	}

	created, err := s.reporteRepo.Create(entity)
	if err != nil {
		return models.ReporteResponse{}, fmt.Errorf("error al guardar el reporte: %w", err)
	}

	if userID != "" && req.RutaID != "" && userID != "anonimo" {
		go func() {
			_ = s.reporteRepo.SuscribirRuta(userID, req.RutaID)
		}()
	}

	return entityToReporteResponse(created), nil
}

func (s *ReporteService) GetAll(tipo string, vigenteStr string, limit int, offset int) ([]models.ReporteResponse, error) {
	var vigenteFilter *bool
	if vigenteStr == "true" {
		v := true
		vigenteFilter = &v
	} else if vigenteStr == "false" {
		v := false
		vigenteFilter = &v
	}

	entities, err := s.reporteRepo.FindAll(tipo, vigenteFilter, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error consultando reportes: %w", err)
	}

	reportes := make([]models.ReporteResponse, 0, len(entities))
	for _, e := range entities {
		reportes = append(reportes, entityToReporteResponse(&e))
	}
	return reportes, nil
}

func (s *ReporteService) GetByID(id string) (models.ReporteResponse, error) {
	entity, err := s.reporteRepo.FindByID(id)
	if err != nil {
		return models.ReporteResponse{}, fmt.Errorf("reporte no encontrado")
	}
	return entityToReporteResponse(entity), nil
}

func (s *ReporteService) Validar(id string, vigente bool) error {
	if err := s.reporteRepo.Validar(id, vigente); err != nil {
		return fmt.Errorf("error validando reporte: %w", err)
	}
	return nil
}

func (s *ReporteService) GetCercanos(lat, lon, radioKm float64, limit int) ([]models.ReporteResponse, error) {
	if radioKm <= 0 {
		radioKm = 5.0
	}
	if limit <= 0 {
		limit = 20
	}

	entities, err := s.reporteRepo.FindCercanos(lat, lon, radioKm, limit)
	if err != nil {
		return nil, fmt.Errorf("error buscando reportes cercanos: %w", err)
	}

	reportes := make([]models.ReporteResponse, 0, len(entities))
	for _, e := range entities {
		reportes = append(reportes, entityToReporteResponse(&e))
	}
	return reportes, nil
}

func entityToReporteResponse(e *entities.ReporteEntity) models.ReporteResponse {
	return models.ReporteResponse{
		ID:             e.ID,
		Tipo:           e.Tipo,
		Latitud:        e.Latitud,
		Longitud:       e.Longitud,
		NotaVoz:        e.NotaVoz,
		RutaID:         e.RutaID,
		Timestamp:      e.Timestamp,
		Vigente:        e.Vigente,
		Confirmaciones: e.Confirmaciones,
	}
}
