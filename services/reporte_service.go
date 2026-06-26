package services

import (
	"fmt"

	"saferoute/entities"
	"saferoute/models"
	"saferoute/repository"
)

// ReporteService contiene la lógica de negocio para los reportes viales.
// Transforma Entities ↔ DTOs y aplica reglas de negocio sin tocar SQL directamente.
type ReporteService struct {
	reporteRepo *repository.ReporteRepository
}

// NewReporteService crea una nueva instancia del servicio de reportes.
func NewReporteService(repo *repository.ReporteRepository) *ReporteService {
	return &ReporteService{reporteRepo: repo}
}

// Create persiste un nuevo reporte vial y devuelve el DTO de respuesta.
// También registra automáticamente la suscripción del usuario a la ruta en background.
func (s *ReporteService) Create(req models.ReporteRequest, userID string) (models.ReporteResponse, error) {
	// Mapear DTO → Entity (sin SQL)
	entity := &entities.ReporteEntity{
		UserID:   userID,
		Tipo:     req.Tipo,
		Latitud:  req.Latitud,
		Longitud: req.Longitud,
		NotaVoz:  req.NotaVoz,
		RutaID:   req.RutaID,
	}

	// Persistir a través del repositorio
	created, err := s.reporteRepo.Create(entity)
	if err != nil {
		return models.ReporteResponse{}, fmt.Errorf("error al guardar el reporte: %w", err)
	}

	// Suscripción automática a la ruta (en background, no bloquea la respuesta)
	if userID != "" && req.RutaID != "" && userID != "anonimo" {
		go func() {
			_ = s.reporteRepo.SuscribirRuta(userID, req.RutaID)
		}()
	}

	// Mapear Entity → DTO de salida
	return entityToReporteResponse(created), nil
}

// GetAll obtiene la lista de reportes con filtros opcionales y devuelve DTOs limpios.
func (s *ReporteService) GetAll(tipo string, vigenteStr string, limit int) ([]models.ReporteResponse, error) {
	var vigenteFilter *bool
	if vigenteStr == "true" {
		v := true
		vigenteFilter = &v
	} else if vigenteStr == "false" {
		v := false
		vigenteFilter = &v
	}

	entities, err := s.reporteRepo.FindAll(tipo, vigenteFilter, limit)
	if err != nil {
		return nil, fmt.Errorf("error consultando reportes: %w", err)
	}

	reportes := make([]models.ReporteResponse, 0, len(entities))
	for _, e := range entities {
		reportes = append(reportes, entityToReporteResponse(&e))
	}
	return reportes, nil
}

// GetByID obtiene un reporte específico por su ID.
func (s *ReporteService) GetByID(id string) (models.ReporteResponse, error) {
	entity, err := s.reporteRepo.FindByID(id)
	if err != nil {
		return models.ReporteResponse{}, fmt.Errorf("reporte no encontrado")
	}
	return entityToReporteResponse(entity), nil
}

// Validar permite confirmar o descartar la vigencia de un reporte.
func (s *ReporteService) Validar(id string, vigente bool) error {
	if err := s.reporteRepo.Validar(id, vigente); err != nil {
		return fmt.Errorf("error validando reporte: %w", err)
	}
	return nil
}

// GetCercanos devuelve los reportes dentro de un radio geográfico.
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

// =============================================================
// Mapeadores internos Entity ↔ DTO
// =============================================================

// entityToReporteResponse mapea una Entity de base de datos a un DTO de salida HTTP.
// Esto protege de filtrar campos internos no deseados hacia el cliente.
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
